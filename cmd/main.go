package main

import (
	"context"
	eda "edaApp"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Initialize EventBus with a logger
	logger := log.New(os.Stdout, "[EventBus] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Example 1: Basic Publish/Subscribe
	fmt.Println("\n=== Basic Publish/Subscribe ===")
	ch1, unsub1 := eventBus.Subscribe("notifications.basic", 10)
	go func() {
		for event := range ch1 {
			fmt.Printf("Basic subscriber received: %v\n", event.Data)
		}
	}()
	eventBus.Publish("notifications.basic", "Hello World")

	// Example 2: Pattern-based subscriptions
	fmt.Println("\n=== Pattern-Based Subscriptions ===")
	// Subscribe to all market BTC events
	ch2, unsub2 := eventBus.Subscribe("market.BTC.*", 10)
	go func() {
		for event := range ch2 {
			fmt.Printf("Pattern subscriber received: %s with data: %v\n",
				event.Type, event.Data)
		}
	}()
	// Publish different events
	eventBus.Publish("market.BTC.price", 57000.0)
	eventBus.Publish("market.BTC.volume", 1234.56)
	eventBus.Publish("market.ETH.price", 2800.0) // Won't be received by our subscriber

	// Example 3: Filtered Subscriptions
	fmt.Println("\n=== Filtered Subscriptions ===")
	// Only receive price updates with value > 50000
	priceFilter := func(event eda.Event) bool {
		price, ok := event.Data.(float64)
		return ok && price > 50000
	}
	ch3, unsub3 := eventBus.SubscribeWithFilter("market.*.price", 10, priceFilter)
	go func() {
		for event := range ch3 {
			fmt.Printf("Filtered subscriber received high price for %s: $%.2f\n",
				event.Type, event.Data)
		}
	}()
	// Publish prices
	eventBus.Publish("market.BTC.price", 57000.0) // Will be received (>50000)
	eventBus.Publish("market.ETH.price", 2800.0)  // Will be filtered out

	// Example 4: Request-Reply pattern
	fmt.Println("\n=== Request-Reply Pattern ===")
	// Start a service
	go func() {
		serviceCh, unsubService := eventBus.Subscribe("service.calculate", 10)
		defer unsubService()

		for event := range serviceCh {
			fmt.Println("Service received calculation request")
			// Extract data
			req := event.Data.(map[string]interface{})
			a := req["a"].(float64)
			b := req["b"].(float64)
			// Calculate and reply
			result := a + b
			eventBus.DeliverResponse(event, result, nil)
		}
	}()

	// Wait for service to start
	time.Sleep(50 * time.Millisecond)

	// Make a request
	ctx := context.Background()
	response, err := eventBus.RequestReply(
		ctx,
		"service.calculate",
		map[string]interface{}{"a": 5.0, "b": 3.0},
		500*time.Millisecond,
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Received calculation result: %v\n", response)
	}

	// Example 5: Multi-Publish
	fmt.Println("\n=== Multi-Publish ===")
	// Subscribe to multiple patterns
	ch5, unsub5 := eventBus.Subscribe("system.**", 10)
	go func() {
		for event := range ch5 {
			fmt.Printf("System subscriber received: %s\n", event.Type)
		}
	}()

	// Publish multiple events at once
	eventBus.PublishMultiple(
		[]string{
			"system.status.updated",
			"system.metrics.cpu",
			"system.metrics.memory",
		},
		map[string]interface{}{
			"timestamp": time.Now(),
			"status":    "healthy",
		},
	)

	// Give time for events to be processed
	time.Sleep(200 * time.Millisecond)

	// Clean up subscriptions
	unsub1()
	unsub2()
	unsub3()
	unsub5()

	fmt.Println("\nEventBus demo completed")
}
