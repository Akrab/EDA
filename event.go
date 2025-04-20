package edaRR

import (
	"context"
	"github.com/Akrab/EDA-RR/patternTree"
	"log"
	"sync"
	"time"
)

type FilterFunc func(Event) bool

type Event struct {
	ID            string                 // Unique event identifier
	Type          string                 // Event type
	CorrelationID string                 // ID for related events (e.g., request-response)
	ReplyTo       string                 // Topic for response event (optional)
	Data          interface{}            // Event payload
	Timestamp     time.Time              // Event creation timestamp
	Metadata      map[string]interface{} // Additional metadata
}

type Subscription struct {
	ID      string
	Channel chan Event
	Done    chan struct{} // Channel for signaling completion
}

type EventBus struct {
	// Pattern matching using PatternTree
	patternTree *patternTree.PatternTree

	// Subscriber channels indexed by pattern
	subscribers map[string][]Subscription

	// Channels for waiting for responses by correlation ID
	responseChannels map[string]chan Event

	logger *log.Logger

	mu     sync.RWMutex
	respMu sync.RWMutex
}

func NewEventBus(logger *log.Logger) *EventBus {
	if logger == nil {
		logger = log.New(log.Writer(), "[EventBus] ", log.LstdFlags)
	}
	return &EventBus{
		patternTree:      patternTree.NewPatternTree(),
		subscribers:      make(map[string][]Subscription),
		responseChannels: make(map[string]chan Event),
		logger:           logger,
	}
}

func (eb *EventBus) Subscribe(pattern string, bufferSize int) (<-chan Event, func()) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan Event, bufferSize)
	doneSignal := make(chan struct{})

	subscription := Subscription{
		ID:      GenerateID(pattern),
		Channel: ch,
		Done:    doneSignal,
	}

	eb.subscribers[pattern] = append(eb.subscribers[pattern], subscription)
	eb.patternTree.AddPattern(pattern)

	unsubscribe := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		close(doneSignal)

		subs := eb.subscribers[pattern]
		for i, sub := range subs {
			if sub.Channel == ch {
				eb.subscribers[pattern] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		if len(eb.subscribers[pattern]) == 0 {
			delete(eb.subscribers, pattern)
		}

		close(ch)
	}

	return ch, unsubscribe
}

func (eb *EventBus) SubscribeWithFilter(pattern string, bufferSize int, filter FilterFunc) (<-chan Event, func()) {
	eb.mu.Lock()

	eb.patternTree.AddPattern(pattern)

	filteredCh := make(chan Event, bufferSize)
	doneSignal := make(chan struct{})

	var once sync.Once
	closeFilteredCh := func() {
		once.Do(func() {
			close(filteredCh)
		})
	}

	// Create internal subscription channel with larger buffer
	internalCh := make(chan Event, bufferSize*2)

	subscription := Subscription{
		ID:      GenerateID("subscription"),
		Channel: internalCh,
		Done:    doneSignal,
	}

	eb.subscribers[pattern] = append(eb.subscribers[pattern], subscription)

	eb.mu.Unlock()

	// Start goroutine for event filtering
	go func() {
		defer closeFilteredCh()

		for {
			select {
			case event, ok := <-internalCh:
				if !ok {
					return
				}

				if filter(event) {
					select {
					case filteredCh <- event:
						// Event sent successfully
					case <-doneSignal:
						return
					default:
						// Channel full, log the drop
						eb.logger.Printf("Filtered channel for pattern %s is full, dropping event %s", pattern, event.ID)
					}
				}
			case <-doneSignal:
				return
			}
		}
	}()

	unsubscribe := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		close(doneSignal)

		subs := eb.subscribers[pattern]
		for i, sub := range subs {
			if sub.Channel == internalCh {
				eb.subscribers[pattern] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		if len(eb.subscribers[pattern]) == 0 {
			delete(eb.subscribers, pattern)
		}

		close(internalCh)
		closeFilteredCh()
	}

	return filteredCh, unsubscribe
}

func (eb *EventBus) Publish(eventType string, data interface{}, metadata ...map[string]interface{}) string {
	// Use shorter lock duration, optimization for high load
	eb.mu.RLock()
	matchingPatterns := eb.findMatchingPatterns(eventType)

	var subscriberCopies []Subscription
	for _, pattern := range matchingPatterns {
		subscriberCopies = append(subscriberCopies, eb.subscribers[pattern]...)
	}
	eb.mu.RUnlock()

	id := GenerateID(eventType)
	meta := make(map[string]interface{})
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	event := Event{
		ID:        id,
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
		Metadata:  meta,
	}

	// Send event to all subscribers without holding the lock
	for _, subscription := range subscriberCopies {
		select {
		case subscription.Channel <- event:
			// Event sent successfully
		case <-subscription.Done:
			// Subscriber completed, skip sending
		default:
			// Channel is full, log event loss
			eb.logger.Printf("Channel for event %s is full, dropping event %s", eventType, id)
		}
	}

	return id
}

func (eb *EventBus) PublishMultiple(eventTypes []string, data interface{}, metadata ...map[string]interface{}) []string {
	eventIDs := make([]string, len(eventTypes))
	events := make([]Event, len(eventTypes))

	meta := make(map[string]interface{})
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	for i, eventType := range eventTypes {
		eventIDs[i] = GenerateID(eventType)
		events[i] = Event{
			ID:        eventIDs[i],
			Type:      eventType,
			Data:      data,
			Timestamp: time.Now(),
			Metadata:  meta,
		}
	}

	// Structure to store subscriber and its events
	type subscriberWithEvents struct {
		sub    Subscription
		events map[string]Event // Map of events for this subscriber
	}

	// Map to track unique subscribers by their ID
	uniqueSubs := make(map[string]*subscriberWithEvents)

	eb.mu.RLock()

	// For each event type, find matching patterns and subscribers
	for i, eventType := range eventTypes {
		matchingPatterns := eb.findMatchingPatterns(eventType)

		for _, pattern := range matchingPatterns {
			for _, subscription := range eb.subscribers[pattern] {
				subID := subscription.ID

				if _, exists := uniqueSubs[subID]; !exists {
					uniqueSubs[subID] = &subscriberWithEvents{
						sub:    subscription,
						events: make(map[string]Event),
					}
				}

				uniqueSubs[subID].events[eventType] = events[i]
			}
		}
	}

	eb.mu.RUnlock()

	// Send events to each unique subscriber
	for _, subWithEvents := range uniqueSubs {
		for _, event := range subWithEvents.events {
			select {
			case subWithEvents.sub.Channel <- event:
				// Event sent successfully
			case <-subWithEvents.sub.Done:
				break
			default:
				eb.logger.Printf("Channel for event %s is full, dropping event %s", event.Type, event.ID)
			}
		}
	}

	return eventIDs
}

func (eb *EventBus) PublishEvent(event Event) {

	eb.mu.RLock()
	matchingPatterns := eb.findMatchingPatterns(event.Type)

	var subscriberCopies []Subscription
	for _, pattern := range matchingPatterns {
		subscriberCopies = append(subscriberCopies, eb.subscribers[pattern]...)
	}
	eb.mu.RUnlock()

	for _, subscription := range subscriberCopies {
		select {
		case subscription.Channel <- event:
			// Event sent successfully
		case <-subscription.Done:
			// Subscriber completed, skip sending
		default:
			// Channel is full, log event loss
			eb.logger.Printf("Channel for event %s is full, dropping event %s", event.Type, event.ID)
		}
	}
}

func (eb *EventBus) findMatchingPatterns(eventType string) []string {
	var matches []string

	for pattern := range eb.subscribers {
		if pattern == eventType {
			matches = append(matches, pattern)
		} else if eb.patternTree.MatchesPattern(pattern, eventType) {
			matches = append(matches, pattern)
		}
	}

	return matches
}

func (eb *EventBus) RequestReply(ctx context.Context, requestType string, data interface{}, timeout time.Duration) (interface{}, error) {
	// Generate unique correlation ID
	correlationID := GenerateID(requestType)

	// Define reply event type
	replyType := requestType + ".reply"

	// Create channel for waiting for response
	eb.respMu.Lock()
	replyChan := make(chan Event, 1)
	eb.responseChannels[correlationID] = replyChan
	eb.respMu.Unlock()

	// Clean up response channels on exit
	defer func() {
		eb.respMu.Lock()
		delete(eb.responseChannels, correlationID)
		eb.respMu.Unlock()
	}()

	// Create request metadata
	metadata := map[string]interface{}{
		"correlationID": correlationID,
		"replyTo":       replyType,
	}

	// Create and publish request event
	event := Event{
		ID:            GenerateID(requestType),
		Type:          requestType,
		CorrelationID: correlationID,
		ReplyTo:       replyType,
		Data:          data,
		Timestamp:     time.Now(),
		Metadata:      metadata,
	}

	eb.PublishEvent(event)

	// Create timeout context if not provided
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Wait for response or timeout
	select {
	case reply := <-replyChan:
		// Check for error in response
		if errMsg, ok := reply.Metadata["error"]; ok && errMsg != nil {
			return reply.Data, NewError(errMsg.(string))
		}
		return reply.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (eb *EventBus) DeliverResponse(originalEvent Event, responseData interface{}, err error) {
	// Check for required fields
	if originalEvent.ReplyTo == "" || originalEvent.CorrelationID == "" {
		eb.logger.Printf("Cannot deliver response: missing ReplyTo or CorrelationID in original event %s", originalEvent.ID)
		return
	}

	// Create response metadata
	metadata := map[string]interface{}{
		"correlationID": originalEvent.CorrelationID,
		"requestID":     originalEvent.ID,
	}

	// Add error to metadata if present
	if err != nil {
		metadata["error"] = err.Error()
	}

	// Create response event
	responseEvent := Event{
		ID:            GenerateID(originalEvent.ReplyTo),
		Type:          originalEvent.ReplyTo,
		CorrelationID: originalEvent.CorrelationID,
		Data:          responseData,
		Timestamp:     time.Now(),
		Metadata:      metadata,
	}

	// Check if sender is waiting for response via dedicated channel
	eb.respMu.RLock()
	replyChan, exists := eb.responseChannels[originalEvent.CorrelationID]
	eb.respMu.RUnlock()

	if exists {
		select {
		case replyChan <- responseEvent:
			// Response sent successfully
		default:
			// Channel full or closed, publish to event bus
			eb.logger.Printf("Response channel for correlation ID %s is full, publishing to event bus", originalEvent.CorrelationID)
			eb.Publish(originalEvent.ReplyTo, responseData, metadata)
		}
	} else {
		// Publish to regular event bus
		eb.Publish(originalEvent.ReplyTo, responseData, metadata)
	}
}

// RequestMultiple sends a request to multiple recipients and collects responses
func (eb *EventBus) RequestMultiple(ctx context.Context, requestTypes []string, data interface{}, timeout time.Duration) (map[string]interface{}, []error) {
	results := make(map[string]interface{})
	errors := make([]error, 0)

	var mu sync.Mutex

	var wg sync.WaitGroup

	// Create timeout context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Send requests to all recipients
	for _, reqType := range requestTypes {
		wg.Add(1)
		go func(requestType string) {
			defer wg.Done()

			// Send request with timeout
			response, err := eb.RequestReply(ctx, requestType, data, 0)

			mu.Lock()
			defer mu.Unlock()

			results[requestType] = response
			if err != nil {
				errors = append(errors, NewError(requestType+": "+err.Error()))
			}
		}(reqType)
	}

	// Wait for all requests to complete
	wg.Wait()

	return results, errors
}

func NewError(msg string) error {
	return &customError{message: msg}
}

type customError struct {
	message string
}

func (e *customError) Error() string {
	return e.message
}
