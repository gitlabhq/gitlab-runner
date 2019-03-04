// Package trie implements the trie data structure
// See https://en.wikipedia.org/wiki/Trie
package trie

import "github.com/markelog/trie/node"

// Trie essential structure
type Trie struct {

	// Root node
	Root *node.Node

	// Size of the trie (counting only the leafs)
	Size int
}

// Walker for the traverse function
// If such function returns false it will stop traverse
type Walker func(item *node.Node) bool

// New returns new Trie
func New() *Trie {
	return &Trie{
		Root: node.New("", ""),
	}
}

// Add stuff to the trie
func (trie *Trie) Add(key string, value interface{}) (result *node.Node) {
	var (
		current = trie.Root
		parent  = trie.Root
		length  = len(key)
	)

	for i := 1; i < length+1; i++ {
		sKey := key[0:i]

		// Create that new node which we should add
		newNode := node.New(sKey, nil)

		// Specify params of the new node as if its branch node
		newNode.Parent = current
		newNode.ImmediateParent = current
		newNode.Leaf = false

		// Check to see if character node exists in keys children
		if current.Keys[sKey] == nil {
			current.Keys[sKey] = newNode

			// Means we already have node here which might not be leaf
			// So we have to replace it
		} else if sKey == key {
			current.Keys[sKey].Leaf = true
			result = newNode
			newNode.Parent = parent

			break
		}

		// Next one
		current = current.Keys[sKey]

		// Only redefine the parent for the potential leaf when parent is also
		// a leaf - so it would be more enjoyable to traverse
		if current.Leaf {
			parent = current
		}

		// Now if its an actual word - specificy as a leaf,
		// not like vile branch (see above for initial params)
		if sKey == key {
			newNode.Leaf = true
			newNode.Value = value
			newNode.Parent = parent

			parent.Children = append(parent.Children, newNode)

			result = newNode
		}
	}

	// Increment the size of the trie
	trie.Size++

	return
}

// Remove removes subtree of the exact key match
func (trie *Trie) Remove(key string) bool {
	target := trie.Find(key)

	// If there is no key like that in the trie
	if target == nil {
		return false
	}

	parent := target.Parent
	immediate := target.ImmediateParent

	// Remove target references from children
	parent.Children = removeKey(parent.Children, target)

	// Remove target reference from the keys
	delete(immediate.Keys, key)

	for immediate != nil {

		// If we have leafs in the branch we still need that node
		if hasLeafs(immediate.Keys) {
			break
		}

		// Nullify removed elements
		immediate.Parent = nil
		immediate.Keys = map[string]*node.Node{}

		immediate = immediate.ImmediateParent
	}

	// Set parent to nil, since we are no longer part of the trie
	target.Parent = nil
	target.ImmediateParent = nil

	// And decrease the size
	trie.Size--

	return true
}

// Yank removes only one leaf not the subtree.
// Difference with Remove() is that Yank does not removes the leaf subtree,
// still removes the branches though
func (trie *Trie) Yank(key string) bool {
	target := trie.Find(key)

	// If there is no key like that in the trie
	if target == nil {
		return false
	}

	parent := target.Parent

	// Check is we have leafs forward down the tree
	childExist := false
	trie.Visit(target, func(item *node.Node) bool {
		childExist = true
		return false
	})

	// If we does not have anything valuable afterwards, then just remove it
	if childExist == false {
		return trie.Remove(key)
	}

	// Otherwise replace leaf with the branch
	target.Leaf = false
	for _, element := range target.Children {
		element.Parent = parent
	}
	target.Children = []*node.Node{}

	trie.Size--

	return true
}

func hasLeafs(keys map[string]*node.Node) bool {
	for _, element := range keys {
		if element.Leaf {
			return true
		}
	}

	return false
}

// Index returns the index of the node,
// if not found returns the -1
func index(list []*node.Node, node *node.Node) int {
	for index, element := range list {
		if node == element {
			return index
		}
	}

	return -1
}

// removes they key from the list if node is in the list
func removeKey(list []*node.Node, node *node.Node) []*node.Node {
	index := index(list, node)

	// It seems we have nothing to do here?
	if index == -1 {
		return list
	}

	// Put target element to the end,
	// with replacing current index with lastest element
	list[len(list)-1], list[index] = list[index], list[len(list)-1]

	// Now miss the latest removed element
	return list[:len(list)-1]
}

// Contains check presence of the key in the trie
func (trie Trie) Contains(key string) bool {
	var (
		current = trie.Root
		length  = len(key)
	)

	// for every character in the word
	for i := 1; i < length+1; i++ {
		sKey := key[0:i]

		// Check if we have such key, since if its not then we can abort
		if current.Keys[sKey] == nil {
			return false
		}

		// Key exist - proceed to the next depth of the trie
		current = current.Keys[sKey]
	}

	// We finished going through all the words, but is it a whole word?
	return current.Leaf
}

// Search returns every word with the given prefix
func (trie Trie) Search(prefix string) (result []*node.Node) {
	var (
		current = trie.Root
		length  = len(prefix)
	)

	for i := 1; i < length+1; i++ {
		partPrefix := prefix[0:i]

		// If we don't have anything to search for anymore
		if current.Keys[partPrefix] == nil {
			return
		}

		// Proceed forward then
		current = current.Keys[partPrefix]
	}

	result = findAll(current, result)

	return
}

// findAll recursively find the words
func findAll(current *node.Node, result []*node.Node) []*node.Node {
	if current.Leaf {
		result = append(result, current)
	}

	for key := range current.Keys {
		result = findAll(current.Keys[key], result)
	}

	return result
}

// Find specific one full matched key
func (trie Trie) Find(key string) *node.Node {
	var (
		current = trie.Root
		length  = len(key)
	)

	for i := 1; i < length+1; i++ {
		partKey := key[0:i]

		// If we don't have anything anymore - return what we got
		if current.Keys[partKey] == nil {
			return nil
		}

		// Proceed forward then
		current = current.Keys[partKey]

		if partKey == key && current.Leaf {
			return current
		}
	}

	return nil
}

// Traverse the leaves (not the branches)
func (trie Trie) Traverse(fn Walker) {
	trie.Visit(trie.Root, fn)
}

// Visit specific part of the tree
func (trie Trie) Visit(current *node.Node, fn Walker) {
	var (
		children = current.Children
		length   = len(children)
	)

	for i := 0; i < length; i++ {
		result := fn(children[i])
		if result == false {
			return
		}

		trie.Visit(children[i], fn)
	}

	return
}

// VisitAll allows visit every node
func (trie Trie) VisitAll(current *node.Node, fn Walker) {
	keys := current.Keys

	for _, element := range keys {
		result := fn(element)
		if result == false {
			return
		}

		trie.VisitAll(element, fn)
	}

	return
}
