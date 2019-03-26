package node_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/sanity-io/litter"

	. "github.com/markelog/trie/node"
)

var _ = Describe("node", func() {
	Describe("New", func() {
		It("should properly create new node", func() {
			new := New("test", 1)

			expected := `&node.Node{
  Key: "test",
  Value: 1,
  Parent: nil,
  ImmediateParent: nil,
  Children: []*node.Node{}
,
  Keys: map[string]*node.Node{
  },
  Leaf: false,
}`

			Expect(litter.Sdump(new)).To(Equal(expected))
		})
	})
})
