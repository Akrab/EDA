package patternTree

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBasicPatternMatching проверяет базовое соответствие шаблонов
func TestBasicPatternMatching(t *testing.T) {
	tree := NewPatternTree()

	// Добавляем простые шаблоны
	tree.AddPattern("market.*.data")
	tree.AddPattern("user.login.success")
	tree.AddPattern("service.status.*")

	tests := []struct {
		eventType string
		expected  bool
	}{
		{"market.BTC.data", true},
		{"market.ETH.data", true},
		{"market.data.BTC", false},  // Не соответствует порядку сегментов
		{"market.BTC.price", false}, // Последний сегмент не соответствует
		{"user.login.success", true},
		{"user.login.error", false}, // Не соответствует последнему сегменту
		{"service.status.up", true},
		{"service.status.down", true},
		{"service.uptime", false}, // Неверное количество сегментов
	}

	for _, test := range tests {
		t.Run(test.eventType, func(t *testing.T) {
			result := tree.Matches(test.eventType)
			if result != test.expected {
				t.Errorf("For event %s, expected %v but got %v",
					test.eventType, test.expected, result)
			}
		})
	}
}

// TestDoubleWildcard проверяет работу с двойной звездочкой (**)
func TestDoubleWildcard(t *testing.T) {
	tree := NewPatternTree()

	// Добавляем шаблоны с **
	tree.AddPattern("market.*.data.**")
	tree.AddPattern("**.critical")
	tree.AddPattern("auth.**")

	tests := []struct {
		eventType string
		expected  bool
	}{
		{"market.BTC.data", false}, // Не хватает сегментов после data
		{"market.BTC.data.volume", true},
		{"market.BTC.data.price.high", true}, // Несколько сегментов после data
		{"system.critical", true},            // ** в начале
		{"app.service.critical", true},       // ** в начале, несколько сегментов
		{"auth", false},                      // ** не соответствует пустой строке здесь
		{"auth.login", true},
		{"auth.login.attempt.failed", true}, // Несколько сегментов после auth
	}

	for _, test := range tests {
		t.Run(test.eventType, func(t *testing.T) {
			result := tree.Matches(test.eventType)
			if result != test.expected {
				t.Errorf("For event %s, expected %v but got %v",
					test.eventType, test.expected, result)
			}
		})
	}
}

// TestComplexPatterns проверяет сложные комбинации шаблонов
func TestComplexPatterns(t *testing.T) {
	tree := NewPatternTree()

	// Сложные шаблоны
	tree.AddPattern("*.**.data")
	tree.AddPattern("market.*.*.data")
	tree.AddPattern("**.*")
	tree.AddPattern("**")

	tests := []struct {
		eventType string
		expected  bool
	}{
		{"app.service.data", true},     // *.**.data
		{"app.service.api.data", true}, // *.**.data
		{"market.BTC.USD.data", true},  // market.*.*.data
		{"any.type.of.event", true},    // **.* и **
		{"single", true},               // **.*
	}

	for _, test := range tests {
		t.Run(test.eventType, func(t *testing.T) {
			result := tree.Matches(test.eventType)
			if result != test.expected {
				t.Errorf("For event %s, expected %v but got %v",
					test.eventType, test.expected, result)
			}
		})
	}
}

// TestCaching проверяет работу кеширования
func TestCaching(t *testing.T) {
	tree := NewPatternTree()

	// Добавляем шаблоны
	tree.AddPattern("market.*.data")
	tree.AddPattern("market.*.data.**")

	// Для измерения времени выполнения
	timeOperation := func(f func() bool) time.Duration {
		start := time.Now()
		f()
		return time.Since(start)
	}

	// Первое обращение (без кеша)
	eventType := "market.BTC.data.volume"
	firstDuration := timeOperation(func() bool {
		return tree.Matches(eventType)
	})

	// Еще одно обращение (уже должно быть в кеше)
	secondDuration := timeOperation(func() bool {
		return tree.Matches(eventType)
	})

	// В реальном мире второй вызов должен быть быстрее, но в тестах
	// разница может быть незаметной. Проверяем только корректность.
	result := tree.Matches(eventType)
	if !result {
		t.Errorf("Expected event %s to match", eventType)
	}

	// Выводим для информации
	t.Logf("First call: %v, Second call: %v", firstDuration, secondDuration)

	// Добавляем новый шаблон - должен сбросить кеш
	tree.AddPattern("new.pattern.*")

	// Проверяем, что после добавления шаблона результаты всё ещё верны
	result = tree.Matches(eventType)
	if !result {
		t.Errorf("After cache invalidation, expected event %s to match", eventType)
	}
}

// TestConcurrency проверяет потокобезопасность
func TestConcurrency(t *testing.T) {
	tree := NewPatternTree()
	tree.AddPattern("market.*.data")
	tree.AddPattern("user.login.*")
	tree.AddPattern("service.*.status")

	// Количество горутин
	numGoroutines := 100
	// Количество операций на горутину
	opsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Запускаем параллельную работу
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

			for j := 0; j < opsPerGoroutine; j++ {
				op := r.Intn(2)

				switch op {
				case 0:
					// Операция проверки
					event := randomEvent(r)
					_ = tree.Matches(event)
				case 1:
					// Операция добавления шаблона (реже)
					if r.Intn(100) < 5 { // 5% шанс
						pattern := randomPattern(r)
						tree.AddPattern(pattern)
					} else {
						event := randomEvent(r)
						_ = tree.Matches(event)
					}

				}

				// Иногда добавляем задержку для большей вариативности
				if r.Intn(100) < 10 {
					time.Sleep(time.Microsecond * time.Duration(r.Intn(100)))
				}
			}
		}(i)
	}

	// Дополнительно проверяем чтение во время записи
	go func() {
		for i := 0; i < 50; i++ {
			tree.Matches("user.login.success")
			time.Sleep(time.Millisecond)
		}
	}()

	// Дожидаемся завершения
	wg.Wait()

	// Финальная проверка на корректность работы
	if !tree.Matches("market.BTC.data") {
		t.Error("Tree should still match 'market.BTC.data' after concurrent operations")
	}
}

// TestEmptyAndEdgeCases проверяет граничные случаи
func TestEmptyAndEdgeCases(t *testing.T) {
	tree := NewPatternTree()

	// Пустое дерево не должно соответствовать ничему
	if tree.Matches("any.event") {
		t.Error("Empty tree should not match any event")
	}

	// Добавляем специфические шаблоны
	tree.AddPattern("*") // Один сегмент
	tree.AddPattern("")  // Пустой шаблон

	// Проверяем одиночный сегмент
	if !tree.Matches("test") {
		t.Error("Single wildcard should match single segment")
	}

	// Проверяем, что многосегментные события не соответствуют одиночному *
	if tree.Matches("test.example") {
		t.Error("Single wildcard should not match multiple segments")
	}

	// Двойная звезда должна соответствовать нескольким сегментам
	tree.AddPattern("**")
	if !tree.Matches("any.number.of.segments") {
		t.Error("Double wildcard should match any segments")
	}

	// Очень длинные события
	longEvent := strings.Repeat("segment.", 100) + "end"
	tree.AddPattern("**.end")
	if !tree.Matches(longEvent) {
		t.Error("Tree should match very long events")
	}

	// Очень длинные шаблоны
	longPattern := strings.Repeat("*.", 100) + "end"
	tree.AddPattern(longPattern)

	longEventMatching := strings.Repeat("test.", 100) + "end"
	if !tree.Matches(longEventMatching) {
		t.Error("Tree should match events with very long patterns")
	}
}

// BenchmarkPatternMatching бенчмарк для измерения производительности
func BenchmarkPatternMatching(b *testing.B) {
	tree := NewPatternTree()
	tree.AddPattern("market.*.data")
	tree.AddPattern("market.*.data.**")
	tree.AddPattern("user.login.*")
	tree.AddPattern("service.*.status")
	tree.AddPattern("**.critical")

	events := []string{
		"market.BTC.data",
		"market.ETH.data.volume",
		"user.login.success",
		"service.web.status",
		"app.service.critical",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		event := events[i%len(events)]
		tree.Matches(event)
	}
}

// BenchmarkOriginalVsTree сравнивает оригинальную функцию с деревом
func BenchmarkOriginalVsTree(b *testing.B) {
	patterns := []string{
		"market.*.data",
		"market.*.data.**",
		"user.login.*",
		"service.*.status",
		"**.critical",
	}

	events := []string{
		"market.BTC.data",
		"market.ETH.data.volume",
		"user.login.success",
		"service.web.status",
		"app.service.critical",
	}

	tree := NewPatternTree()
	for _, pattern := range patterns {
		tree.AddPattern(pattern)
	}

	b.Run("Original", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := events[i%len(events)]
			for _, pattern := range patterns {
				patternMatches(pattern, event)
			}
		}
	})

	b.Run("Tree", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := events[i%len(events)]
			tree.Matches(event)
		}
	})
}

// Вспомогательные функции для генерации случайных событий и шаблонов

func randomEvent(r *rand.Rand) string {
	domains := []string{"market", "user", "service", "app", "system", "data"}
	actions := []string{"create", "update", "delete", "query", "login", "logout", "data", "status"}
	statuses := []string{"success", "error", "pending", "complete", "critical"}

	parts := []string{}

	// Добавляем от 1 до 4 сегментов
	numParts := r.Intn(4) + 1

	for i := 0; i < numParts; i++ {
		switch i {
		case 0:
			parts = append(parts, domains[r.Intn(len(domains))])
		case 1:
			if r.Intn(2) == 0 {
				// Иногда добавляем идентификатор
				parts = append(parts, "id"+strconv.Itoa(r.Intn(100)))
			} else {
				parts = append(parts, actions[r.Intn(len(actions))])
			}
		default:
			parts = append(parts, statuses[r.Intn(len(statuses))])
		}
	}

	return strings.Join(parts, ".")
}

func randomPattern(r *rand.Rand) string {
	domains := []string{"market", "user", "service", "app", "system", "data", "*"}
	actions := []string{"create", "update", "delete", "query", "login", "logout", "data", "status", "*"}
	statuses := []string{"success", "error", "pending", "complete", "critical", "*", "**"}

	parts := []string{}

	// Добавляем от 1 до 4 сегментов
	numParts := r.Intn(4) + 1

	for i := 0; i < numParts; i++ {
		switch i {
		case 0:
			if r.Intn(10) < 8 {
				parts = append(parts, domains[r.Intn(len(domains)-1)])
			} else {
				parts = append(parts, "*")
			}
		case 1:
			parts = append(parts, actions[r.Intn(len(actions))])
		default:
			parts = append(parts, statuses[r.Intn(len(statuses))])
		}
	}

	// Иногда добавляем ** в конец
	if numParts < 4 && r.Intn(10) < 3 {
		parts = append(parts, "**")
	}

	return strings.Join(parts, ".")
}
