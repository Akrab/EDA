package main

import (
	eda "github.com/Akrab/EDA-RR"
	"log"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "[Pattern Example] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Подписка на шаблоны с использованием wildcards
	wildcard, unsub1 := eventBus.Subscribe("market.*.price", 10)
	defer unsub1()

	doubleWild, unsub2 := eventBus.Subscribe("user.**", 10)
	defer unsub2()

	// Обработка событий с шаблоном market.*.price
	go func() {
		for event := range wildcard {
			logger.Printf("Получено событие цены: %s = %v",
				event.Type, event.Data)
		}
	}()

	// Обработка всех событий пользователя
	go func() {
		for event := range doubleWild {
			logger.Printf("Получено событие пользователя: %s", event.Type)
		}
	}()

	// Публикация событий
	logger.Println("Публикация событий...")

	eventBus.Publish("market.BTC.price", 45000.0)
	eventBus.Publish("market.ETH.price", 3200.0)
	eventBus.Publish("user.login", "login event")
	eventBus.Publish("user.profile.update", "profile update event")

	time.Sleep(200 * time.Millisecond)
	logger.Println("Пример завершен")
}
