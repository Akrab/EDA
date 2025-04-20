package eda

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	eventCounters = make(map[string]*uint64)
	counterMutex  sync.RWMutex
)

func generateID(eventType string) string {

	counterMutex.Lock()
	counter, exists := eventCounters[eventType]
	if !exists {
		var newCounter uint64 = 0
		eventCounters[eventType] = &newCounter
		counter = &newCounter
	}
	counterMutex.Unlock()

	count := atomic.AddUint64(counter, 1)

	var typeCode string
	parts := strings.Split(eventType, ".")
	for _, part := range parts {
		if len(part) > 0 {
			typeCode += string(part[0])
		}
	}

	if len(typeCode) > 5 {
		typeCode = typeCode[:4]
	}

	now := time.Now()
	timeCode := fmt.Sprintf("%02d%02d%02d%03d",
		now.Hour(), now.Minute(), now.Second(),
		now.Nanosecond()/1000000)

	randomBytes := make([]byte, 2)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	// Format ID: {type_code}-{timestamp}-{counter}-{random}
	// Event type: market.order.created
	// Generated ID: moc-152233012-000001-c4d8
	// Event type: market.order.price.update
	// Generated ID: mopu-152231789-000001-3f2e

	return fmt.Sprintf("%s-%s-%06d-%s", typeCode, timeCode, count%1000000, randomStr)
}
