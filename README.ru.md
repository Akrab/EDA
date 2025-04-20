# EDA-RR

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


EDA-RR - это простая библиотека для работы с событиями в Go, выделенная из личного проекта. Она предоставляет базовую функциональность для организации взаимодействия между компонентами приложения через события.

## Возможности

- **Publish-Subscribe**: Асинхронная публикация и подписка на события
- **Шаблоны маршрутизации**: Поддержка одиночных (`*`) и множественных (`**`) wildcard-ов
- **Фильтрация событий**: Подписка на события с фильтрацией по содержимому
- **Request-Response**: Синхронное взаимодействие между компонентами
- **Множественная публикация**: Публикация события в несколько топиков
- **Множественные запросы**: Параллельная отправка запросов нескольким получателям

## Установка

```bash
go get github.com/Akrab/EDA-RR
```
## Быстрый старт

```go
package main

import (
    "log"
    "os"
    "time"
    
    "github.com/Akrab/EDA-RR"
)

func main() {
    // Создаем шину событий
    eventBus := eda.NewEventBus(log.New(os.Stdout, "[EDA] ", log.LstdFlags))
    
    // Подписываемся на события
    ch, unsubscribe := eventBus.Subscribe("user.login", 10)
    defer unsubscribe()
    
    // Обрабатываем события
    go func() {
        for event := range ch {
            log.Printf("Получено событие: %v", event)
        }
    }()
    
    // Публикуем событие
    eventBus.Publish("user.login", map[string]string{
        "username": "john_doe",
    })
    
    time.Sleep(100 * time.Millisecond)
}
```
## Примеры использования

Смотрите директорию [examples](./examples) для примеров:

- [Базовое использование](./examples/basic/main.go)
- [Шаблоны маршрутизации](./examples/patterns/main.go)
- [Запрос-ответ](./examples/request-reply/main.go)
- [Публикация в несколько топиков](./examples/publishing_multiple_topics/main.go)
- [Параллельный запрос-ответ](./examples/parallel_multiple_requests/main.go)
## Документация
### Основные концепции

- **Событие (Event)**: Содержит идентификатор, тип, данные и метаданные

- **Подписка (Subscription)**: Связывает шаблон события с каналом обработки

- **Шаблон (Pattern)**: Определяет правила маршрутизации событий
    - `user.login` - точное совпадение
    - `user.*` - любое событие с префиксом "user." и одним сегментом
    - `user.**` - любое событие с префиксом "user." и любым количеством сегментов

#### MIT. Без гарантий и ответственности.