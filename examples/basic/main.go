package main

import (
	"log"
	"os"
	"time"

	eda "github.com/Akrab/EDA-RR"
)

func main() {
	logger := log.New(os.Stdout, "[EDA] ", log.LstdFlags)
	eb := eda.NewEventBus(logger)

	// Подписка на событие
	ch, unsub := eb.Subscribe("user.login", 10)
	defer unsub()

	// Публикация события
	eb.Publish("user.login", map[string]string{"username": "test_user"})

	// Получение события
	select {
	case event := <-ch:
		logger.Printf("Получено событие: %v", event)
	case <-time.After(time.Second):
		logger.Printf("Таймаут ожидания события")
	}
}
