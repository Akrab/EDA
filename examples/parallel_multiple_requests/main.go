package main

import (
	"context"
	eda "github.com/Akrab/EDA-RR"
	"log"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "[Multiple Requests Example] ", log.LstdFlags)
	eventBus := eda.NewEventBus(logger)

	// Start services for handling different aspects of order processing
	setupInventoryService(eventBus, logger)
	setupShippingService(eventBus, logger)
	setupDiscountService(eventBus, logger)

	// Order data
	orderData := map[string]interface{}{
		"order_id":    "ORD-9876",
		"customer_id": "CUST-1234",
		"items": []map[string]interface{}{
			{
				"product_id": "PROD-5678",
				"quantity":   2,
				"price":      39.99,
			},
		},
		"shipping_address": "123 Main St, Anytown, USA",
	}

	// Make parallel requests to different services
	logger.Println("Processing order with multiple services...")
	ctx := context.Background()
	results, errors := eventBus.RequestMultiple(
		ctx,
		[]string{"inventory.check", "shipping.calculate", "discount.apply"},
		orderData,
		200*time.Millisecond,
	)

	// Check for errors
	if len(errors) > 0 {
		logger.Printf("Encountered %d errors during order processing:", len(errors))
		for _, err := range errors {
			logger.Printf("- Error: %v", err)
		}
	}

	// Process all results
	logger.Println("Order processing results:")
	logger.Printf("- Inventory: %v", results["inventory.check"])
	logger.Printf("- Shipping: %v", results["shipping.calculate"])
	logger.Printf("- Discount: %v", results["discount.apply"])

	// Calculate final price
	var totalPrice float64

	// Base price from items
	items := orderData["items"].([]map[string]interface{})
	for _, item := range items {
		totalPrice += item["price"].(float64) * float64(item["quantity"].(int))
	}

	// Add shipping
	shippingResult := results["shipping.calculate"].(map[string]interface{})
	shippingCost := shippingResult["cost"].(float64)
	totalPrice += shippingCost

	// Apply discount
	discountResult := results["discount.apply"].(map[string]interface{})
	discountAmount := discountResult["discount_amount"].(float64)
	totalPrice -= discountAmount

	logger.Printf("Final order price: $%.2f", totalPrice)
}

func setupInventoryService(eventBus *eda.EventBus, logger *log.Logger) {
	ch, _ := eventBus.Subscribe("inventory.check", 10)

	go func() {
		for event := range ch {
			// Simulate inventory check
			orderData := event.Data.(map[string]interface{})
			items := orderData["items"].([]map[string]interface{})

			allAvailable := true
			for _, item := range items {
				// Simulate check (always available in this example)
				productID := item["product_id"].(string)
				logger.Printf("Checking inventory for %s", productID)
			}

			eventBus.DeliverResponse(event, map[string]interface{}{
				"available": allAvailable,
				"message":   "All items in stock",
			}, nil)
		}
	}()
}

func setupShippingService(eventBus *eda.EventBus, logger *log.Logger) {
	ch, _ := eventBus.Subscribe("shipping.calculate", 10)

	go func() {
		for event := range ch {
			// Simulate shipping calculation
			orderData := event.Data.(map[string]interface{})
			address := orderData["shipping_address"].(string)

			// Simulate calculation
			logger.Printf("Calculating shipping to: %s", address)

			// Add random delay to simulate processing time
			time.Sleep(50 * time.Millisecond)

			eventBus.DeliverResponse(event, map[string]interface{}{
				"cost":          8.99,
				"carrier":       "Fast Shipping Co.",
				"delivery_days": 3,
			}, nil)
		}
	}()
}

func setupDiscountService(eventBus *eda.EventBus, logger *log.Logger) {
	ch, _ := eventBus.Subscribe("discount.apply", 10)

	go func() {
		for event := range ch {
			// Simulate discount calculation
			orderData := event.Data.(map[string]interface{})
			customerID := orderData["customer_id"].(string)

			// Simulate loyalty check
			logger.Printf("Checking discounts for customer: %s", customerID)

			eventBus.DeliverResponse(event, map[string]interface{}{
				"discount_amount": 5.00,
				"discount_type":   "LOYALTY",
				"discount_code":   "LOYAL10",
			}, nil)
		}
	}()
}
