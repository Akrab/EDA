package main

import (
	"context"
	eda "github.com/Akrab/EDA-RR"
	"log"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "[Marketplace Example] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Start the inventory service
	ch, unsub := eventBus.Subscribe("inventory.check", 10)
	defer unsub()

	go func() {
		for event := range ch {
			logger.Printf("Received inventory check request: %s", event.ID)

			// Extract parameters
			data, ok := event.Data.(map[string]interface{})
			if !ok {
				eventBus.DeliverResponse(event, nil, eda.NewError("Invalid request format"))
				continue
			}

			productID, ok := data["product_id"].(string)
			if !ok {
				eventBus.DeliverResponse(event, nil, eda.NewError("Missing product ID"))
				continue
			}

			quantity, ok := data["quantity"].(float64)
			if !ok {
				eventBus.DeliverResponse(event, nil, eda.NewError("Invalid quantity format"))
				continue
			}

			// Simulate database check
			available := true
			var stock float64 = 100 // Simulated inventory level

			if quantity > stock {
				available = false
			}

			// Send response
			logger.Printf("Product %s availability check: %v (requested: %.0f, in stock: %.0f)",
				productID, available, quantity, stock)

			eventBus.DeliverResponse(event, map[string]interface{}{
				"product_id": productID,
				"available":  available,
				"stock":      stock,
			}, nil)
		}
	}()

	// Send inventory check request
	logger.Println("Sending inventory check request...")
	ctx := context.Background()
	response, err := eventBus.RequestReply(
		ctx,
		"inventory.check",
		map[string]interface{}{
			"product_id": "PROD-12345",
			"quantity":   5.0,
		},
		100*time.Millisecond,
	)

	if err != nil {
		logger.Fatalf("Request error: %v", err)
	}

	// Process the response
	respMap, ok := response.(map[string]interface{})
	if !ok {
		logger.Fatalf("Invalid response format")
	}

	available, ok := respMap["available"].(bool)
	if !ok {
		logger.Fatalf("Invalid availability data")
	}

	if available {
		logger.Printf("Product is available! Stock: %.0f", respMap["stock"].(float64))
	} else {
		logger.Printf("Product is not available in requested quantity")
	}
}
