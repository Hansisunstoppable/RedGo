package zset

import (
	"Godis/datastruct/skiplist"
	"fmt"
	"sort"
	"strconv"
)

// ZSet 相关操作接口
type ZSet interface {
	Add(member string, score float64) bool
	Len() int
	Encoding() int
	GetSkiplist() *skiplist.SkipList
	Score(member string) (float64, bool)
	RangeByRank(start, stop int) []string
	Remove(member string) bool
	Count(min, max float64) int
}

// 一开始为 Listpack，后续进化为 Skiplist
const (
	encodingListpack = iota // 0
	encodingSkiplist        // 1
)

// 用于限制 Listpack 的最大长度，超过长度后，使用 Skiplist 来存储
const listpackMaxSize = 128

type zset struct {
	encoding int // 使用 Listpack 还是 skipList 存储
	listpack [][2]string
	dict     map[string]float64 // 用来存储 member 和 score 的映射
	skiplist *skiplist.SkipList
}

// New 创建一个 zset
func NewZSet() ZSet {
	return &zset{
		encoding: encodingListpack,
		listpack: make([][2]string, 0),
	}
}

// Encoding 返回编码方式
func (z *zset) Encoding() int {
	return z.encoding
}

// GetSkiplist 返回跳表实例
func (z *zset) GetSkiplist() *skiplist.SkipList {
	if z.encoding == encodingSkiplist {
		return z.skiplist
	}
	return nil
}

// Len 返回 zset 内元素数量。无需统计跳表长度，直接返回 len(dict) 即可。
func (z *zset) Len() int {
	if z.encoding == encodingListpack {
		return len(z.listpack)
	}
	return len(z.dict)
}

// Score 根据 member 返回 score
func (z *zset) Score(member string) (float64, bool) {
	// listpack 下需要遍历
	if z.encoding == encodingListpack {
		for _, pair := range z.listpack {
			if pair[0] == member {
				score, err := parseScore(pair[1])
				if err != nil {
					return 0, false
				}
				return score, true
			}
		}
		return 0, false
	}
	// dict 下直接返回即可
	score, exists := z.dict[member]
	return score, exists
}

func formatScore(score float64) string {
	return fmt.Sprintf("%f", score)
}

// 从 listpack 编码转换为 skiplist 编码
func (z *zset) convertToSkiplist() {
	if z.encoding == encodingSkiplist {
		return
	}

	// 初始化跳表（skiplist）和字典（dict）
	z.skiplist = skiplist.NewSkipList()
	z.dict = make(map[string]float64, len(z.listpack))

	// 将 listpack 中的所有元素转移到 skiplist 和 dict 中
	for _, pair := range z.listpack {
		member := pair[0]
		score, _ := parseScore(pair[1])
		z.dict[member] = score
		z.skiplist.Insert(member, score)
	}

	// 更新编码类型并清空 listpack
	z.encoding = encodingSkiplist
	z.listpack = nil
}

// 将分数（score）字符串解析为 float64 的辅助函数
func parseScore(scoreStr string) (float64, error) {
	return strconv.ParseFloat(scoreStr, 64)
}

// Add 方法：向有序集合中添加或更新成员
func (z *zset) Add(member string, score float64) bool {
	// 检查是否正在使用 listpack 编码
	if z.encoding == encodingListpack {
		// 检查成员是否已存在于 listpack 中
		for i, pair := range z.listpack {
			if pair[0] == member {
				// 如果成员已存在，则更新其分数
				z.listpack[i][1] = formatScore(score)
				return false // 返回 false 表示成员已存在（未添加新成员）
			}
		}

		// 将新成员添加到 listpack
		z.listpack = append(z.listpack, [2]string{member, formatScore(score)})

		// 如果 listpack 的大小增长过大，则转换为 skiplist
		if len(z.listpack) > listpackMaxSize {
			z.convertToSkiplist()
		}
		return true // 返回 true 表示添加了新成员
	}

	// 正在使用 skiplist/dict 编码
	existingScore, exists := z.dict[member]
	if exists {
		// 如果分数发生变化，则同时更新 dict 和 skiplist
		if existingScore != score {
			// 从 skiplist 中删除旧分数对应的节点
			z.skiplist.Delete(member, existingScore)
			// 以新分数插入到 skiplist
			z.skiplist.Insert(member, score)
			// 更新 dict 中的分数
			z.dict[member] = score
		}
		return false // 返回 false 表示成员已存在（未添加新成员）
	}

	// 将新成员同时添加到 dict 和 skiplist
	z.dict[member] = score
	z.skiplist.Insert(member, score)
	return true // 返回 true 表示添加了新成员
}

// RangeByRank 返回范围内的元素
func (z *zset) RangeByRank(start int, stop int) []string {
	// 从 listpack 中取出 start，stop 范围内的元素
	if z.encoding == encodingListpack {
		// 将 listpack 内的数据复制一份
		pairs := make([][2]string, len(z.listpack))
		copy(pairs, z.listpack)

		// 按照 score 排序
		sort.Slice(pairs, func(i, j int) bool {
			scoreI, _ := parseScore(pairs[i][1])
			scoreJ, _ := parseScore(pairs[j][1])
			return scoreI < scoreJ
		})

		// 处理 index
		size := len(pairs)
		if start < 0 {
			start = size + start
		}
		if stop < 0 {
			stop = size + stop
		}
		if start < 0 {
			start = 0
		}
		if stop >= size {
			stop = size - 1
		}
		if start > stop || start >= size {
			return []string{}
		}

		// 返回结果
		result := make([]string, 0, stop-start+1)
		for i := start; i <= stop; i++ {
			result = append(result, pairs[i][0])
		}
		return result
	}
	return z.skiplist.RangeByRank(start, stop)
}

// Remove 从 zset 中删除成员
func (z *zset) Remove(member string) bool {
	// 从 listpack 中删除
	if z.encoding == encodingListpack {
		for i, pair := range z.listpack {
			if pair[0] == member {
				// 移除该元素
				z.listpack = append(z.listpack[:i], z.listpack[i+1:]...)
				return true
			}
		}
		return false
	}

	// 从跳表中删除
	score, exists := z.dict[member]
	if exists {
		z.skiplist.Delete(member, score)
		delete(z.dict, member)
		return true
	}
	return false
}

// Count 返回指定范围内的元素数量
func (z *zset) Count(min, max float64) int {
	if z.encoding == encodingListpack {
		count := 0
		for _, pair := range z.listpack {
			score, _ := parseScore(pair[1])
			if score >= min && score <= max {
				count++
			}
		}
		return count
	}
	return z.skiplist.CountInRange(min, max)
}
