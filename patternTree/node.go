package patternTree

import "sort"

// PatternNode represents a node in the pattern tree
type PatternNode struct {
	// Pattern segment (part between dots)
	segment string

	// Flag indicating if this node can be the end of a pattern
	isEndOfPattern bool

	// Sorted regular child nodes (for binary search)
	children []*PatternNode

	// Node with a single wildcard (*)
	wildcardChild *PatternNode

	// Node with a double wildcard (**)
	doubleWildcardChild *PatternNode
}

// addChild adds a regular child node while maintaining sorted order
func (node *PatternNode) addChild(child *PatternNode) {
	// Find the insertion position using binary search
	i := sort.Search(len(node.children), func(i int) bool {
		return node.children[i].segment >= child.segment
	})

	// If such a node already exists, update it
	if i < len(node.children) && node.children[i].segment == child.segment {
		node.children[i].isEndOfPattern = node.children[i].isEndOfPattern || child.isEndOfPattern
		return
	}

	// Insert the new node at the appropriate position
	node.children = append(node.children, nil)
	if i < len(node.children)-1 {
		copy(node.children[i+1:], node.children[i:len(node.children)-1])
	}
	node.children[i] = child
}

// findChild finds a regular child node using binary search
func (node *PatternNode) findChild(segment string) *PatternNode {
	i := sort.Search(len(node.children), func(i int) bool {
		return node.children[i].segment >= segment
	})

	if i < len(node.children) && node.children[i].segment == segment {
		return node.children[i]
	}
	return nil
}
