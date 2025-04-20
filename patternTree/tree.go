package patternTree

import (
	"strings"
	"sync"
)

type PatternTree struct {
	root     *PatternNode
	mutex    sync.RWMutex    // For thread safety
	cache    map[string]bool // Cache of match results
	patterns map[string]bool // Set of added patterns
}

func NewPatternTree() *PatternTree {
	return &PatternTree{
		root: &PatternNode{
			segment:  "",
			children: make([]*PatternNode, 0),
		},
		cache:    make(map[string]bool),
		patterns: make(map[string]bool),
	}
}

// AddPattern adds a pattern to the tree
func (tree *PatternTree) AddPattern(pattern string) {
	tree.mutex.Lock()
	defer tree.mutex.Unlock()

	// Check if the pattern already exists
	if _, exists := tree.patterns[pattern]; exists {
		// Pattern already exists, do nothing
		return
	}

	// Add the pattern to the set of patterns
	tree.patterns[pattern] = true

	// Clear the cache when adding a new pattern
	tree.cache = make(map[string]bool)

	parts := strings.Split(pattern, ".")
	current := tree.root

	for i, part := range parts {
		var found *PatternNode

		if part == "*" {
			// Handle single wildcard (*)
			if current.wildcardChild == nil {
				current.wildcardChild = &PatternNode{
					segment:  "*",
					children: make([]*PatternNode, 0),
				}
			}
			found = current.wildcardChild
		} else if part == "**" {
			// Handle double wildcard (**)
			if current.doubleWildcardChild == nil {
				current.doubleWildcardChild = &PatternNode{
					segment:  "**",
					children: make([]*PatternNode, 0),
				}
			}
			found = current.doubleWildcardChild
		} else {
			// Handle regular segment
			found = current.findChild(part)

			if found == nil {
				found = &PatternNode{
					segment:  part,
					children: make([]*PatternNode, 0),
				}
				current.addChild(found)
			}
		}

		// If this is the last part of the pattern, mark the node as end of pattern
		if i == len(parts)-1 {
			found.isEndOfPattern = true
		}

		current = found
	}
}

// matchesRecursive is a recursive function for pattern matching
func (tree *PatternTree) matchesRecursive(node *PatternNode, parts []string, index int) bool {
	// If we've processed all parts of the event and are at the end of a pattern
	if index == len(parts) {
		return node.isEndOfPattern
	}

	// Current part of the event
	part := parts[index]

	// 1. Check regular child nodes with binary search
	if child := node.findChild(part); child != nil {
		if tree.matchesRecursive(child, parts, index+1) {
			return true
		}
	}

	// 2. Check single wildcard (*)
	if node.wildcardChild != nil && tree.matchesRecursive(node.wildcardChild, parts, index+1) {
		return true
	}

	// 3. Check double wildcard (**)
	if node.doubleWildcardChild != nil {
		// a) ** matches the current part, and we continue with the same ** node
		if tree.matchesRecursive(node.doubleWildcardChild, parts, index+1) {
			return true
		}

		// b) ** matches this and all remaining parts
		if node.doubleWildcardChild.isEndOfPattern {
			return true
		}

		// c) Check the child nodes of the ** node
		for _, child := range node.doubleWildcardChild.children {
			if (child.segment == part || child.segment == "*") &&
				tree.matchesRecursive(child, parts, index+1) {
				return true
			}
		}

		// d) Special case - ** can match zero segments
		for i := index; i < len(parts); i++ {
			for _, child := range node.doubleWildcardChild.children {
				if (child.segment == parts[i] || child.segment == "*") &&
					tree.matchesRecursive(child, parts, i+1) {
					return true
				}
			}
		}
	}

	return false
}

// Matches checks if an event type matches any pattern in the tree with caching
func (tree *PatternTree) Matches(eventType string) bool {
	tree.mutex.RLock()
	// First check the cache
	if result, exists := tree.cache[eventType]; exists {
		tree.mutex.RUnlock()
		return result
	}
	tree.mutex.RUnlock()

	// If the result is not in the cache, perform the check
	parts := strings.Split(eventType, ".")
	result := tree.matchesRecursive(tree.root, parts, 0)

	// Save the result in the cache
	tree.mutex.Lock()
	defer tree.mutex.Unlock()
	tree.cache[eventType] = result

	return result
}

// Pattern matching function from the question
func (tree *PatternTree) patternMatches(pattern, eventType string) bool {

	if !strings.Contains(pattern, "*") {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	typeParts := strings.Split(eventType, ".")

	if !strings.Contains(pattern, "**") && len(patternParts) != len(typeParts) {
		return false
	}

	patternIndex := 0
	typeIndex := 0

	for patternIndex < len(patternParts) && typeIndex < len(typeParts) {
		// Current pattern segment
		part := patternParts[patternIndex]

		// Case "**" - can match 0 or more segments
		if part == "**" {
			// If this is the last segment of the pattern, then all remaining segments of the type match
			if patternIndex == len(patternParts)-1 {
				return true
			}

			// Find the next non-"**" segment after the current one
			nextPatternIndex := patternIndex + 1
			for nextPatternIndex < len(patternParts) && patternParts[nextPatternIndex] == "**" {
				nextPatternIndex++
			}

			// If there are no other segments after "**", then all remaining segments of the type match
			if nextPatternIndex == len(patternParts) {
				return true
			}

			// Find the next segment in the type that matches the next segment after "**" in the pattern
			nextPattern := patternParts[nextPatternIndex]
			for ; typeIndex < len(typeParts); typeIndex++ {
				// Check if the current type segment matches the next pattern segment
				if nextPattern == "*" || nextPattern == typeParts[typeIndex] {
					// If it matches, check the remaining part recursively
					remaining := strings.Join(patternParts[nextPatternIndex:], ".")
					remainingType := strings.Join(typeParts[typeIndex:], ".")
					if tree.patternMatches(remaining, remainingType) {
						return true
					}
				}
			}

			// If no match is found, return false
			return false

		} else if part == "*" {
			// Case "*" - matches exactly one segment
			patternIndex++
			typeIndex++
		} else {
			// Regular segment - must match exactly
			if part != typeParts[typeIndex] {
				return false
			}
			patternIndex++
			typeIndex++
		}
	}

	// Check that all segments are matched
	// If there are unexamined pattern segments, check if they are all "**"
	for patternIndex < len(patternParts) {
		if patternParts[patternIndex] != "**" {
			return false
		}
		patternIndex++
	}

	// If there are unexamined type segments, return false
	return typeIndex == len(typeParts)
}

// MatchesPattern checks if an event matches a specific pattern
func (tree *PatternTree) MatchesPattern(pattern, eventType string) bool {
	// Form a key for the cache

	if pattern == eventType {
		return true
	}

	cacheKey := pattern + ":" + eventType

	tree.mutex.RLock()
	// Check the cache
	if result, exists := tree.cache[cacheKey]; exists {
		tree.mutex.RUnlock()
		return result
	}
	tree.mutex.RUnlock()

	// If the original function is used as a black box
	result := tree.patternMatches(pattern, eventType)

	// Save the result in the cache
	tree.mutex.Lock()
	defer tree.mutex.Unlock()
	tree.cache[cacheKey] = result

	return result
}

// Example usage
func Example() {
	// Create a pattern tree
	tree := NewPatternTree()

	// Add patterns
	tree.AddPattern("market.*.data")
	tree.AddPattern("market.*.data.**")
	tree.AddPattern("user.login.success")
	tree.AddPattern("user.*.error")

	// Check matches
	// tree.Matches("market.BTC.data") -> true
	// tree.Matches("market.ETH.data.volume") -> true
	// tree.Matches("market.ETH.price") -> false
	// tree.Matches("user.login.success") -> true
	// tree.Matches("user.register.error") -> true

	// Check with caching - repeated calls will be faster
	// tree.Matches("market.BTC.data") -> true (from cache)

}
