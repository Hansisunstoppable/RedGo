package set

import (
	"math/rand"
	"strconv"
	"time"
)

// 无序集合数据结构，保证元素唯一性

// intset 的最大元素数量，当元素数量超过这个值时，转换为 hashTable
const (
	SET_MAX_INTSET_ENTRIES = 512
)

type HashSet struct {
	dict     map[string]struct{}
	intset   *IntSet
	isIntset bool
}

// NewHashSet 创建一个 HashSet
func NewHashSet() *HashSet {
	return &HashSet{
		dict:     make(map[string]struct{}),
		intset:   NewIntSet(),
		isIntset: true, // 开始时，默认采用 Intset
	}
}

// Add 向集合添加元素
func (set *HashSet) Add(member string) int {
	if set.isIntset {
		if val, err := strconv.ParseInt(member, 10, 64); err == nil {
			if ok := set.intset.Add(val); ok {
				if set.intset.Len() > SET_MAX_INTSET_ENTRIES { // intSet 元素数量超过阈值，转换为 hashTable
					set.convertToHashTable()
				}
				return 1
			}
			return 0
		} else { // err != nil 代表 member 不是整数
			// 要添加的元素不是整数，将 intSet 转换为 hashTable
			set.convertToHashTable()
		}
	}

	if _, exists := set.dict[member]; exists {
		return 0 // 集合内已存在
	}
	set.dict[member] = struct{}{}
	return 1 // 成功加入集合
}

// Contains implements Set.
func (set *HashSet) Contains(member string) bool {
	if set.isIntset { // IntSet 找元素，需要遍历
		if val, err := strconv.ParseInt(member, 10, 64); err == nil {
			return set.intset.Contains(val)
		}
		return false //
	}
	_, exist := set.dict[member] // hashTable 找元素, 直接获取即可
	return exist

}

// ForEach
func (set *HashSet) ForEach(consumer func(member string) bool) {
	if set.isIntset {
		set.intset.ForEach(func(val int64) bool {
			return consumer(strconv.FormatInt(val, 10))
		})
	} else {
		for member := range set.dict {
			if !consumer(member) {
				break
			}
		}
	}
}

// Len
func (set *HashSet) Len() int {
	if set.isIntset {
		return set.intset.Len()
	}
	return len(set.dict)
}

// Members 返回集合中所有元素
func (set *HashSet) Members() []string {
	if set.isIntset {
		members := make([]string, 0, set.intset.Len())
		set.intset.ForEach(func(value int64) bool {
			members = append(members, strconv.FormatInt(value, 10))
			return true
		})
		return members
	}
	// 哈希
	members := make([]string, 0, len(set.dict))
	for member := range set.dict {
		members = append(members, member)
	}

	return members
}

// RandomDistinctMembers 随机获取count 个不同数字的集合
func (set *HashSet) RandomDistinctMembers(count int) []string {
	size := set.Len()
	if count <= 0 || size == 0 {
		return []string{}
	}

	if count >= size {
		return set.Members()
	}

	members := set.Members()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(members), func(i, j int) { // len(members) 为打乱的范围
		members[i], members[j] = members[j], members[i]
	})

	return members[:count] // 返回前 count 个元素

}

// RandomMembers  随机获取 count 个数字，可能重复
func (set *HashSet) RandomMembers(count int) []string {
	size := set.Len()
	if count <= 0 || size == 0 {
		return []string{}
	}

	res := make([]string, count)
	members := set.Members()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < count; i++ {
		res[i] = members[r.Intn(size)] // 使用 r.Intn(size) 获取随机索引，可能获取到同一个索引，导致重复
	}
	return res
}

// Remove 移除集合内的元素
func (set *HashSet) Remove(member string) int {
	if set.isIntset {
		if val, err := strconv.ParseInt(member, 10, 64); err == nil {
			if ok := set.intset.Remove(val); ok {
				return 1
			}
			return 0
		}
	}

	if _, exist := set.dict[member]; exist {
		delete(set.dict, member)
		return 1
	}
	return 0
}

// convertToHashTable 将 IntSet 转换为哈希表
func (set *HashSet) convertToHashTable() {
	if !set.isIntset { // 已经是 哈希表
		return
	}

	// 将 IntSet 中的所有元素插入 hashTable
	set.intset.ForEach(func(value int64) bool {
		set.dict[strconv.FormatInt(value, 10)] = struct{}{}
		return true
	})
	set.isIntset = false
}

// IsIntSet 检查集合是否为 IntSet
func (set *HashSet) IsIntSet() bool {
	return set.isIntset
}
