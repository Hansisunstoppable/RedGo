package skiplist

import (
	"math/rand"
	"time"
)

const maxLevel = 16 // 跳表最大层数

// Node 跳表上的结点
type Node struct {
	Member  string
	Score   float64 // 按 Score 排序
	Forward []*Node // 对应该结点不同层的下一个结点
}

// SkipList represents a skip list
type SkipList struct {
	header *Node      // 头节点
	tail   *Node      // 尾节点
	level  int        // 当前跳表层数
	length int        // 跳表元素数量
	rand   *rand.Rand // 根据概率分布，随机产生当前结点插入的层数
}

// New SkipList creates a new skip list
func NewSkipList() *SkipList {
	header := &Node{
		Forward: make([]*Node, maxLevel), // 初始化头节点的 Forward 切片
	}
	return &SkipList{
		header: header,
		level:  1,                                               // 初始层级为 1
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())), // 初始化随机数生成器
	}
}

// randomLevel 随机生成当前插入的层号
func (sl *SkipList) randomLevel() int {
	level := 1 // 层数至少为 1
	// 以 25% 的概率增加层级，直到达到 maxLevel 或概率判定失败
	for level < maxLevel && sl.rand.Float32() < 0.25 {
		level++
	}
	return level
}

// Insert 插入结点到跳表
func (sl *SkipList) Insert(member string, score float64) {
	update := make([]*Node, maxLevel) // 存储每一层需要更新的前驱节点
	x := sl.header

	// 1. 查找插入位置：从最高层向下查找
	for i := sl.level - 1; i >= 0; i-- {
		// 在当前层向右查找，直到找到第一个大于等于待插入元素 score 的节点
		for x.Forward[i] != nil &&
			(x.Forward[i].Score < score || // 按照 score 排序
				(x.Forward[i].Score == score && x.Forward[i].Member < member)) { // score 相同时，按照 member 字典序排列
			x = x.Forward[i]
		}
		// 记录下这一层需要修改 Forward 指针的节点 (即新节点的前驱)s
		update[i] = x
	}
	// 此时 x 是最底层中小于新节点的最后一个节点

	// 2. 生成新节点的随机层级
	level := sl.randomLevel()

	// 3. 更新 SkipList 的最大层级 (如果需要)
	if level > sl.level {
		// 如果新节点的层级超过当前最大层级，需要扩展 update 数组
		for i := sl.level; i < level; i++ { // 可能一次性增长多层
			update[i] = sl.header // 新增层级的前驱节点是 header（因为这个新的层级为空）
		}
		sl.level = level // 更新 SkipList 的当前最大层级
	}

	// 4. 创建新节点
	newNode := &Node{
		Member:  member,
		Score:   score,
		Forward: make([]*Node, level), // Forward 切片大小为新节点的层级
	}

	// 5. 更新指针，将新节点链入 SkipList，只需要插入前 level 层
	for i := 0; i < level; i++ {
		newNode.Forward[i] = update[i].Forward[i] // 新节点的 Forward 指向原前驱节点的下一个节点
		update[i].Forward[i] = newNode            // 前驱节点的 Forward 指向新节点
	}

	// 6. 更新尾节点指针 (如果新节点是最后一个节点)
	if newNode.Forward[0] == nil {
		sl.tail = newNode
	}

	sl.length++ // 更新 SkipList 的长度
}

// Delete 删除跳表中的元素
func (sl *SkipList) Delete(member string, score float64) bool {
	update := make([]*Node, maxLevel)
	x := sl.header

	// 1. 查找目标节点的前驱节点
	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil &&
			(x.Forward[i].Score < score ||
				(x.Forward[i].Score == score && x.Forward[i].Member < member)) {
			x = x.Forward[i]
		}
		update[i] = x // 记录每层的前驱
	}

	// 2. 定位目标节点
	// x 现在是最底层目标节点的前驱，x.Forward[0] 可能是目标节点
	targetNode := x.Forward[0]

	// 3. 检查节点是否存在且匹配
	if targetNode != nil && targetNode.Score == score && targetNode.Member == member {
		// 4. 更新指针，在所有层级中移除目标节点
		for i := 0; i < sl.level; i++ {
			// 如果 update[i] 的下一个节点不是目标节点，说明目标节点不在这一层或更高层
			if update[i].Forward[i] != targetNode {
				break // 可以提前结束
			}
			// 将前驱节点的 Forward 指向目标节点的下一个节点，完成移除
			update[i].Forward[i] = targetNode.Forward[i]
		}

		// 5. 更新尾节点指针 (如果删除的是尾节点)
		if targetNode == sl.tail {
			// 新的尾节点是 update[0] (最底层的前驱)
			// 如果 update[0] 是 header，说明列表空了，tail 应为 nil
			if update[0] == sl.header {
				sl.tail = nil
			} else {
				sl.tail = update[0]
			}
		}

		// 6. 更新 SkipList 的最大层级 (如果需要)
		// 如果删除节点后，最高层变为空，则降低 SkipList 的 level
		for sl.level > 1 && sl.header.Forward[sl.level-1] == nil {
			sl.level--
		}

		sl.length-- // 更新长度
		return true // 删除成功
	}

	return false // 未找到或不匹配，删除失败
}

// CountInRange 统计范围间的元素个数
func (sl *SkipList) CountInRange(min, max float64) int {
	count := 0
	x := sl.header

	// 从高层开始遍历，更快
	for i := sl.level - 1; i >= 0; i-- {
		// 遍历当前层，找到第一个大于等于 min 的结点
		for x.Forward[i] != nil && x.Forward[i].Score < min {
			x = x.Forward[i]
		}
	}

	// 遍历所有 min 到 max 的元素，并统计
	x = x.Forward[0]
	for x != nil && x.Score <= max {
		count++
		x = x.Forward[0]
	}

	return count
}

// RangeByScore 统计范围间的元素，返回元素名称切片
func (sl *SkipList) RangeByScore(min, max float64, offset, count int) []string {
	result := []string{}
	x := sl.header

	// 从高层开始遍历，更快
	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && x.Forward[i].Score < min {
			x = x.Forward[i]
		}
	}

	// 遍历所有 min 到 max 的元素，并收集所有元素名
	x = x.Forward[0]
	skipped := 0

	for x != nil && x.Score <= max {
		if offset < 0 || skipped >= offset {
			result = append(result, x.Member)
			// Stop if we've collected enough elements
			if count > 0 && len(result) >= count {
				break
			}
		} else {
			skipped++
		}
		x = x.Forward[0]
	}

	return result
}

// RangeByRank 从 start 位置开始，返回接下来的 stop - start + 1 个成员
func (sl *SkipList) RangeByRank(start, stop int) []string {
	result := []string{}

	// 处理负数索引
	if start < 0 {
		start = sl.length + start
	}
	if stop < 0 {
		stop = sl.length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= sl.length {
		stop = sl.length - 1
	}
	if start > stop || start >= sl.length {
		return result
	}

	// 在第 0 层遍历，遍历到第 start 个元素
	x := sl.header.Forward[0]
	for i := 0; i < start && x != nil; i++ {
		x = x.Forward[0]
	}

	// 收集元素
	for i := start; i <= stop && x != nil; i++ {
		result = append(result, x.Member)
		x = x.Forward[0]
	}

	return result
}

// GetRank 返回元素的排名
func (sl *SkipList) GetRank(member string, score float64) int {
	rank := 0
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil &&
			(x.Forward[i].Score < score ||
				(x.Forward[i].Score == score && x.Forward[i].Member < member)) {
			rank += 1 // 累加跳过的结点
			x = x.Forward[i]
		}
	}

	// 因为上面是找严格大于 score 的，所以要走到下一格。
	x = x.Forward[0]
	if x != nil && x.Member == member {
		return rank
	}

	return -1 /// 未找到
}
