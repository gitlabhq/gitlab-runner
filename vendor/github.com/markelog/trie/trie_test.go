package trie

import (
	"github.com/markelog/trie/node"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("trie", func() {
	Describe("Contains() and Add()", func() {
		It("should return the new node", func() {
			var (
				trie     = New()
				node     = trie.Add("test", 1)
				expected = trie.Search("test")[0]
			)

			Expect(node).To(BeIdenticalTo(expected))
		})

		It("should add node to the root", func() {
			trie := New()

			trie.Add("test", 1)

			Expect(trie.Root.Children).To(HaveLen(1))
			Expect(trie.Root.Children[0].Value.(int)).To(Equal(1))
			Expect(trie.Root.Children[0].Parent).To(Equal(trie.Root))
		})

		It("should add nodes in correct order", func() {
			trie := New()
			node := trie.Add("t", 1)
			trie.Add("target", 1)

			Expect(node.Keys["ta"].Keys["tar"].Keys["targ"].Keys["targe"]).ToNot(BeNil())
		})

		It("should correctly build a tree for dependant nodes", func() {
			trie := New()

			trie.Add("a", nil)
			trie.Add("ab", nil)
			trie.Add("abcd", nil)

			children := trie.Root.Children

			Expect(trie.Size).To(Equal(3))
			Expect(children).To(HaveLen(1))
			Expect(children[0].Children).To(HaveLen(1))

			abcd := children[0].Children[0].Children[0]
			Expect(abcd.Key).To(Equal("abcd"))
			Expect(abcd.ImmediateParent.Key).To(Equal("abc"))
			Expect(abcd.ImmediateParent.Children).To(HaveLen(0))
		})

		It("should add child with longer key", func() {
			trie := New()

			trie.Add("a", nil)
			trie.Add("abcd", nil)

			target := trie.Find("a")

			Expect(target.Children).To(HaveLen(1))
			Expect(target.Children[0].Key).To(Equal("abcd"))
		})

		It("should set proper immediate parent", func() {
			trie := New()

			trie.Add("a", nil)
			abcd := trie.Add("abcd", nil)

			Expect(abcd.ImmediateParent.Key).To(Equal("abc"))
		})

		It("should replace branch with the leaf", func() {
			trie := New()

			trie.Add("a", nil)

			abcd := trie.Add("abcd", nil)
			abc := trie.Add("abc", nil)

			Expect(abc.Parent.Key).To(Equal("a"))

			Expect(abc.Parent.Leaf).To(Equal(true))
			Expect(abc.ImmediateParent.Leaf).To(Equal(false))
			Expect(abc.ImmediateParent.Key).To(Equal("ab"))

			Expect(abcd.Parent.Leaf).To(Equal(true))
			Expect(abcd.ImmediateParent.Leaf).To(Equal(true))
			Expect(abcd.ImmediateParent.Key).To(Equal("abc"))
		})

		It("should replace branch with the leaf and find it afterwards", func() {
			trie := New()

			trie.Add("a", nil)

			trie.Add("abcd", nil)
			trie.Add("abc", nil)

			Expect(trie.Find("abc").Key).To(Equal("abc"))
			Expect(trie.Find("abc").Leaf).To(BeTrue())
		})

		It("should correctly build a tree with intermediate nodes", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("avqd", "foo")
			trie.Add("ac", 3)

			a := trie.Root.Children[0]

			Expect(a.Children).To(HaveLen(3))
			Expect(a.Keys).To(HaveLen(3))

			av := a.Keys["av"]

			Expect(av.Keys["avq"].Parent).To(Equal(av))
		})

		It("should add node to the trie", func() {
			trie := New()

			trie.Add("test", 1)

			Expect(trie.Contains("test")).To(Equal(true))
		})

		It("should not show false positive for non-existent branch/leaf", func() {
			trie := New()

			Expect(trie.Contains("test")).To(Equal(false))
		})

		It("should not show false positive for non-existent leaf", func() {
			trie := New()

			trie.Add("test", 1)

			Expect(trie.Contains("tes")).To(Equal(false))
		})
	})

	Describe("Remove", func() {
		It("should not change the tree if key is not present", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Remove("tset")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should not change the tree if key is not present", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Remove("tset")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should not remove the key with half prefix", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Remove("t")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should nullify parent of the target", func() {
			trie := New()

			trie.Add("t", nil)
			target := trie.Add("target", nil)

			trie.Remove("target")

			Expect(target.Parent).To(BeNil())
			Expect(target.ImmediateParent).To(BeNil())
		})

		It("should change the tree size", func() {
			trie := New()

			trie.Add("t", nil)
			trie.Add("target", nil)

			trie.Remove("target")

			Expect(trie.Size).To(Equal(1))
		})

		It("should remove child from parent of the target", func() {
			trie := New()
			parent := trie.Add("t", nil)
			trie.Add("target", nil)

			trie.Remove("target")

			Expect(parent.Keys).To(BeEmpty())
			Expect(parent.Children).To(BeEmpty())
		})

		It("should remove all references from the branches", func() {
			trie := New()

			target := trie.Add("target", nil)

			trie.Remove("target")

			Expect(target.Parent).To(BeNil())
			Expect(target.ImmediateParent).To(BeNil())
		})

		It("should not remove the full subtree of there is still children in it", func() {
			trie := New()
			parent := trie.Add("t", nil)
			trie.Add("tar", nil)
			trie.Add("target", nil)

			trie.Remove("target")

			Expect(parent.Children).To(HaveLen(1))
			Expect(parent.Children[0].Key).To(Equal("tar"))

			ta := parent.Keys["ta"]
			Expect(ta.Key).To(Equal("ta"))
			Expect(ta.Keys["tar"].Key).To(Equal("tar"))

			Expect(ta.Keys["targ"]).To(BeNil())
		})

		It("should still find intermediate leaf", func() {
			trie := New()

			trie.Add("t", nil)
			trie.Add("tar", nil)
			trie.Add("target", nil)

			trie.Remove("target")

			Expect(trie.Search("tar")).To(HaveLen(1))
			Expect(trie.Search("tar")[0].Key).To(Equal("tar"))
		})
	})

	Describe("Yank", func() {
		It("should not change the tree if key is not present", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Yank("tset")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should not change the tree if key is not present", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Yank("tset")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should not remove the key with half prefix", func() {
			trie := New()

			trie.Add("test", 1)

			result := trie.Yank("t")

			Expect(result).To(Equal(false))
			Expect(trie.Size).To(Equal(1))
		})

		It("should nullify parent of the target", func() {
			trie := New()

			trie.Add("t", nil)
			target := trie.Add("target", nil)

			trie.Yank("target")

			Expect(target.Parent).To(BeNil())
			Expect(target.ImmediateParent).To(BeNil())
		})

		It("should remove only specific node", func() {
			trie := New()

			// I don't have good imagination for these names :/
			trie.Add("t", nil)
			trie.Add("targ", nil)
			trie.Add("target", nil)
			trie.Add("targos", nil)
			trie.Add("targetos", nil)

			trie.Yank("targ")

			Expect(trie.Find("target").Parent.Key).To(Equal("t"))
			Expect(trie.Find("targos").Parent.Key).To(Equal("t"))
			Expect(trie.Find("targetos").Parent.Key).To(Equal("target"))

			Expect(trie.Find("targ")).To(BeNil())

			Expect(trie.Size).To(Equal(4))

			exist := false
			isCorrect := false
			trie.VisitAll(trie.Root, func(item *node.Node) bool {
				if item.Key == "targ" {
					exist = true
					isCorrect = item.Leaf == false
					return false
				}

				return true
			})

			Expect(exist).To(Equal(true))
			Expect(isCorrect).To(Equal(true))
		})

		It("should remove specific node and all branches as well", func() {
			trie := New()

			// I don't have good imagination for these names :/
			t := trie.Add("t", nil)
			trie.Add("targetos", nil)

			trie.Yank("targetos")

			Expect(t.Keys).To(HaveLen(0))
			Expect(t.Children).To(HaveLen(0))
			Expect(trie.Size).To(Equal(1))
		})
	})

	Describe("Search", func() {
		It("should find all the words", func() {
			trie := New()

			trie.Add("t", 1)
			trie.Add("te", "foo")
			trie.Add("tes", 3)
			trie.Add("test", 4)

			result := trie.Search("t")

			Expect(result).To(HaveLen(4))

			Expect(result[0].Value.(int)).To(Equal(1))
			Expect(result[1].Value.(string)).To(Equal("foo"))
		})

		It("should not find anything", func() {
			trie := New()

			trie.Add("t", 1)
			trie.Add("te", 1)
			trie.Add("tes", 1)
			trie.Add("test", 1)

			Expect(trie.Search("q")).To(HaveLen(0))
		})

		It("finds cool", func() {
			tree := New()

			tree.Add("cool", "So cool")
			tree.Add("coolio", 54)

			result := tree.Search("cool")

			Expect(result).To(HaveLen(2))
			Expect(result[0].Key).To(Equal("cool"))
			Expect(result[1].Key).To(Equal("coolio"))
		})
	})

	Describe("Find", func() {
		It("should specific word", func() {
			trie := New()

			trie.Add("t", 1)
			trie.Add("te", "foo")
			trie.Add("tes", 3)
			trie.Add("test", 4)

			result := trie.Find("test")

			Expect(result.Value.(int)).To(Equal(4))
			Expect(result.Key).To(Equal("test"))
		})

		It("should not find branches", func() {
			trie := New()

			trie.Add("t", nil)
			trie.Add("test", nil)

			result := trie.Find("tes")

			Expect(result).To(BeNil())
		})

		It("should not find anything", func() {
			trie := New()

			trie.Add("t", 1)
			trie.Add("te", 1)
			trie.Add("tes", 1)
			trie.Add("test", 1)

			Expect(trie.Find("q")).To(BeNil())
		})
	})

	Describe("Traverse", func() {
		It("should traverse all the nodes", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("ac", 3)

			i := 0
			trie.Traverse(func(item *node.Node) bool {
				i++
				return true
			})

			Expect(i).To(Equal(3))
		})

		It("should stop if return value is false", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("ac", 3)

			i := 0
			trie.Traverse(func(item *node.Node) bool {
				i++
				return false
			})

			Expect(i).To(Equal(1))
		})
	})

	Describe("Visit", func() {
		It("should traverse only part of the tree", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("abc", 3)

			noda := trie.Search("ab")[0]

			i := 0
			trie.Visit(noda, func(item *node.Node) bool {
				i++
				return true
			})

			Expect(i).To(Equal(1))
		})

		It("should stop if return value is false", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("ac", 3)

			i := 0
			trie.Visit(trie.Root, func(item *node.Node) bool {
				i++
				return false
			})

			Expect(i).To(Equal(1))
		})
	})

	Describe("VisitAll", func() {
		It("should traverse all parts of the tree", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("abcd", 3)

			noda := trie.Find("ab")

			i := 0
			trie.VisitAll(noda, func(item *node.Node) bool {
				i++
				return true
			})

			Expect(i).To(Equal(2))
		})

		It("should stop if return value is false", func() {
			trie := New()

			trie.Add("a", 1)
			trie.Add("ab", "foo")
			trie.Add("ac", 3)

			i := 0
			trie.VisitAll(trie.Root, func(item *node.Node) bool {
				i++
				return false
			})

			Expect(i).To(Equal(1))
		})
	})

	Describe("internal methods", func() {
		It("index()", func() {
			noda := &node.Node{}
			nodes := []*node.Node{}

			Expect(index(nodes, noda)).To(Equal(-1))
		})

		It("removeKey()", func() {
			noda := &node.Node{}
			nodes := []*node.Node{}

			Expect(removeKey(nodes, noda)).To(Equal(nodes))
		})
	})
})
