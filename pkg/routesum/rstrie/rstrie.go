// Package rstrie provides a datatype that supports building a space-efficient summary of networks and IPs.
package rstrie

import (
	"bytes"

	"github.com/PatrickCronin/routesum/pkg/routesum/bitslice"
)

// RSTrie is a radix-like trie of radix 2 whose stored "words" are the binary representations of networks and IPs. An
// optimization rstrie makes over a generic radix tree is that since routes covered by other routes don't need to be
// stored, each node in the trie will have either 0 or 2 children; never 1.
type RSTrie struct {
	root *node
}

type node struct {
	children *[2]*node
	bits     bitslice.BitSlice
}

// NewRSTrie returns an initialized RSTrie for use
func NewRSTrie() *RSTrie {
	return &RSTrie{
		root: nil,
	}
}

// InsertRoute inserts a new BitSlice into the trie. Each insert results in a space-optimized trie structure
// representing its contents. If a route being inserted is already covered by an existing route, it's simply ignored. If
// a route being inserted covers one or more routes already in the trie, those nodes are removed and replaced by the new
// route.
func (t *RSTrie) InsertRoute(routeBits bitslice.BitSlice) {
	// If the trie has no root node, simply create one to store the new route
	if t.root == nil {
		t.root = &node{
			bits:     routeBits,
			children: nil,
		}
		return
	}

	// Otherwise, perform a non-recursive search of the trie's nodes for the best place to insert the route, and do so.
	visited := []*node{}
	curNode := t.root
	remainingRouteBits := routeBits

	for {
		remainingRouteBitsLen := len(remainingRouteBits)
		curNodeBitsLen := len(curNode.bits)

		// Does the requested route cover the current node? If so, update the current node.
		if remainingRouteBitsLen <= curNodeBitsLen && bytes.HasPrefix(curNode.bits, remainingRouteBits) {
			curNode.bits = remainingRouteBits
			curNode.children = nil
			return
		}

		if curNodeBitsLen <= remainingRouteBitsLen && bytes.HasPrefix(remainingRouteBits, curNode.bits) {
			// Does the current node cover the requested route? If so, we're done.
			if curNode.isLeaf() {
				return
			}

			// Otherwise, we traverse to the correct child.
			remainingRouteBits = remainingRouteBits[curNodeBitsLen:]
			visited = append(visited, curNode)
			curNode = curNode.children[remainingRouteBits[0]]
			continue
		}

		// Otherwise the requested route diverges from the current node. We'll need to split the current node.

		// As an optimization, if the split would result in a new node whose children represent a complete subtrie, we
		// just update the current node, instead of allocating new nodes and optimizing them away immediately after.
		if curNode.isLeaf() &&
			curNodeBitsLen == remainingRouteBitsLen &&
			commonPrefixLen(curNode.bits, remainingRouteBits) == len(curNode.bits)-1 {
			curNode.bits = curNode.bits[:len(curNode.bits)-1]
			curNode.children = nil
		} else {
			newNode := splitNodeForRoute(curNode, remainingRouteBits)
			visitedLen := len(visited)
			if visitedLen == 0 {
				t.root = newNode
			} else {
				visited[visitedLen-1].children[newNode.bits[0]] = newNode
			}
		}

		simplifyVisitedSubtries(visited)
		return
	}
}

func (n *node) childrenAreCompleteSubtrie() bool {
	if n.isLeaf() {
		return false
	}

	if !n.children[0].isLeaf() || !n.children[1].isLeaf() {
		return false
	}

	if len(n.children[0].bits) != 1 || len(n.children[1].bits) != 1 {
		return false
	}

	return true
}

func (n *node) isLeaf() bool {
	return n.children == nil
}

func splitNodeForRoute(oldNode *node, routeBits bitslice.BitSlice) *node {
	commonBitsLen := commonPrefixLen(oldNode.bits, routeBits)
	commonBits := oldNode.bits[:commonBitsLen]

	routeNode := &node{
		bits:     routeBits[commonBitsLen:],
		children: nil,
	}
	oldNode.bits = oldNode.bits[commonBitsLen:]

	newNode := &node{
		bits:     commonBits,
		children: &[2]*node{},
	}
	newNode.children[routeNode.bits[0]] = routeNode
	newNode.children[oldNode.bits[0]] = oldNode

	return newNode
}

// A completed subtrie is a node in the trie whose children when taken together represent the complete subtrie below the
// node. For example, if a node represented the route "00", and it had a child for "0" and a child for "1", the node
// would be representing the "000" and "001" routes. But that's the same as having a single node for "00".
// simplifyCompletedSubtries takes a stack of visited nodes and simplifies completed subtries as far down the stack as
// possible. If at any point in the stack we find a node representing an incomplete subtrie, we stop.
func simplifyVisitedSubtries(visited []*node) {
	for i := len(visited) - 1; i >= 0; i-- {
		if visited[i].isLeaf() {
			return
		}

		if !visited[i].childrenAreCompleteSubtrie() {
			return
		}

		visited[i].children = nil
	}
}

func commonPrefixLen(a, b bitslice.BitSlice) int {
	i := 0
	maxLen := min(len(a), len(b))
	for ; i < maxLen; i++ {
		if a[i] != b[i] {
			break
		}
	}

	return i
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

type traversalStep struct {
	n                  *node
	precedingRouteBits bitslice.BitSlice
}

// Contents returns the BitSlices contained in the RSTrie.
func (t *RSTrie) Contents() []bitslice.BitSlice {
	// If the trie is empty
	if t.root == nil {
		return []bitslice.BitSlice{}
	}

	// Otherwise
	queue := []traversalStep{
		{
			n:                  t.root,
			precedingRouteBits: bitslice.BitSlice{},
		},
	}

	contents := []bitslice.BitSlice{}
	for len(queue) > 0 {
		step := queue[0]
		queue = queue[1:]

		stepRouteBits := bitslice.BitSlice{}
		stepRouteBits = append(stepRouteBits, step.precedingRouteBits...)
		stepRouteBits = append(stepRouteBits, step.n.bits...)

		if step.n.isLeaf() {
			contents = append(contents, stepRouteBits)
		} else {
			queue = append([]traversalStep{
				{
					n:                  step.n.children[0],
					precedingRouteBits: stepRouteBits,
				},
				{
					n:                  step.n.children[1],
					precedingRouteBits: stepRouteBits,
				},
			}, queue...)
		}
	}

	return contents
}