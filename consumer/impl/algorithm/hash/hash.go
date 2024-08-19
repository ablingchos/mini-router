package consistenthash

import (
	"crypto/sha1"
	"sort"
	"strconv"
	"sync"
)

// HashRing 结构体
type HashRing struct {
	nodes       map[uint32]string
	sortedKeys  []uint32
	virtualNode int
	sync.RWMutex
}

// NewHashRing 创建一个新的 HashRing
func NewHashRing(virtualNode int) *HashRing {
	return &HashRing{
		nodes:       make(map[uint32]string),
		virtualNode: virtualNode,
	}
}

// AddNode 添加节点
func (h *HashRing) AddNode(node string) {
	h.Lock()
	defer h.Unlock()

	for i := 0; i < h.virtualNode; i++ {
		virtualNodeKey := h.hashKey(node + strconv.Itoa(i))
		h.nodes[virtualNodeKey] = node
		h.sortedKeys = append(h.sortedKeys, virtualNodeKey)
	}
	sort.Slice(h.sortedKeys, func(i, j int) bool {
		return h.sortedKeys[i] < h.sortedKeys[j]
	})
}

// RemoveNode 移除节点
func (h *HashRing) RemoveNode(node string) {
	h.Lock()
	defer h.Unlock()

	for i := 0; i < h.virtualNode; i++ {
		virtualNodeKey := h.hashKey(node + strconv.Itoa(i))
		delete(h.nodes, virtualNodeKey)
		h.removeKey(virtualNodeKey)
	}
}

// GetNode 获取数据对应的节点
func (h *HashRing) GetNode(key string) string {
	h.RLock()
	defer h.RUnlock()

	if len(h.nodes) == 0 {
		return ""
	}

	hash := h.hashKey(key)
	idx := h.searchKey(hash)
	return h.nodes[h.sortedKeys[idx]]
}

// hashKey 计算哈希值
func (h *HashRing) hashKey(key string) uint32 {
	hash := sha1.New()
	hash.Write([]byte(key))
	return uint32(hash.Sum(nil)[0])<<24 | uint32(hash.Sum(nil)[1])<<16 | uint32(hash.Sum(nil)[2])<<8 | uint32(hash.Sum(nil)[3])
}

// searchKey 二分查找
func (h *HashRing) searchKey(hash uint32) int {
	idx := sort.Search(len(h.sortedKeys), func(i int) bool {
		return h.sortedKeys[i] >= hash
	})
	if idx == len(h.sortedKeys) {
		return 0
	}
	return idx
}

// removeKey 移除哈希环中的键
func (h *HashRing) removeKey(key uint32) {
	idx := sort.Search(len(h.sortedKeys), func(i int) bool {
		return h.sortedKeys[i] >= key
	})
	if idx < len(h.sortedKeys) && h.sortedKeys[idx] == key {
		h.sortedKeys = append(h.sortedKeys[:idx], h.sortedKeys[idx+1:]...)
	}
}
