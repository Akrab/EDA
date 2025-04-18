package eda

import (
	"edaApp/pkg/patternTree"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID            string                 // Уникальный идентификатор события
	Type          string                 // Тип события
	CorrelationID string                 // ID для связанных событий (например, запрос-ответ)
	ReplyTo       string                 // Тема для ответного события (опционально)
	Data          interface{}            // Полезная нагрузка события
	Timestamp     time.Time              // Время создания события
	Metadata      map[string]interface{} // Дополнительные метаданные
}

type EventBus struct {
	subscribers map[string][]chan Event
	patternTree *patternTree.PatternTree
	mu          sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
		patternTree: patternTree.NewPatternTree(),
	}
}

func (eb *EventBus) Publish(eventType string, data interface{}, metadata ...map[string]interface{}) string {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	id := generateID(eventType)
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

	eb.deliverToSubscribers(eventType, event)

	parts := strings.Split(eventType, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		eb.deliverToSubscribers(prefix, event)
	}

	// Подписки с wildcard (например, "fetch.*.data")
	for pattern := range eb.subscribers {

		if eb.patternTree.MatchesPattern(pattern, eventType) {
			eb.deliverToSubscribers(pattern, event)
		}
	}

	return id
}

func (eb *EventBus) deliverToSubscribers(subscriberType string, event Event) {
	for _, ch := range eb.subscribers[subscriberType] {
		select {
		case ch <- event:
			// Событие отправлено
		default:
			// Канал полон, пропускаем
		}
	}
}

func (eb *EventBus) Subscribe(eventType string, bufferSize int) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.patternTree.AddPattern(eventType)
	ch := make(chan Event, bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}
