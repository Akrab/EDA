# EDA-RR

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

EDA-RR is a simple event library for Go, extracted from a personal project. It provides basic functionality for organizing communication between application components through events.

## Features

- **Publish-Subscribe**: Asynchronous event publishing and subscription
- **Routing Patterns**: Support for single (`*`) and multiple (`**`) wildcards
- **Event Filtering**: Subscribe to events with content-based filtering
- **Request-Response**: Synchronous communication between components
- **Multiple Publishing**: Publishing events to multiple topics
- **Multiple Requests**: Parallel request dispatching to multiple recipients

## Installation

```bash
go get github.com/Akrab/EDA-RR
```
## Quick Start

```go
package main

import (
  "log"
  "os"
  "time"

  "github.com/Akrab/EDA-RR"
)

func main() {
  // Create an event bus
  eventBus := eda.NewEventBus(log.New(os.Stdout, "[EDA] ", log.LstdFlags))

  // Subscribe to events
  ch, unsubscribe := eventBus.Subscribe("user.login", 10)
  defer unsubscribe()

  // Process events
  go func() {
    for event := range ch {
      log.Printf("Received event: %v", event)
    }
  }()

  // Publish an event
  eventBus.Publish("user.login", map[string]string{
    "username": "john_doe",
  })

  time.Sleep(100 * time.Millisecond)
}
```
## Usage Examples

See the [examples](./examples) directory for samples:

- [Basic Usage](./examples/basic/main.go)
- [Pattern Routing](./examples/patterns/main.go)
- [Request-Response](./examples/request-reply/main.go)
## Documentation
### Core Concepts

- **Event**: Contains identifier, type, data, and metadata

- **Subscription**:  Links an event pattern to a processing channel

- **Pattern**:  Defines event routing rules
    - `user.login` - exact match
    - `user.*` - any event with "user." prefix and one segment
    - `user.**` -  any event with "user." prefix and any number of segments

#### MIT. No warranties or liabilities.