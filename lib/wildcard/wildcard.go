package wildcard

// 实现 KEYS 命令的模式匹配
// wildcard: 通配符

// typeCode
const (
	normal     = iota // 接下来的常量，从 0 开始递增
	all               // *
	any               // ?
	setSymbol         // []
	rangSymbol        // [a-b]
	negSymbol         // [^a]
)

// item 存储字符模式，set 字段为 [xxx] 这类范围通对应的字符集合
type item struct {
	character byte
	set       map[byte]bool
	typeCode  int
}

// contains 判断字符模式是否包含或者排除了 c，针对 [a-b] 与 [^a] 类
func (i *item) contains(c byte) bool {
	if i.typeCode == setSymbol { // [abc] 类
		_, ok := i.set[c]
		return ok
	} else if i.typeCode == rangSymbol { // [a-b] 类
		// TODO 以下代码可注释？
		// if _, ok := i.set[c]; ok {
		// 	return true
		// }
		var (
			min uint8 = 255
			max uint8 = 0
		)
		for k := range i.set {
			if min > k {
				min = k
			}
			if max < k {
				max = k
			}
		}
		return c >= min && c <= max // 范围内即可
	} else if i.typeCode == negSymbol { // [^a-b] 类, 排除匹配
		_, ok := i.set[c]
		return !ok
	} else { // 单字符
		_, ok := i.set[c]
		return !ok
	}
}

// Pattern 包含 item 切片，能够实现匹配
type Pattern struct {
	items []*item
}

// CompilePattern 将字符串编译为字符模式
func CompilePattern(src string) *Pattern {
	items := make([]*item, 0)
	escape := false // 接下来的字符是否需要转义
	inSet := false  // 是否在处理 [] 类范围通配，要将范围内的字符加入 item.set
	var set map[byte]bool
	for _, v := range src {
		c := byte(v)
		if escape { // 当前字符需要被转义，不论是什么都直接视作普通字符串
			items = append(items, &item{typeCode: normal, character: c})
			escape = false
		} else if c == '*' {
			items = append(items, &item{typeCode: all})
		} else if c == '?' {
			items = append(items, &item{typeCode: any})
		} else if c == '\\' {
			escape = true
		} else if c == '[' { // 遇到字符集匹配
			if !inSet {
				inSet = true // 接下里遇到 ] 之前，之间的字符都要加入 item.set
				set = make(map[byte]bool)
			} else {
				set[c] = true
			}
		} else if c == ']' { // 结束字符集匹配
			if inSet {
				inSet = false
				typeCode := setSymbol
				if _, ok := set['-']; ok { // 存在 -，范围匹配
					typeCode = rangSymbol
					delete(set, '-')
				}
				if _, ok := set['^']; ok { // 存在 ^，取反匹配
					typeCode = negSymbol
					delete(set, '^')
				}
				items = append(items, &item{typeCode: typeCode, set: set})
			} else {
				items = append(items, &item{typeCode: normal, character: c})
			}
		} else {
			if inSet {
				set[c] = true // 添加进字符集
			} else {
				items = append(items, &item{typeCode: normal, character: c}) // 最普通的正常字符
			}
		}
	}
	return &Pattern{
		items: items,
	}
}

// IsMatch 判断 key 是否匹配 pattern
func (p *Pattern) IsMatch(key string) bool {
	if len(p.items) == 0 {
		return len(key) == 0
	}
	// 注意，m 和 n 不一定相等，因为如 [a-b] 这样多个字符会被编译为 一个模式 item
	n := len(p.items)
	m := len(key)

	table := make([][]bool, m+1) // 创建二维 dp 数组
	for i := 0; i < m+1; i++ {
		table[i] = make([]bool, n+1)
	}
	table[0][0] = true // 代表 key[0] 与 模式串[0] 一段能匹配（空配空）
	for j := 1; j < n+1; j++ {
		// 模式串[j] 为 *, 则一定能匹配; 注意 模式串[j] 的下标为 j - 1
		table[0][j] = table[0][j-1] && p.items[j-1].typeCode == all
	}

	for i := 1; i < m+1; i++ {
		for j := 1; j < n+1; j++ {
			if p.items[j-1].typeCode == all { // 注意模式串[j] 的下标为 j - 1
				table[i][j] = table[i][j-1] || table[i-1][j] // 两种情况，别漏
			} else {
				if !table[i-1][j-1] { // 前一段匹配失败，且当前不是通配符 *，一定无法匹配
					continue
				}
				switch p.items[j-1].typeCode {
				case any: // 单字符通配
					table[i][j] = true
				case normal: // 普通字符匹配
					if p.items[j-1].character == uint8(key[i-1]) {
						table[i][j] = true
					}
				default: // 其余 [] 类范围匹配
					if p.items[j-1].contains(uint8(key[i-1])) && p.items[j-1].typeCode >= setSymbol {
						table[i][j] = true
					}
				}
			}
		}
	}
	return table[m][n] // table[m][n] 代表 key[0...m] 与 p[0...n] 能否匹配成功
}
