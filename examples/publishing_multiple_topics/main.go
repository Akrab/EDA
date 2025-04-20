package main

import (
	eda "github.com/Akrab/EDA-RR"
	"log"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "[Multiple Publish Example] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Subscribe to catalog updates
	catalogCh, unsub1 := eventBus.Subscribe("catalog.price.update", 10)
	defer unsub1()

	// Subscribe to product events
	productCh, unsub2 := eventBus.Subscribe("product.*.event", 10)
	defer unsub2()

	// Subscribe to analytics events
	analyticsCh, unsub3 := eventBus.Subscribe("analytics.pricing.**", 10)
	defer unsub3()

	// Process events
	go func() {
		for event := range catalogCh {
			data := event.Data.(map[string]interface{})
			logger.Printf("CATALOG: Updating price for product %s to $%.2f",
				data["product_id"], data["new_price"])
		}
	}()

	go func() {
		for event := range productCh {
			data := event.Data.(map[string]interface{})
			logger.Printf("NOTIFICATION: Price change for product %s from $%.2f to $%.2f",
				data["product_id"], data["old_price"], data["new_price"])
		}
	}()

	go func() {
		for event := range analyticsCh {
			data := event.Data.(map[string]interface{})
			logger.Printf("ANALYTICS: Recorded price change of %.1f%% for product %s",
				((data["new_price"].(float64)/data["old_price"].(float64))-1)*100,
				data["product_id"])
		}
	}()

	// Create product price update data
	priceUpdateData := map[string]interface{}{
		"product_id": "PROD-5678",
		"old_price":  49.99,
		"new_price":  39.99,
		"timestamp":  time.Now(),
	}

	// Publish to multiple topics
	eventTypes := []string{
		"catalog.price.update",
		"product.price.event",
		"analytics.pricing.change",
	}

	logger.Println("Publishing price update to multiple topics...")
	eventIDs := eventBus.PublishMultiple(eventTypes, priceUpdateData)

	logger.Printf("Published to %d topics with IDs: %v", len(eventIDs), eventIDs)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}
