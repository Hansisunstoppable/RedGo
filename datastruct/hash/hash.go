package hash

const (
	// 当哈希中的条目数超过该值时，将转换为 hashTable
	hashMaxListpackEntries = 512
	// 当哈希中值的长度超过该值时，将转换为 hashTable
	hashMaxListpackValue = 64
)

// 哈希的编码类型
const (
	encodingListpack  = iota // Listpack 编码（适用于小哈希）
	encodingHashTable        // 哈希表编码（适用于大哈希）
)

type Hash struct {
	encoding int               // 哈希的编码类型
	listpack [][2]string       // 使用 Go 切片模拟 Listpack 结构
	dict     map[string]string // 标准哈希表存储
}

// MakeHash 创建一个新的 Hash 实例
func MakeHash() *Hash {
	return &Hash{
		encoding: encodingListpack, // 默认使用 Listpack 编码
		listpack: make([][2]string, 0),
		dict:     make(map[string]string),
	}
}

// Get 从哈希中获取指定字段的值
func (h *Hash) Get(field string) (val string, exists bool) {
	// 如果使用 Listpack 编码，在 Listpack 中查找
	if h.encoding == encodingListpack {
		for _, entry := range h.listpack {
			if entry[0] == field {
				return entry[1], true
			}
		}
		return "", false
	}
	val, exists = h.dict[field]
	return
}

// Set 设置哈希中指定字段的值
// 如果字段已存在，则更新值并返回 0；如果不存在，则新增并返回 1
func (h *Hash) Set(field, value string) int {
	if h.encoding == encodingListpack {
		// 如果 Listpack 条目数或字段/值长度超过阈值，则转换为哈希表
		if len(h.listpack) >= hashMaxListpackEntries || len(field) > hashMaxListpackValue || len(value) > hashMaxListpackValue {
			h.convertToHashTable()
		}
	}
	if h.encoding == encodingListpack {
		// 检查字段是否已存在于 Listpack 中
		for i, entry := range h.listpack {
			if entry[0] == field {
				h.listpack[i][1] = value
				return 0 // 更新已有条目
			}
		}
		// 添加新条目
		h.listpack = append(h.listpack, [2]string{field, value})
		return 1
	}
	_, exists := h.dict[field]
	h.dict[field] = value
	if exists {
		return 0 // 更新已有条目
	}
	return 1 // 新增条目
}

// Delete 从哈希中删除指定字段
func (h *Hash) Delete(field string) int {
	count := 0
	if h.encoding == encodingListpack {
		for i, entry := range h.listpack {
			if entry[0] == field {
				// 删除条目：将最后一个条目移到当前位置（哈希无序，可交换）
				lastIndex := len(h.listpack) - 1
				h.listpack[i] = h.listpack[lastIndex]
				h.listpack = h.listpack[:lastIndex]
				count++
				break
			}
		}
	} else {
		// 从哈希表中删除字段
		if _, exists := h.dict[field]; exists {
			delete(h.dict, field)
			count++
		}
	}
	return count
}

// Len 返回哈希中的条目数量
func (h *Hash) Len() int {
	if h.encoding == encodingListpack {
		return len(h.listpack)
	}
	return len(h.dict)
}

// GetAll 返回哈希中所有的字段和值
func (h *Hash) GetAll() map[string]string {
	result := make(map[string]string)
	if h.encoding == encodingListpack {
		for _, entry := range h.listpack {
			result[entry[0]] = entry[1]
		}
	} else {
		for field, value := range h.dict {
			result[field] = value
		}
	}
	return result
}

// Fields 返回哈希中所有的字段
func (h *Hash) Fields() []string {
	if h.encoding == encodingListpack {
		fields := make([]string, len(h.listpack))
		for i, entry := range h.listpack {
			fields[i] = entry[0]
		}
		return fields
	}
	fields := make([]string, 0, len(h.dict))
	for field := range h.dict {
		fields = append(fields, field)
	}
	return fields
}

// Values 返回哈希中所有的值
func (h *Hash) Values() []string {
	if h.encoding == encodingListpack {
		values := make([]string, len(h.listpack))
		for i, entry := range h.listpack {
			values[i] = entry[1]
		}
		return values
	}
	values := make([]string, 0, len(h.dict))
	for _, value := range h.dict {
		values = append(values, value)
	}
	return values
}

// Exists 检查指定字段是否存在于哈希中
func (h *Hash) Exists(field string) bool {
	_, exists := h.Get(field)
	return exists
}

// convertToHashTable 将哈希从 Listpack 编码转换为哈希表编码
func (h *Hash) convertToHashTable() {
	if h.encoding == encodingHashTable {
		return
	}
	h.dict = make(map[string]string, len(h.listpack))
	for _, entry := range h.listpack {
		h.dict[entry[0]] = entry[1]
	}
	h.encoding = encodingHashTable
	h.listpack = nil // 清空 Listpack 释放内存
}

// Encoding 返回哈希当前的编码类型
func (h *Hash) Encoding() int {
	return h.encoding
}

// Clear 清空哈希中的所有条目
func (h *Hash) Clear() {
	h.listpack = nil
	h.dict = nil
	h.encoding = encodingListpack
}
