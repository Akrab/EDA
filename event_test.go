package edaRR

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

// TestBasicPublishSubscribe tests the basic publish-subscribe functionality
func TestBasicPublishSubscribe(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	ch, unsub := eb.Subscribe("test.event", 10)
	defer unsub()

	expectedData := map[string]string{"message": "hello world"}
	eb.Publish("test.event", expectedData)

	// Wait for event reception
	select {
	case event := <-ch:
		// Check event content
		if event.Type != "test.event" {
			t.Errorf("Expected event type 'test.event', got '%s'", event.Type)
		}

		data, ok := event.Data.(map[string]string)
		if !ok {
			t.Fatalf("Failed to cast event data to map[string]string")
		}

		if data["message"] != "hello world" {
			t.Errorf("Expected message 'hello world', got '%s'", data["message"])
		}

	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event")
	}
}

// TestEventPatterns tests event routing by patterns
func TestEventPatterns(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Structure for counting received messages
	counts := struct {
		wildcard   int
		doubleWild int
		exactMatch int
		noMatch    int
		mu         sync.Mutex
	}{}

	// Subscriptions with different patterns
	wildcardCh, unsub1 := eb.Subscribe("market.*.data", 10)
	doubleWildCh, unsub2 := eb.Subscribe("market.**", 10)
	exactCh, unsub3 := eb.Subscribe("market.BTC.price", 10)
	noMatchCh, unsub4 := eb.Subscribe("user.login", 10)

	defer unsub1()
	defer unsub2()
	defer unsub3()
	defer unsub4()

	// Goroutines for counting events
	var wg sync.WaitGroup
	wg.Add(4)

	// Handler for single wildcard pattern
	go func() {
		defer wg.Done()
		for range wildcardCh {
			counts.mu.Lock()
			counts.wildcard++
			counts.mu.Unlock()
		}
	}()

	// Handler for double wildcard pattern
	go func() {
		defer wg.Done()
		for range doubleWildCh {
			counts.mu.Lock()
			counts.doubleWild++
			counts.mu.Unlock()
		}
	}()

	// Handler for exact match
	go func() {
		defer wg.Done()
		for range exactCh {
			counts.mu.Lock()
			counts.exactMatch++
			counts.mu.Unlock()
		}
	}()

	// Handler for non-matching event
	go func() {
		defer wg.Done()
		for range noMatchCh {
			counts.mu.Lock()
			counts.noMatch++
			counts.mu.Unlock()
		}
	}()

	// Publish events
	eb.Publish("market.BTC.data", "BTC data")
	eb.Publish("market.ETH.data", "ETH data")
	eb.Publish("market.BTC.price", "BTC price")
	eb.Publish("market.BTC.trade", "BTC trade")
	eb.Publish("user.login", "User login")

	// Wait for handlers to process events
	time.Sleep(200 * time.Millisecond)

	// Check results
	counts.mu.Lock()
	defer counts.mu.Unlock()

	if counts.wildcard != 2 {
		t.Errorf("Expected 2 wildcards events, got %d", counts.wildcard)
	}

	if counts.doubleWild != 4 {
		t.Errorf("Expected 4 double wildcard events, got %d", counts.doubleWild)
	}

	if counts.exactMatch != 1 {
		t.Errorf("Expected 1 exact match event, got %d", counts.exactMatch)
	}

	if counts.noMatch != 1 {
		t.Errorf("Expected 1 no match event, got %d", counts.noMatch)
	}
}

func TestFilteredSubscription(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Create filter for events with data.value > 100
	filter := func(event Event) bool {
		data, ok := event.Data.(map[string]interface{})
		if !ok {
			return false
		}

		value, ok := data["value"].(float64)
		if !ok {
			return false
		}

		return value > 100.0
	}

	// Subscribe with filter
	ch, unsub := eb.SubscribeWithFilter("test.value", 10, filter)

	// Counter for received events
	var receivedCount int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for range ch {
			mu.Lock()
			receivedCount++
			mu.Unlock()
		}
	}()

	// Publish events with different values
	eb.Publish("test.value", map[string]interface{}{"value": 50.0})
	eb.Publish("test.value", map[string]interface{}{"value": 150.0})
	eb.Publish("test.value", map[string]interface{}{"value": 75.0})
	eb.Publish("test.value", map[string]interface{}{"value": 200.0})

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	unsub()

	// Wait for goroutine completion
	wg.Wait()

	// Check result - should receive only 2 events with value > 100
	mu.Lock()
	count := receivedCount
	mu.Unlock()
	if count != 2 {
		t.Errorf("Expected 2 filtered events, got %d", count)
	}
}

// TestRequestReply tests synchronous request-response
func TestRequestReply(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Start request handler
	ch, unsub := eb.Subscribe("calc.add", 10)
	defer unsub()

	go func() {
		for event := range ch {
			// Extract request data
			data, ok := event.Data.(map[string]interface{})
			if !ok {
				continue
			}

			a, aOk := data["a"].(float64)
			b, bOk := data["b"].(float64)

			if !aOk || !bOk {
				// Send error
				eb.DeliverResponse(event, nil, NewError("Invalid parameters"))
				continue
			}

			// Calculate result
			result := a + b

			// Send response
			eb.DeliverResponse(event, map[string]interface{}{"result": result}, nil)
		}
	}()

	// Send request
	ctx := context.Background()
	response, err := eb.RequestReply(
		ctx,
		"calc.add",
		map[string]interface{}{"a": 5.0, "b": 3.0},
		100*time.Millisecond,
	)

	// Check result
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	respMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map response, got %T", response)
	}

	result, ok := respMap["result"].(float64)
	if !ok {
		t.Fatalf("Expected float64 result, got %T", respMap["result"])
	}

	if result != 8.0 {
		t.Errorf("Expected result 8.0, got %f", result)
	}

	// Test timeout
	_, err = eb.RequestReply(
		ctx,
		"calc.nonexistent",
		map[string]interface{}{"a": 5.0, "b": 3.0},
		50*time.Millisecond,
	)

	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}
}

// TestRequestMultiple tests multiple requests
func TestRequestMultiple(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Start handlers for different services
	service1Ch, unsub1 := eb.Subscribe("service1.data", 10)
	service2Ch, unsub2 := eb.Subscribe("service2.data", 10)

	defer unsub1()
	defer unsub2()

	// Handler for service 1
	go func() {
		for event := range service1Ch {
			// Reply with delay
			time.Sleep(10 * time.Millisecond)
			eb.DeliverResponse(event, "service1 response", nil)
		}
	}()

	// Handler for service 2
	go func() {
		for event := range service2Ch {
			// Reply with delay
			time.Sleep(20 * time.Millisecond)
			eb.DeliverResponse(event, "service2 response", nil)
		}
	}()

	// Request to multiple services
	ctx := context.Background()
	results, errors := eb.RequestMultiple(
		ctx,
		[]string{"service1.data", "service2.data"},
		"test data",
		100*time.Millisecond,
	)

	// Check results
	if len(errors) > 0 {
		t.Fatalf("Expected no errors, got: %v", errors)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	service1Result, ok1 := results["service1.data"].(string)
	service2Result, ok2 := results["service2.data"].(string)

	if !ok1 || service1Result != "service1 response" {
		t.Errorf("Wrong service1 response: %v", results["service1.data"])
	}

	if !ok2 || service2Result != "service2 response" {
		t.Errorf("Wrong service2 response: %v", results["service2.data"])
	}
}

// TestPublishMultiple tests multiple publishing
func TestPublishMultiple(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	var counts struct {
		subscriber1 int
		subscriber2 int
		mu          sync.Mutex
	}

	// Create two subscribers with overlapping patterns
	ch1, unsub1 := eb.Subscribe("market.**", 10)
	ch2, unsub2 := eb.Subscribe("*.BTC.*", 10)

	defer unsub1()
	defer unsub2()

	// Goroutines for counting events
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range ch1 {
			counts.mu.Lock()
			counts.subscriber1++
			counts.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		for range ch2 {
			counts.mu.Lock()
			counts.subscriber2++
			counts.mu.Unlock()
		}
	}()

	eventTypes := []string{
		"market.BTC.price",
		"market.BTC.volume",
		"market.ETH.price",
	}

	eb.PublishMultiple(eventTypes, "test data")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check results
	counts.mu.Lock()
	defer counts.mu.Unlock()

	// Subscriber 1 should receive all 3 events (market.**)
	if counts.subscriber1 != 3 {
		t.Errorf("Subscriber1 should receive 3 events, got %d", counts.subscriber1)
	}

	// Subscriber 2 should receive 2 events (*.BTC.*)
	if counts.subscriber2 != 2 {
		t.Errorf("Subscriber2 should receive 2 events, got %d", counts.subscriber2)
	}
}

// TestUnsubscribe tests unsubscribe functionality
func TestUnsubscribe(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	ch, unsub := eb.Subscribe("test.event", 10)

	eb.Publish("test.event", "before unsubscribe")

	// Check that event is received
	select {
	case event := <-ch:
		if event.Data != "before unsubscribe" {
			t.Errorf("Expected 'before unsubscribe', got %v", event.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event before unsubscribe")
	}

	unsub()

	eb.Publish("test.event", "after unsubscribe")

	// Check that event is NOT received (channel should be closed)
	select {
	case event, ok := <-ch:
		if ok {
			t.Errorf("Expected channel to be closed, got event: %v", event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel not closed after unsubscribe")
	}
}
