package dict

import (
	"Godis/lib/wildcard"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentDict 支持并发访问的底层数据结构，不仅仅可以对 key 并发访问，还能够对 key 对应的数据结构并发访问
type ConcurrentDict struct {
	table      []*shard
	count      int32
	shardCount int
}

// shard 一个分片，可能对应多组 kv！因为不同的 key 可能 hash 到同一个分片。
type shard struct {
	m     map[string]interface{} // 存储该分片对应的多组 kv
	mutex sync.RWMutex           //分片锁，读写锁
}

// computeCapacity 计算参数 param 的最接近的 2 的幂，作为分片锁容量
func computeCapacity(param int) (size int) {
	if param <= 16 {
		return 16
	}
	n := param - 1
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	if n < 0 {
		return math.MaxInt32
	}
	return n + 1
}

// MakeConcurrent 根据给定的分片锁数量，创建 ConcurrentDict
func MakeConcurrentDict(shardCount int) *ConcurrentDict {
	if shardCount == 1 {
		table := []*shard{
			{
				m: make(map[string]interface{}),
			},
		}
		return &ConcurrentDict{
			count:      0,
			table:      table,
			shardCount: shardCount,
		}
	}
	shardCount = computeCapacity(shardCount) // 找到最接近的 2 的幂
	table := make([]*shard, shardCount)
	for i := 0; i < shardCount; i++ {
		table[i] = &shard{
			m: make(map[string]interface{}),
		}
	}
	d := &ConcurrentDict{
		count:      0,
		table:      table,
		shardCount: shardCount,
	}
	return d
}

// prime32 FNV 哈希算法使用的质数 16777619
const prime32 = uint32(16777619)

// fnv32 fnv 哈希算法
func fnv32(key string) uint32 {
	hash := uint32(2166136261)      // 初始化哈希值为 2166136261
	for i := 0; i < len(key); i++ { // 遍历字符串每个字节
		hash *= prime32        // 先乘以质数
		hash ^= uint32(key[i]) //再与当前字节异或
	}
	return hash
}

// spread 根据 key 计算分片的 index
func (dict *ConcurrentDict) spread(key string) uint32 {
	if dict == nil {
		panic("dict is nil")
	}
	if len(dict.table) == 1 {
		return 0
	}
	hashCode := fnv32(key)
	tableSize := uint32(len(dict.table))
	return (tableSize - 1) & hashCode
}

// getShard 根据 index 获取 shard，包含 kv 与 锁
func (dict *ConcurrentDict) getShard(index uint32) *shard {
	if dict == nil {
		panic("dict is nil")
	}
	return dict.table[index]
}

// Get 根据 key 获取 value, 自动加锁，业务逻辑代码无需管理锁
func (dict *ConcurrentDict) Get(key string) (val interface{}, exists bool) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	val, exists = s.m[key]
	return
}

// GetWithLock 根据 key 获取 value, 不自动加锁，获取锁给调用者进行管理
func (dict *ConcurrentDict) GetWithLock(key string) (val interface{}, exists bool) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	val, exists = s.m[key]
	return
}

// Len 返回底层数据结构的长度
func (dict *ConcurrentDict) Len() int {
	if dict == nil {
		panic("dict is nil")
	}
	return int(atomic.LoadInt32(&dict.count))
}

// Put 将 kv 放入，自动管理锁
func (dict *ConcurrentDict) Put(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.m[key]; ok {
		s.m[key] = val
		return 0
	}
	dict.addCount()
	s.m[key] = val
	return 1
}

func (dict *ConcurrentDict) PutWithLock(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)

	if _, ok := s.m[key]; ok {
		s.m[key] = val
		return 0
	}
	dict.addCount()
	s.m[key] = val
	return 1
}

// PutIfAbsent 如果不存在，则 put
func (dict *ConcurrentDict) PutIfAbsent(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.m[key]; ok {
		return 0
	}
	s.m[key] = val
	dict.addCount()
	return 1
}

func (dict *ConcurrentDict) PutIfAbsentWithLock(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)

	if _, ok := s.m[key]; ok {
		return 0
	}
	s.m[key] = val
	dict.addCount()
	return 1
}

// PutIfExists 如果存在，才 put
func (dict *ConcurrentDict) PutIfExists(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.m[key]; ok {
		s.m[key] = val
		return 1
	}
	return 0
}

func (dict *ConcurrentDict) PutIfExistsWithLock(key string, val interface{}) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)

	if _, ok := s.m[key]; ok {
		s.m[key] = val
		return 1
	}
	return 0
}

// Remove 根据 key，移除底层 kv
func (dict *ConcurrentDict) Remove(key string) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.m[key]; ok {
		delete(s.m, key)
		dict.decreaseCount()
		return 1
	}
	return 0
}

func (dict *ConcurrentDict) RemoveWithLock(key string) (result int) {
	if dict == nil {
		panic("dict is nil")
	}
	index := dict.spread(key)
	s := dict.getShard(index)

	if _, ok := s.m[key]; ok {
		delete(s.m, key)
		dict.decreaseCount()
		return 1
	}
	return 0
}
func (dict *ConcurrentDict) addCount() int32 {
	return atomic.AddInt32(&dict.count, 1)
}

func (dict *ConcurrentDict) decreaseCount() int32 {
	return atomic.AddInt32(&dict.count, -1)
}

// ForEach 遍历字典，不会访问在遍历期间新插入的条目
func (dict *ConcurrentDict) ForEach(consumer Consumer) {
	if dict == nil {
		panic("dict is nil")
	}

	for _, s := range dict.table {
		s.mutex.RLock() // 遍历上读锁
		f := func() bool {
			defer s.mutex.RUnlock()
			for key, value := range s.m {
				continues := consumer(key, value)
				if !continues {
					return false
				}
			}
			return true
		}
		if !f() {
			break
		}
	}
}

// Keys 获取所有 key
func (dict *ConcurrentDict) Keys() []string {
	keys := make([]string, dict.Len())
	i := 0
	dict.ForEach(func(key string, val interface{}) bool {
		if i < len(keys) {
			keys[i] = key
			i++
		} else {
			keys = append(keys, key)
		}
		return true
	})
	return keys
}

// RandomKey 随即返回一个 key
func (shard *shard) RandomKey() string {
	if shard == nil {
		panic("shard is nil")
	}
	shard.mutex.RLock()
	defer shard.mutex.RUnlock()

	for key := range shard.m {
		return key
	}
	return ""
}

// RandomKeys 随机返回 llimit 个 key，可能包含重复
func (dict *ConcurrentDict) RandomKeys(limit int) []string {
	size := dict.Len()
	if limit >= size {
		return dict.Keys()
	}
	shardCount := len(dict.table)

	result := make([]string, limit)
	nR := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < limit; {
		s := dict.getShard(uint32(nR.Intn(shardCount))) // 随机获取一个分片
		if s == nil {
			continue
		}
		key := s.RandomKey() // 从一个分片从获取随机的 key
		if key != "" {
			result[i] = key
			i++
		}
	}
	return result
}

// RandomDistinctKeys 随机返回 limit 个 key，不包含重复
func (dict *ConcurrentDict) RandomDistinctKeys(limit int) []string {
	size := dict.Len()
	if limit >= size {
		return dict.Keys()
	}

	shardCount := len(dict.table)
	result := make(map[string]struct{}) // 用 map 来去重
	nR := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(result) < limit {
		shardIndex := uint32(nR.Intn(shardCount))
		s := dict.getShard(shardIndex)
		if s == nil {
			continue
		}
		key := s.RandomKey()
		if key != "" {
			if _, exists := result[key]; !exists {
				result[key] = struct{}{}
			}
		}
	}
	arr := make([]string, limit)
	i := 0
	for k := range result {
		arr[i] = k
		i++
	}
	return arr
}

func (dict *ConcurrentDict) Clear() {
	*dict = *MakeConcurrentDict(dict.shardCount)
}

// toLockIndices 获取一组 key 对应的 分片 index，按照顺序或者逆序排列
func (dict *ConcurrentDict) toLockIndices(keys []string, reverse bool) []uint32 {
	indexMap := make(map[uint32]struct{})
	for _, key := range keys {
		index := dict.spread(key)
		indexMap[index] = struct{}{}
	}
	indices := make([]uint32, 0, len(indexMap))
	for index := range indexMap {
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool {
		if !reverse {
			return indices[i] < indices[j]
		}
		return indices[i] > indices[j]
	})
	return indices
}

// RWLocks 根据一组 key，对其分片加 写锁 或者 读锁
func (dict *ConcurrentDict) RWLocks(writeKeys []string, readKeys []string) {
	keys := append(writeKeys, readKeys...)
	indices := dict.toLockIndices(keys, false) // 获取这一组 key 对应的分片，升序排列，打破循环等待，不会死锁！
	writeIndexSet := make(map[uint32]struct{})

	for _, wKey := range writeKeys { // 获取写锁的分片
		idx := dict.spread(wKey)
		writeIndexSet[idx] = struct{}{}
	}
	for _, index := range indices {
		_, w := writeIndexSet[index] // 判断是否是写锁
		mu := &dict.table[index].mutex
		if w { // 加写锁
			mu.Lock()
		} else { // 加读锁
			mu.RLock()
		}
	}
}

// RWUnLocks  根据一组 key，对其分片解 写锁 或者 读锁
func (dict *ConcurrentDict) RWUnLocks(writeKeys []string, readKeys []string) {
	keys := append(writeKeys, readKeys...)
	indices := dict.toLockIndices(keys, true)
	writeIndexSet := make(map[uint32]struct{})
	for _, wKey := range writeKeys {
		idx := dict.spread(wKey)
		writeIndexSet[idx] = struct{}{}
	}
	for _, index := range indices {
		_, w := writeIndexSet[index]
		mu := &dict.table[index].mutex
		if w {
			mu.Unlock()
		} else {
			mu.RUnlock()
		}
	}
}

func stringsToBytes(strSlice []string) [][]byte {
	byteSlice := make([][]byte, len(strSlice))
	for i, str := range strSlice {
		byteSlice[i] = []byte(str)
	}
	return byteSlice
}

// DictScan 按照给定模式，匹配所有 key
func (dict *ConcurrentDict) DictScan(cursor int, count int, pattern string) ([][]byte, int) {
	size := dict.Len()
	result := make([][]byte, 0)

	if pattern == "*" && count >= size {
		return stringsToBytes(dict.Keys()), 0
	}

	matchKey := wildcard.CompilePattern(pattern)

	shardCount := len(dict.table)
	shardIndex := cursor

	for shardIndex < shardCount {
		shard := dict.table[shardIndex]
		shard.mutex.RLock()
		if len(result)+len(shard.m) > count && shardIndex > cursor {
			shard.mutex.RUnlock()
			return result, shardIndex
		}

		for key := range shard.m {
			if pattern == "*" || matchKey.IsMatch(key) {
				result = append(result, []byte(key))
			}
		}
		shard.mutex.RUnlock()
		shardIndex++
	}

	return result, 0
}
