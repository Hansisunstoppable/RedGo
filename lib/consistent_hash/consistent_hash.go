package consistenthash

import (
	"hash/crc32"
	"sort"
)

type NodeMap struct {
	hashFunc func(data []byte) uint32
	// TODO 可优化为有序表（跳表），实现 O(logN) 的插入。当前有序顺序表插入为 O(n)
	nodeHashs   []int          // 切片来存储所有节点的哈希值, 排序后方便二分查找
	nodehashMap map[int]string // 结点哈希值与结点名的的映射，便于通过哈希值查找节点。
}

func NewNodeMap(hashFunc func(data []byte) uint32) *NodeMap {
	m := &NodeMap{
		hashFunc:    hashFunc,
		nodeHashs:   make([]int, 0),
		nodehashMap: make(map[int]string),
	}
	// 设置默认的哈希函数为 crc32
	if m.hashFunc == nil {
		m.hashFunc = crc32.ChecksumIEEE
	}
	return m
}

// 判断 NodeMap 是否为空，即集群上是否有结点
func (m *NodeMap) IsEmpty() bool {
	return len(m.nodeHashs) == 0
}

// 添加结点, 传入字符串切片，包含多个结点的名称
func (m *NodeMap) AddNodes(nodes ...string) {
	for _, node := range nodes {
		if node == "" {
			continue
		}
		hashKey := int(m.hashFunc([]byte(node)))
		m.nodeHashs = append(m.nodeHashs, hashKey)
		m.nodehashMap[hashKey] = node
	}
	// 切片与跳表的核心复杂度区别
	// 跳表可以实现 logN 插入，切片 append 后排序，时间复杂度为 NlogN
	sort.Ints(m.nodeHashs)
}

// 找到数据 key 对应的结点， 即大于 hash(key) 最近的结点
func (m *NodeMap) PickNode(key string) string {
	if m.IsEmpty() {
		return ""
	}
	hashKey := int(m.hashFunc([]byte(key)))
	// 查找到第一个大于 hash(key) 的结点，作为该数据归属的结点
	index := sort.Search(len(m.nodeHashs), func(i int) bool {
		return m.nodeHashs[i] >= hashKey
	})
	// 找不到大于 hash(key) 的结点，则返回第一个结点
	if index == len(m.nodeHashs) {
		index = 0
	}
	return m.nodehashMap[m.nodeHashs[index]]
}
