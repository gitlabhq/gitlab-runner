// Package node implements data value intendent to be used in the trie package
package node

// Node stores essential node data
type Node struct {

	// Key of the node
	Key string

	// Value of the node
	Value interface{}

	// Parent leaf of the node
	Parent *Node

	// ImmediateParent might be a branch or a leaf
	ImmediateParent *Node

	// Children of the node
	Children []*Node

	// Keys of children
	Keys map[string]*Node

	// Leaf, is it?
	Leaf bool
}

// New returns new Node (so we wouldn't have to define so many params)
func New(key string, value interface{}) *Node {
	return &Node{
		Key:      key,
		Value:    value,
		Parent:   nil,
		Children: []*Node{},
		Keys:     map[string]*Node{},
		Leaf:     false,
	}
}
