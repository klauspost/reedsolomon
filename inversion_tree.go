package reedsolomon

import (
	"sync"
)

type inversionNode struct {
	mutex    sync.RWMutex
	matrix   matrix
	children []*inversionNode
}

func (n inversionNode) GetInvertedMatrix(invalidIndices []int) matrix {
	return n.getInvertedMatrix(invalidIndices, 0)
}

func (n inversionNode) InsertInvertedMatrix(invalidIndices []int, matrix matrix, shards int) {
	n.insertInvertedMatrix(invalidIndices, matrix, shards, 0)
}

func (n inversionNode) getInvertedMatrix(invalidIndices []int, parent int) matrix {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	node := n.children[invalidIndices[0]-parent]
	if node == nil {
		return nil
	}

	if len(invalidIndices) > 1 {
		return node.getInvertedMatrix(invalidIndices[1:], invalidIndices[0]+1)
	}
	return n.matrix
}

func (n inversionNode) insertInvertedMatrix(invalidIndices []int, matrix matrix, shards, parent int) {
	node := n.children[invalidIndices[0]-parent]
	if node == nil {
		n.mutex.Lock()
		defer n.mutex.Unlock()
		node = &inversionNode{
			mutex:    sync.RWMutex{},
			children: make([]*inversionNode, shards-invalidIndices[0]),
		}
		n.children[invalidIndices[0]-parent] = node
	}

	if len(invalidIndices) > 1 {
		node.insertInvertedMatrix(invalidIndices[1:], matrix, invalidIndices[0]+1, shards)
	} else {
		node.mutex.Lock()
		defer node.mutex.Unlock()
		node.matrix = matrix
	}
}
