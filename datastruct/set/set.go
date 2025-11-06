package set

// Set 相关操作接口
type Set interface {
	Add(member string) int                     // 向集合添加一个成员，成功添加返回 1，已存在返回 0
	Remove(member string) int                  // 从集合移除一个成员，成功移除返回 1，不存在返回 0
	Contains(member string) bool               // 判断集合是否包含指定成员
	Members() []string                         // 获取集合中所有成员
	Len() int                                  // 获取集合成员数量
	ForEach(consumer func(member string) bool) // 遍历所有成员，consumer 返回 false 时停止遍历
	RandomMembers(count int) []string          // 随机获取指定数量的成员（可能重复）
	RandomDistinctMembers(count int) []string  // 随机获取指定数量的不重复成员
	IsIntSet() bool                            // 判断集合是否为 IntSet
}
