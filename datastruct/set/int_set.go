package set

import (
	"encoding/binary"
	"fmt"
	"math"
)

// 动态编码的整数集合
// 根据整数集合的大小，我们可以使用不同的编码类型来节省空间。
const (
	INTSET_ENC_INT16 = 2 // 使用 16 位整数编码
	INTSET_ENC_INT32 = 4 // 使用 32 位整数编码
	INTSET_ENC_INT64 = 8 // 使用 64 位整数编码
)

// IntSet 是一个整数集合结构体
type IntSet struct {
	encoding uint32 // 当前使用的编码方式
	length   uint32 // 集合中整数的个数
	contents []byte // 存储整数的字节数组
}

// Remove 删除一个整数
func (is *IntSet) Remove(val int64) bool {
	pos := is.findPosition(val)
	if pos < 0 {
		return false
	}
	is.removeAt(pos)
	return true
}

// removeAt 删除指定位置的元素
func (is *IntSet) removeAt(pos int) {
	if pos < 0 || pos >= int(is.length) {
		return
	}
	offset := pos * int(is.encoding)
	endOffset := is.Len() * int(is.encoding)

	// 把待删除的部分覆盖掉
	copy(is.contents[offset:], is.contents[(offset+int(is.encoding)):endOffset])

	// 重新截取切片，把末尾的删除掉
	is.contents = is.contents[:endOffset-int(is.encoding)]
	is.length--

}

// Contains 检查 IntSet 中是否存在指定的值
func (is *IntSet) Contains(val int64) bool {
	pos := is.findPosition(val)
	return pos >= 0
}

// ForEach 遍历并处理 IntSet 中的所有元素
func (is *IntSet) ForEach(consumer func(val int64) bool) {
	for i := uint32(0); i < is.length; i++ {
		if !consumer(is.getValueAt(i)) {
			break
		}
	}
}

// NewIntSet 创建一个新的 IntSet，使用默认的 16 位整数编码
func NewIntSet() *IntSet {
	return &IntSet{
		encoding: INTSET_ENC_INT16, // 默认编码为 16 位整数
		length:   0,                // 初始长度为 0
		contents: make([]byte, 0),  // 初始内容为空字节切片
	}
}

func (is *IntSet) Len() int {
	return int(is.length)
}

// Add 添加一个整数到 IntSet
func (is *IntSet) Add(value int64) bool {
	// 判断要添加的元素是否超出当前编码范围
	var requiredEncoding uint32
	if value < math.MinInt16 || value > math.MaxInt16 { // 超出 16 位编码范围
		if value < math.MinInt32 || value > math.MaxInt32 { // 超出 32 位编码范围
			requiredEncoding = INTSET_ENC_INT64 // 使用 64 位编码
		} else {
			requiredEncoding = INTSET_ENC_INT32 // 使用 32 位编码
		}
	} else {
		requiredEncoding = INTSET_ENC_INT16 // 使用 16 位编码
	}

	// 若需要更长的编码，则进行编码升级
	if requiredEncoding > is.encoding {
		is.upgradeEncoding(requiredEncoding)
	}

	// 在 IntSet 内查找该元素
	pos := is.findPosition(value)
	if pos >= 0 {
		return false // 元素早已存在
	}

	// 插入元素
	pos = -(pos + 1) // 二分查找失败时返回负数，需要重新解析为正数插入位置
	is.insertAt(pos, value)
	return true
}

// upgradeEncoding 升级编码
func (is *IntSet) upgradeEncoding(newEncoding uint32) {
	if newEncoding <= is.encoding {
		return
	}

	oldValues := is.ToSlice() // 获取所有元素
	is.encoding = newEncoding
	is.length = 0
	is.contents = make([]byte, int(newEncoding)*len(oldValues)) // 新的字节切片容量

	// 将所有元素重新添加到集合中，会按照新的编码方式插入
	for _, v := range oldValues {
		is.Add(v)
	}
}

// ToSlice 将当前集合中的所有元素保存到一个切片中
func (is *IntSet) ToSlice() []int64 {
	result := make([]int64, is.length)
	for i := uint32(0); i < is.length; i++ {
		result[i] = is.getValueAt(i)
	}
	return result
}

// getValueAt 获取指定索引处的值
func (is *IntSet) getValueAt(index uint32) int64 {
	if index >= is.length {
		panic(fmt.Sprintf("Index out of bounds: %d", index))
	}
	offset := index * is.encoding // 每一个元素占用 is.encoding 个字节
	// 根据编码方式解码
	switch is.encoding {
	case INTSET_ENC_INT16:
		return int64(int16(binary.LittleEndian.Uint16(is.contents[offset:])))
	case INTSET_ENC_INT32:
		return int64(int32(binary.LittleEndian.Uint32(is.contents[offset:])))
	case INTSET_ENC_INT64:
		return int64(binary.LittleEndian.Uint64(is.contents[offset:]))
	}
	panic("Invalid encoding")
}

// insertAt 插入元素到指定位置
func (is *IntSet) insertAt(pos int, value int64) {
	// 新的字节切片长度 = 新的字节切片长度 + 一个元素的长度
	oldLen := len(is.contents)
	newLen := oldLen + int(is.encoding)

	if cap(is.contents) < newLen { // 如果切片容量不足，则扩容
		newContents := make([]byte, newLen, newLen*2) // 直接将容量乘二
		copy(newContents, is.contents)
		is.contents = newContents
	} else {
		is.contents = is.contents[:newLen] // 更新切片长度
	}

	// 获取实际要插入的位置 offset
	offset := pos * int(is.encoding)
	if pos < int(is.length) {
		// 顺序表的插入，需要将后面的所有元素后移 is.encoding 个字节
		copy(is.contents[offset+int(is.encoding):], is.contents[offset:oldLen])
	}

	// 插入新元素
	switch is.encoding {
	case INTSET_ENC_INT16:
		binary.LittleEndian.PutUint16(is.contents[offset:], uint16(value))
	case INTSET_ENC_INT32:
		binary.LittleEndian.PutUint32(is.contents[offset:], uint32(value))
	case INTSET_ENC_INT64:
		binary.LittleEndian.PutUint64(is.contents[offset:], uint64(value))
	}

	is.length++
}

// findPosition 在集合中找到指定值的 index。若不存在，返回负数。
func (is *IntSet) findPosition(value int64) int {
	// 二分查找
	low, high := 0, int(is.length)-1
	for low <= high {

		mid := (low + high) / 2
		midValue := is.getValueAt(uint32(mid))
		if midValue < value {
			low = mid + 1
		} else if midValue > value {
			high = mid - 1
		} else {
			return mid // Found
		}
	}
	return -(low + 1) // 没有查找到，返回第一个大于 value 的元素下标，或者 length（即尾后）位置
}
