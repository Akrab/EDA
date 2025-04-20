package patternTree

import (
	"testing"
)

// TestPatternMatching tests the basic functionality of pattern matching
func TestPatternMatching(t *testing.T) {
	tree := NewPatternTree()

	// Add patterns
	tree.AddPattern("market.*.data")
	tree.AddPattern("market.*.data.**")
	tree.AddPattern("user.login.success")
	tree.AddPattern("user.*.error")

	// Test cases
	tests := []struct {
		eventType string
		pattern   string
		expected  bool
	}{
		// Single wildcard patterns
		{"market.BTC.data", "market.*.data", true},
		{"market.ETH.data", "market.*.data", true},
		{"market.BTC.price", "market.*.data", false},

		// Multiple wildcard patterns
		{"market.BTC.data.volume", "market.*.data.**", true},
		{"market.BTC.data.price.max", "market.*.data.**", true},
		{"market.BTC.trade.volume", "market.*.data.**", false},

		// Exact match
		{"user.login.success", "user.login.success", true},
		{"user.login.failure", "user.login.success", false},

		// Mixed patterns
		{"user.register.error", "user.*.error", true},
		{"user.login.error", "user.*.error", true},
		{"user.error", "user.*.error", false},
	}

	for _, test := range tests {
		result := tree.MatchesPattern(test.pattern, test.eventType)
		if result != test.expected {
			t.Errorf("MatchesPattern(%s, %s) = %v, expected %v",
				test.pattern, test.eventType, result, test.expected)
		}
	}
}

// TestEdgeCases tests edge cases and potential issues
func TestEdgeCases(t *testing.T) {
	tree := NewPatternTree()

	// Test empty and invalid patterns
	tree.AddPattern("")
	if tree.MatchesPattern("", "some.event") {
		t.Error("Empty pattern should not match any event")
	}

	// Test pattern with all ** symbols
	tree.AddPattern("**.**.**")
	if !tree.MatchesPattern("**.**.**", "a.b.c") {
		t.Error("Pattern with all ** should match any event")
	}

	// Test pattern syntax
	complexPattern := "a.*.b.**.c.*.d"
	tree.AddPattern(complexPattern)

	validMatches := []string{
		"a.x.b.c.y.d",
		"a.x.b.something.else.c.y.d",
	}

	invalidMatches := []string{
		"a.x.c.y.d",         // missing b
		"a.x.b.something.d", // missing c
	}

	for _, eventType := range validMatches {
		if !tree.MatchesPattern(complexPattern, eventType) {
			t.Errorf("Pattern %s should match %s", complexPattern, eventType)
		}
	}

	for _, eventType := range invalidMatches {
		if tree.MatchesPattern(complexPattern, eventType) {
			t.Errorf("Pattern %s should not match %s", complexPattern, eventType)
		}
	}
}
