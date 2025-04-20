package eda

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

// TestBasicPublishSubscribe проверяет базовый функционал публикации и подписки
func TestBasicPublishSubscribe(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Создаем подписку
	ch, unsub := eb.Subscribe("test.event", 10)
	defer unsub()

	// Публикуем событие
	expectedData := map[string]string{"message": "hello world"}
	eb.Publish("test.event", expectedData)

	// Ждем получения события
	select {
	case event := <-ch:
		// Проверяем содержимое события
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

// TestEventPatterns проверяет маршрутизацию событий по шаблонам
func TestEventPatterns(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Структура для подсчета полученных сообщений
	counts := struct {
		wildcard   int
		doubleWild int
		exactMatch int
		noMatch    int
		mu         sync.Mutex
	}{}

	// Подписки с разными шаблонами
	wildcardCh, unsub1 := eb.Subscribe("market.*.data", 10)
	doubleWildCh, unsub2 := eb.Subscribe("market.**", 10)
	exactCh, unsub3 := eb.Subscribe("market.BTC.price", 10)
	noMatchCh, unsub4 := eb.Subscribe("user.login", 10)

	defer unsub1()
	defer unsub2()
	defer unsub3()
	defer unsub4()

	// Горутины для подсчета событий
	var wg sync.WaitGroup
	wg.Add(4)

	// Обработчик для одиночного шаблона
	go func() {
		defer wg.Done()
		for range wildcardCh {
			counts.mu.Lock()
			counts.wildcard++
			counts.mu.Unlock()
		}
	}()

	// Обработчик для двойного шаблона
	go func() {
		defer wg.Done()
		for range doubleWildCh {
			counts.mu.Lock()
			counts.doubleWild++
			counts.mu.Unlock()
		}
	}()

	// Обработчик для точного совпадения
	go func() {
		defer wg.Done()
		for range exactCh {
			counts.mu.Lock()
			counts.exactMatch++
			counts.mu.Unlock()
		}
	}()

	// Обработчик для события без совпадения
	go func() {
		defer wg.Done()
		for range noMatchCh {
			counts.mu.Lock()
			counts.noMatch++
			counts.mu.Unlock()
		}
	}()

	// Публикуем события
	eb.Publish("market.BTC.data", "BTC data")
	eb.Publish("market.ETH.data", "ETH data")
	eb.Publish("market.BTC.price", "BTC price")
	eb.Publish("market.BTC.trade", "BTC trade")
	eb.Publish("user.login", "User login")

	// Ждем небольшую задержку, чтобы обработчики успели отработать
	time.Sleep(200 * time.Millisecond)

	// Проверяем результаты
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

	// Создаем фильтр для событий с data.value > 100
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

	// Подписываемся с фильтром
	ch, unsub := eb.SubscribeWithFilter("test.value", 10, filter)

	// Счетчик полученных событий
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

	// Публикуем события с разными значениями
	eb.Publish("test.value", map[string]interface{}{"value": 50.0})
	eb.Publish("test.value", map[string]interface{}{"value": 150.0})
	eb.Publish("test.value", map[string]interface{}{"value": 75.0})
	eb.Publish("test.value", map[string]interface{}{"value": 200.0})

	// Ждем небольшую задержку для обработки
	time.Sleep(100 * time.Millisecond)

	// Отписываемся, чтобы закрыть горутину
	unsub()

	// Ждем завершения горутины
	wg.Wait()

	// Проверяем результат - должны быть получены только 2 события с value > 100
	mu.Lock()
	count := receivedCount
	mu.Unlock()
	if count != 2 {
		t.Errorf("Expected 2 filtered events, got %d", count)
	}
}

// TestRequestReply проверяет синхронный запрос-ответ
func TestRequestReply(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Запускаем обработчик запросов
	ch, unsub := eb.Subscribe("calc.add", 10)
	defer unsub()

	go func() {
		for event := range ch {
			// Извлекаем данные запроса
			data, ok := event.Data.(map[string]interface{})
			if !ok {
				continue
			}

			a, aOk := data["a"].(float64)
			b, bOk := data["b"].(float64)

			if !aOk || !bOk {
				// Отправляем ошибку
				eb.DeliverResponse(event, nil, NewError("Invalid parameters"))
				continue
			}

			// Вычисляем результат
			result := a + b

			// Отправляем ответ
			eb.DeliverResponse(event, map[string]interface{}{"result": result}, nil)
		}
	}()

	// Отправляем запрос
	ctx := context.Background()
	response, err := eb.RequestReply(
		ctx,
		"calc.add",
		map[string]interface{}{"a": 5.0, "b": 3.0},
		100*time.Millisecond,
	)

	// Проверяем результат
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

	// Проверяем таймаут
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

// TestRequestMultiple проверяет множественные запросы
func TestRequestMultiple(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Запускаем обработчики разных сервисов
	service1Ch, unsub1 := eb.Subscribe("service1.data", 10)
	service2Ch, unsub2 := eb.Subscribe("service2.data", 10)

	defer unsub1()
	defer unsub2()

	// Обработчик для сервиса 1
	go func() {
		for event := range service1Ch {
			// Отвечаем с задержкой
			time.Sleep(10 * time.Millisecond)
			eb.DeliverResponse(event, "service1 response", nil)
		}
	}()

	// Обработчик для сервиса 2
	go func() {
		for event := range service2Ch {
			// Отвечаем с задержкой
			time.Sleep(20 * time.Millisecond)
			eb.DeliverResponse(event, "service2 response", nil)
		}
	}()

	// Запрос к нескольким сервисам
	ctx := context.Background()
	results, errors := eb.RequestMultiple(
		ctx,
		[]string{"service1.data", "service2.data"},
		"test data",
		100*time.Millisecond,
	)

	// Проверяем результаты
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

// TestPublishMultiple проверяет множественную публикацию
func TestPublishMultiple(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Счетчик событий
	var counts struct {
		subscriber1 int
		subscriber2 int
		mu          sync.Mutex
	}

	// Создаем двух подписчиков с перекрывающимися шаблонами
	ch1, unsub1 := eb.Subscribe("market.**", 10)
	ch2, unsub2 := eb.Subscribe("*.BTC.*", 10)

	defer unsub1()
	defer unsub2()

	// Горутины для подсчета событий
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

	// Публикуем несколько связанных событий
	eventTypes := []string{
		"market.BTC.price",
		"market.BTC.volume",
		"market.ETH.price",
	}

	// Используем PublishMultiple
	eb.PublishMultiple(eventTypes, "test data")

	// Ждем небольшую задержку для обработки
	time.Sleep(100 * time.Millisecond)

	// Проверяем результаты
	counts.mu.Lock()
	defer counts.mu.Unlock()

	// Подписчик 1 должен получить все 3 события (market.**)
	if counts.subscriber1 != 3 {
		t.Errorf("Subscriber1 should receive 3 events, got %d", counts.subscriber1)
	}

	// Подписчик 2 должен получить 2 события (*.BTC.*)
	if counts.subscriber2 != 2 {
		t.Errorf("Subscriber2 should receive 2 events, got %d", counts.subscriber2)
	}
}

// TestUnsubscribe проверяет функционал отписки
func TestUnsubscribe(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	eb := NewEventBus(logger)

	// Создаем подписку
	ch, unsub := eb.Subscribe("test.event", 10)

	// Публикуем событие
	eb.Publish("test.event", "before unsubscribe")

	// Проверяем, что событие получено
	select {
	case event := <-ch:
		if event.Data != "before unsubscribe" {
			t.Errorf("Expected 'before unsubscribe', got %v", event.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for event before unsubscribe")
	}

	// Отписываемся
	unsub()

	// Публикуем еще одно событие
	eb.Publish("test.event", "after unsubscribe")

	// Проверяем, что событие НЕ получено (канал должен быть закрыт)
	select {
	case event, ok := <-ch:
		if ok {
			t.Errorf("Expected channel to be closed, got event: %v", event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel not closed after unsubscribe")
	}
}
