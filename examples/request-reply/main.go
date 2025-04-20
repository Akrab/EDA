package main

import (
	"context"
	eda "github.com/Akrab/EDA-RR"
	"log"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "[Request-Reply Example] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Запускаем сервис калькулятора
	ch, unsub := eventBus.Subscribe("calc.add", 10)
	defer unsub()

	go func() {
		for event := range ch {
			logger.Printf("Получен запрос: %s", event.ID)

			// Извлекаем параметры
			data, ok := event.Data.(map[string]interface{})
			if !ok {
				eventBus.DeliverResponse(event, nil, eda.NewError("Invalid data format"))
				continue
			}

			a, aOk := data["a"].(float64)
			b, bOk := data["b"].(float64)

			if !aOk || !bOk {
				eventBus.DeliverResponse(event, nil, eda.NewError("Invalid parameters"))
				continue
			}

			// Вычисляем результат
			result := a + b
			logger.Printf("Результат: %.2f + %.2f = %.2f", a, b, result)

			// Отправляем ответ
			eventBus.DeliverResponse(event, map[string]interface{}{
				"result": result,
			}, nil)
		}
	}()

	// Отправляем запрос
	logger.Println("Отправка запроса...")
	ctx := context.Background()
	response, err := eventBus.RequestReply(
		ctx,
		"calc.add",
		map[string]interface{}{"a": 5.0, "b": 3.0},
		100*time.Millisecond,
	)

	if err != nil {
		logger.Fatalf("Ошибка запроса: %v", err)
	}

	// Обрабатываем ответ
	respMap, ok := response.(map[string]interface{})
	if !ok {
		logger.Fatalf("Некорректный формат ответа")
	}

	result, ok := respMap["result"].(float64)
	if !ok {
		logger.Fatalf("Результат не найден")
	}

	logger.Printf("Получен ответ: %.2f", result)
}
