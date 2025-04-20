package edaRR

import (
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestGenerateIDFormat verifies the correct format of generated IDs
func TestGenerateIDFormat(t *testing.T) {
	// Expected format: {type_code}-{timestamp}-{counter}-{random}
	// Example: moc-152233012-000001-c4d8

	formatRegex := regexp.MustCompile(`^[a-z0-9]+-\d{9}-\d{6}-[a-f0-9]{4}$`)

	samples := []struct {
		eventType string
		expected  string // Expected prefix for type code
	}{
		{"market.order.created", "moc"},
		{"user.login", "ul"},
		{"system.startup", "ss"},
		{"data.processing.job.started", "dpjs"},
		{"a.b.c.d.e.f", "abcd"}, // Should truncate to 4 chars
	}

	for _, sample := range samples {
		id := GenerateID(sample.eventType)

		// Test overall format
		if !formatRegex.MatchString(id) {
			t.Errorf("ID %s for event type %s does not match expected format",
				id, sample.eventType)
		}

		// Test type code
		parts := strings.Split(id, "-")
		if len(parts) != 4 {
			t.Errorf("ID %s should have 4 parts separated by hyphens", id)
			continue
		}

		typeCode := parts[0]
		if sample.eventType == "a.b.c.d.e.f" {
			// Special case for truncation
			if typeCode != "abcd" {
				t.Errorf("Expected truncated type code 'abcd' for %s, got %s",
					sample.eventType, typeCode)
			}
		} else if !strings.HasPrefix(typeCode, sample.expected) {
			t.Errorf("Type code for %s should start with %s, got %s",
				sample.eventType, sample.expected, typeCode)
		}
	}
}

// TestGenerateIDUniqueness verifies that generated IDs are unique
func TestGenerateIDUniqueness(t *testing.T) {
	// Test uniqueness for a single event type
	eventType := "test.event"
	count := 1000
	ids := make(map[string]bool, count)

	for i := 0; i < count; i++ {
		id := GenerateID(eventType)
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	// Test uniqueness for different event types
	eventTypes := []string{
		"user.login",
		"user.logout",
		"market.order.create",
		"market.order.cancel",
	}

	multiIds := make(map[string]bool, len(eventTypes)*10)
	for _, eventType := range eventTypes {
		for i := 0; i < 10; i++ {
			id := GenerateID(eventType)
			if multiIds[id] {
				t.Errorf("Duplicate ID generated across event types: %s", id)
			}
			multiIds[id] = true
		}
	}
}

// TestGenerateIDCounter checks that the counter increments correctly
func TestGenerateIDCounter(t *testing.T) {
	eventType := "counter.test"

	// Generate several IDs for the same event type
	id1 := GenerateID(eventType)
	id2 := GenerateID(eventType)
	id3 := GenerateID(eventType)

	// Extract counters
	parts1 := strings.Split(id1, "-")
	parts2 := strings.Split(id2, "-")
	parts3 := strings.Split(id3, "-")

	if len(parts1) < 3 || len(parts2) < 3 || len(parts3) < 3 {
		t.Fatal("Generated IDs do not have expected format")
	}

	counter1 := parts1[2]
	counter2 := parts2[2]
	counter3 := parts3[2]

	// Check that counters are incrementing
	if counter1 == counter2 || counter2 == counter3 {
		t.Errorf("Counters should be different: %s, %s, %s",
			counter1, counter2, counter3)
	}
}

// TestGenerateIDConcurrency tests the function under concurrent usage
func TestGenerateIDConcurrency(t *testing.T) {
	var wg sync.WaitGroup
	eventType := "concurrent.test"
	count := 100

	// Use a channel to collect IDs from goroutines
	idChan := make(chan string, count)

	// Launch multiple goroutines to generate IDs
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := GenerateID(eventType)
			idChan <- id
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(idChan)

	// Check for duplicates
	ids := make(map[string]bool)
	for id := range idChan {
		if ids[id] {
			t.Errorf("Duplicate ID generated in concurrent execution: %s", id)
		}
		ids[id] = true
	}

	// Verify we got the expected number of IDs
	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}
