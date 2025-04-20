package patternTree

import (
	"testing"
)

// TestPatternMatching проверяет основную функциональность сопоставления шаблонов
func TestPatternMatching(t *testing.T) {
	tree := NewPatternTree()

	// Добавляем шаблоны
	tree.AddPattern("market.*.data")
	tree.AddPattern("market.*.data.**")
	tree.AddPattern("user.login.success")
	tree.AddPattern("user.*.error")

	// Тестовые случаи
	tests := []struct {
		eventType string
		pattern   string
		expected  bool
	}{
		// Одиночные шаблоны
		{"market.BTC.data", "market.*.data", true},
		{"market.ETH.data", "market.*.data", true},
		{"market.BTC.price", "market.*.data", false},

		// Множественные шаблоны
		{"market.BTC.data.volume", "market.*.data.**", true},
		{"market.BTC.data.price.max", "market.*.data.**", true},
		{"market.BTC.trade.volume", "market.*.data.**", false},

		// Точное совпадение
		{"user.login.success", "user.login.success", true},
		{"user.login.failure", "user.login.success", false},

		// Смешанные шаблоны
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

// TestEdgeCases проверяет граничные случаи и потенциальные проблемы
func TestEdgeCases(t *testing.T) {
	tree := NewPatternTree()

	// Проверка пустых и некорректных шаблонов
	tree.AddPattern("")
	if tree.MatchesPattern("", "some.event") {
		t.Error("Empty pattern should not match any event")
	}

	// Проверка шаблона со всеми символами **
	tree.AddPattern("**.**.**")
	if !tree.MatchesPattern("**.**.**", "a.b.c") {
		t.Error("Pattern with all ** should match any event")
	}

	// Проверка синтаксиса шаблонов
	complexPattern := "a.*.b.**.c.*.d"
	tree.AddPattern(complexPattern)

	validMatches := []string{
		"a.x.b.c.y.d",
		"a.x.b.something.else.c.y.d",
	}

	invalidMatches := []string{
		"a.x.c.y.d",         // отсутствует b
		"a.x.b.something.d", // отсутствует c
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
