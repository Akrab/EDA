package edaRR

import (
	"context"
	"edaApp/patternTree"
	"log"
	"sync"
	"time"
)

type FilterFunc func(Event) bool

type Event struct {
	ID            string                 // Уникальный идентификатор события
	Type          string                 // Тип события
	CorrelationID string                 // ID для связанных событий (например, запрос-ответ)
	ReplyTo       string                 // Тема для ответного события (опционально)
	Data          interface{}            // Полезная нагрузка события
	Timestamp     time.Time              // Время создания события
	Metadata      map[string]interface{} // Дополнительные метаданные
}

type Subscription struct {
	ID      string
	Channel chan Event
	Done    chan struct{} // Канал для сигнализации завершения
}

type EventBus struct {
	// Сопоставление шаблонов с помощью PatternTree
	patternTree *patternTree.PatternTree

	// Каналы подписчиков, индексированные по шаблону
	subscribers map[string][]Subscription

	// Каналы для ожидания ответов по correlation ID
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

	// Создаем новый канал для подписчика
	ch := make(chan Event, bufferSize)
	doneSignal := make(chan struct{})

	subscription := Subscription{
		ID:      generateID(pattern),
		Channel: ch,
		Done:    doneSignal,
	}

	// Добавляем канал в список подписчиков для этого шаблона
	eb.subscribers[pattern] = append(eb.subscribers[pattern], subscription)

	// Добавляем шаблон в дерево шаблонов для эффективного сопоставления
	eb.patternTree.AddPattern(pattern)

	// Функция для отписки
	unsubscribe := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		// Сигнал для завершения всех горутин, связанных с этой подпиской
		close(doneSignal)

		subs := eb.subscribers[pattern]
		for i, sub := range subs {
			if sub.Channel == ch {
				// Удаляем подписку из списка
				eb.subscribers[pattern] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		// Если подписчиков на этот шаблон больше нет, убираем шаблон из дерева
		if len(eb.subscribers[pattern]) == 0 {
			delete(eb.subscribers, pattern)
			// Примечание: удаление шаблона из PatternTree не реализовано в исходном коде
		}

		// Закрываем канал
		close(ch)
	}

	return ch, unsubscribe
}

func (eb *EventBus) SubscribeWithFilter(pattern string, bufferSize int, filter FilterFunc) (<-chan Event, func()) {
	eb.mu.Lock()

	// Добавляем шаблон в дерево шаблонов
	eb.patternTree.AddPattern(pattern)

	// Создаем канал для фильтрованных событий
	filteredCh := make(chan Event, bufferSize)
	doneSignal := make(chan struct{})

	// Для контроля закрытия канала
	var once sync.Once
	closeFilteredCh := func() {
		once.Do(func() {
			close(filteredCh)
		})
	}

	// Создаем канал внутренней подписки с большим буфером
	internalCh := make(chan Event, bufferSize*2)

	subscription := Subscription{
		ID:      generateID("subscription"),
		Channel: internalCh,
		Done:    doneSignal,
	}

	// Добавляем внутренний канал в список подписчиков
	eb.subscribers[pattern] = append(eb.subscribers[pattern], subscription)

	eb.mu.Unlock()

	// Запускаем горутину для фильтрации событий
	go func() {
		// Используем closeFilteredCh вместо прямого закрытия канала
		defer closeFilteredCh()

		for {
			select {
			case event, ok := <-internalCh:
				if !ok {
					// Канал закрыт, выходим из цикла
					return
				}

				// Применяем фильтр и отправляем событие только если оно проходит проверку
				if filter(event) {
					select {
					case filteredCh <- event:
						// Событие отправлено успешно
					case <-doneSignal:
						// Сигнал о завершении
						return
					default:
						// Канал получателя переполнен, логируем
						eb.logger.Printf("Filtered channel for pattern %s is full, dropping event %s", pattern, event.ID)
					}
				}
			case <-doneSignal:
				// Сигнал о завершении работы
				return
			}
		}
	}()

	// Функция отписки
	unsubscribe := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		// Сигнал для завершения горутины фильтрации
		close(doneSignal)

		// Находим и удаляем подписку из списка
		subs := eb.subscribers[pattern]
		for i, sub := range subs {
			if sub.Channel == internalCh {
				// Удаляем подписку из списка
				eb.subscribers[pattern] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		// Если подписчиков на этот шаблон больше нет, убираем шаблон
		if len(eb.subscribers[pattern]) == 0 {
			delete(eb.subscribers, pattern)
		}

		// Закрываем внутренний канал
		close(internalCh)

		// Закрываем фильтрованный канал, используя once для предотвращения двойного закрытия
		closeFilteredCh()
	}

	return filteredCh, unsubscribe
}

func (eb *EventBus) Publish(eventType string, data interface{}, metadata ...map[string]interface{}) string {
	// Используем более короткую блокировку, оптимизация для высокой нагрузки
	eb.mu.RLock()
	matchingPatterns := eb.findMatchingPatterns(eventType)

	// Собираем все релевантные подписки
	var subscriberCopies []Subscription
	for _, pattern := range matchingPatterns {
		subscriberCopies = append(subscriberCopies, eb.subscribers[pattern]...)
	}
	eb.mu.RUnlock()

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

	// Отправляем событие всем подписчикам без удержания блокировки
	for _, subscription := range subscriberCopies {
		select {
		case subscription.Channel <- event:
			// Событие отправлено
		case <-subscription.Done:
			// Подписчик завершил работу, пропускаем отправку
		default:
			// Канал полон, логируем потерю события
			eb.logger.Printf("Channel for event %s is full, dropping event %s", eventType, id)
		}
	}

	return id
}

func (eb *EventBus) PublishMultiple(eventTypes []string, data interface{}, metadata ...map[string]interface{}) []string {
	// Генерируем IDs для всех событий
	eventIDs := make([]string, len(eventTypes))
	events := make([]Event, len(eventTypes))

	// Формируем метаданные
	meta := make(map[string]interface{})
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	// Создаем все события
	for i, eventType := range eventTypes {
		eventIDs[i] = generateID(eventType)
		events[i] = Event{
			ID:        eventIDs[i],
			Type:      eventType,
			Data:      data,
			Timestamp: time.Now(),
			Metadata:  meta,
		}
	}

	// Структура для хранения подписчика и его событий
	type subscriberWithEvents struct {
		sub    Subscription
		events map[string]Event // Карта событий для этого подписчика
	}

	// Карта для отслеживания уникальных подписчиков по их ID
	uniqueSubs := make(map[string]*subscriberWithEvents)

	// Блокировка для согласованного чтения из EventBus
	eb.mu.RLock()

	// Для каждого типа события находим соответствующие шаблоны и подписчиков
	for i, eventType := range eventTypes {
		// Находим все шаблоны, которые соответствуют типу события
		matchingPatterns := eb.findMatchingPatterns(eventType)

		// Для каждого подходящего шаблона добавляем его подписчиков
		for _, pattern := range matchingPatterns {
			for _, subscription := range eb.subscribers[pattern] {
				// Используем ID подписки как уникальный идентификатор
				subID := subscription.ID

				// Создаем запись для этого подписчика, если её ещё нет
				if _, exists := uniqueSubs[subID]; !exists {
					uniqueSubs[subID] = &subscriberWithEvents{
						sub:    subscription,
						events: make(map[string]Event),
					}
				}

				// Добавляем событие в карту (автоматически заменяет, если уже существует)
				uniqueSubs[subID].events[eventType] = events[i]
			}
		}
	}

	eb.mu.RUnlock()

	// Отправляем события каждому уникальному подписчику
	for _, subWithEvents := range uniqueSubs {
		for _, event := range subWithEvents.events {
			select {
			case subWithEvents.sub.Channel <- event:
				// Событие отправлено успешно
			case <-subWithEvents.sub.Done:
				// Подписчик завершил работу, пропускаем
				break
			default:
				// Канал полон, логируем потерю события
				eb.logger.Printf("Channel for event %s is full, dropping event %s", event.Type, event.ID)
			}
		}
	}

	return eventIDs
}

func (eb *EventBus) PublishEvent(event Event) {

	eb.mu.RLock()
	matchingPatterns := eb.findMatchingPatterns(event.Type)

	// Собираем все релевантные подписки
	var subscriberCopies []Subscription
	for _, pattern := range matchingPatterns {
		subscriberCopies = append(subscriberCopies, eb.subscribers[pattern]...)
	}
	eb.mu.RUnlock()

	// Отправляем событие всем подписчикам без удержания блокировки
	for _, subscription := range subscriberCopies {
		select {
		case subscription.Channel <- event:
			// Событие отправлено
		case <-subscription.Done:
			// Подписчик завершил работу, пропускаем отправку
		default:
			// Канал полон, логируем потерю события
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
	// Генерируем уникальный correlation ID
	correlationID := generateID(requestType)

	// Определяем тип ответного события
	replyType := requestType + ".reply"

	// Создаем канал для ожидания ответа
	eb.respMu.Lock()
	replyChan := make(chan Event, 1)
	eb.responseChannels[correlationID] = replyChan
	eb.respMu.Unlock()

	// Очистка каналов ответов при выходе
	defer func() {
		eb.respMu.Lock()
		delete(eb.responseChannels, correlationID)
		eb.respMu.Unlock()
	}()

	// Создаем метаданные запроса
	metadata := map[string]interface{}{
		"correlationID": correlationID,
		"replyTo":       replyType,
	}

	// Публикуем запрос
	event := Event{
		ID:            generateID(requestType),
		Type:          requestType,
		CorrelationID: correlationID,
		ReplyTo:       replyType,
		Data:          data,
		Timestamp:     time.Now(),
		Metadata:      metadata,
	}

	// Отправляем запрос через метод Publish
	eb.PublishEvent(event)

	// Создаем контекст с таймаутом, если не предоставлен
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Ждем ответа или таймаута
	select {
	case reply := <-replyChan:
		// Проверяем на ошибку в ответе
		if errMsg, ok := reply.Metadata["error"]; ok && errMsg != nil {
			return reply.Data, NewError(errMsg.(string))
		}
		return reply.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (eb *EventBus) DeliverResponse(originalEvent Event, responseData interface{}, err error) {
	// Проверяем наличие необходимых полей
	if originalEvent.ReplyTo == "" || originalEvent.CorrelationID == "" {
		eb.logger.Printf("Cannot deliver response: missing ReplyTo or CorrelationID in original event %s", originalEvent.ID)
		return
	}

	// Формируем метаданные ответа
	metadata := map[string]interface{}{
		"correlationID": originalEvent.CorrelationID,
		"requestID":     originalEvent.ID,
	}

	// Если есть ошибка, добавляем ее в метаданные
	if err != nil {
		metadata["error"] = err.Error()
	}

	// Создаем событие-ответ
	responseEvent := Event{
		ID:            generateID(originalEvent.ReplyTo),
		Type:          originalEvent.ReplyTo,
		CorrelationID: originalEvent.CorrelationID,
		Data:          responseData,
		Timestamp:     time.Now(),
		Metadata:      metadata,
	}

	// Сначала проверяем, ожидает ли отправитель ответа через выделенный канал
	eb.respMu.RLock()
	replyChan, exists := eb.responseChannels[originalEvent.CorrelationID]
	eb.respMu.RUnlock()

	if exists {
		// Отправляем напрямую в канал ожидания
		select {
		case replyChan <- responseEvent:
			// Ответ отправлен
		default:
			// Канал полон или закрыт, публикуем в обычную шину
			eb.logger.Printf("Response channel for correlation ID %s is full, publishing to event bus", originalEvent.CorrelationID)
			eb.Publish(originalEvent.ReplyTo, responseData, metadata)
		}
	} else {
		// Публикуем в обычную шину событий
		eb.Publish(originalEvent.ReplyTo, responseData, metadata)
	}
}

// RequestMultiple отправляет запрос нескольким получателям и собирает ответы
func (eb *EventBus) RequestMultiple(ctx context.Context, requestTypes []string, data interface{}, timeout time.Duration) (map[string]interface{}, []error) {
	results := make(map[string]interface{})
	errors := make([]error, 0)

	var mu sync.Mutex

	var wg sync.WaitGroup

	// Создаем контекст с таймаутом
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Отправляем запросы всем получателям
	for _, reqType := range requestTypes {
		wg.Add(1)
		go func(requestType string) {
			defer wg.Done()

			// Отправляем запрос с таймаутом
			response, err := eb.RequestReply(ctx, requestType, data, 0)

			mu.Lock()
			defer mu.Unlock()

			results[requestType] = response
			if err != nil {
				errors = append(errors, NewError(requestType+": "+err.Error()))
			}
		}(reqType)
	}

	// Ждем завершения всех запросов
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
