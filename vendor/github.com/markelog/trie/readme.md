# Trie [![Build Status](https://travis-ci.org/markelog/trie.svg?branch=master)](https://travis-ci.org/markelog/trie) [![GoDoc](https://godoc.org/github.com/markelog/trie?status.svg)](https://godoc.org/github.com/markelog/trie) [![Go Report Card](https://goreportcard.com/badge/github.com/markelog/trie)](https://goreportcard.com/report/github.com/markelog/trie) [![Coverage ](https://coveralls.io/repos/github/markelog/trie/badge.svg?branch=master)](https://coveralls.io/github/markelog/trie?branch=master)

> Implementation of "Trie" data structure

See https://en.wikipedia.org/wiki/Trie. Why yet another "trie" implementation?

Some of them do not expose needed methods like traversing, some of them do not expose
parents or children. And some of them just plain peculiar. At least I think so :)

## Install
```sh
go get github.com/markelog/trie
```

## Usage

Simple example here, see [docs](https://godoc.org/github.com/markelog/trie) for more stuff and explanations


```go
package main

import (
	"github.com/markelog/trie"
	"github.com/markelog/trie/node"
)

func main() {
	tree := trie.New()

	tree.Add("cool", "So cool")
	tree.Add("coolio", 54) // Age of his at the moment of writing

	println(tree.Size) // 2

	println(tree.Contains("cool")) // true

	println(len(tree.Root.Children))   // 1
	println(tree.Root.Children[0].Key) // "cool"

	results := tree.Search("cool") // []string{"cool", "coolio"}
	cool := results[0]

	println(cool.Value.(string))  // "So cool"
	println(cool.Children[0].Key) // "coolio"

	coolio := tree.Find("coolio")

	println(coolio.Value.(int)) // 54

	println(coolio.Key)        // "coolio"
	println(coolio.Parent.Key) // "cool"

	i := 0
	tree.Traverse(func(item *node.Node) bool {
		i++
		return true
	})

	println(i) // 2

	i = 0
	tree.Visit(cool, func(item *node.Node) bool {
		i++
		return true
	})

	println(i) // 1

	// Including branches
	i = 0
	tree.VisitAll(cool, func(item *node.Node) bool {
		i++
		return true
	})

	println(i) // 2
}


```
