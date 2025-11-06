package set

import (
	"strconv"
	"testing"
)

// TestNewHashSet 测试新集合结构的创建
func TestNewHashSet(t *testing.T) {
	set := NewHashSet()
	if set == nil {
		t.Fatal("无法创建新的哈希集合")
	}
	if !set.isIntset {
		t.Errorf("新建的哈希集合默认应使用 intset 编码")
	}
	if set.Len() != 0 {
		t.Errorf("新建的哈希集合应为空，实际有 %d 个元素", set.Len())
	}
}

// TestAddAndContains 测试基本的添加和包含操作
func TestAddAndContains(t *testing.T) {
	set := NewHashSet()
	// 测试添加整数
	result := set.Add("42")
	if result != 1 {
		t.Errorf("添加新成员时 Add 应返回 1，实际返回 %d", result)
	}
	// 测试已存在成员的包含性
	exists := set.Contains("42")
	if !exists {
		t.Error("添加后应能检测到成员存在")
	}
	// 测试添加重复成员
	result = set.Add("42")
	if result != 0 {
		t.Errorf("添加已存在成员时 Add 应返回 0，实际返回 %d", result)
	}
	// 测试不存在成员的包含性
	exists = set.Contains("99")
	if exists {
		t.Error("不存在的成员应返回 false")
	}
}

// TestRemove 测试 Remove 操作
func TestRemove(t *testing.T) {
	set := NewHashSet()
	set.Add("100")
	set.Add("200")
	// 测试移除存在的成员
	count := set.Remove("100")
	if count != 1 {
		t.Errorf("移除存在的成员时 Remove 应返回 1，实际返回 %d", count)
	}
	// 验证成员已被移除
	exists := set.Contains("100")
	if exists {
		t.Error("移除后成员不应再存在")
	}
	// 测试移除不存在的成员
	count = set.Remove("nonexistent")
	if count != 0 {
		t.Errorf("移除不存在的成员时 Remove 应返回 0，实际返回 %d", count)
	}
}

// TestEncoding 测试从 intset 到哈希表的编码转换
func TestEncoding(t *testing.T) {
	set := NewHashSet()
	// 初始编码应为 intset
	if !set.isIntset {
		t.Error("初始编码应为 intset")
	}
	// 添加字符串成员以触发编码转换
	set.Add("abc")
	// 编码现在应为哈希表
	if set.isIntset {
		t.Error("添加非整数后编码应为哈希表")
	}
	// 验证编码转换后的数据完整性
	if !set.Contains("abc") {
		t.Error("编码转换后成员仍应存在")
	}
	// 测试添加混合数据
	set.Add("123")
	set.Add("def")
	if !set.Contains("123") || !set.Contains("def") {
		t.Error("混合添加后集合应包含所有成员")
	}
}

// TestLargeNumberOfEntries 测试因元素数量过多而触发的编码转换
func TestLargeNumberOfEntries(t *testing.T) {
	set := NewHashSet()
	// 添加足够多的元素以触发基于数量的编码转换
	for i := 0; i < SET_MAX_INTSET_ENTRIES+1; i++ {
		set.Add(strconv.Itoa(i))
	}
	// 编码现在应为哈希表
	if set.isIntset {
		t.Error("超出元素数量限制后编码应为哈希表")
	}
	// 验证部分随机元素
	for i := 0; i < 10; i++ {
		if !set.Contains(strconv.Itoa(i)) {
			t.Error("编码转换后数据完整性出现问题")
		}
	}
}

// TestMembers 测试 Members 方法
func TestMembers(t *testing.T) {
	set := NewHashSet()
	set.Add("100")
	set.Add("200")
	set.Add("300")
	members := set.Members()
	if len(members) != 3 {
		t.Errorf("预期有 3 个成员，实际得到 %d 个", len(members))
	}
	// 检查成员内容（集合无序）
	memberMap := make(map[string]bool)
	for _, m := range members {
		memberMap[m] = true
	}
	if !memberMap["100"] || !memberMap["200"] || !memberMap["300"] {
		t.Error("Members 返回的数据不正确")
	}
}

// TestForEach 测试 ForEach 方法
func TestForEach(t *testing.T) {
	set := NewHashSet()
	set.Add("100")
	set.Add("200")
	set.Add("300")
	count := 0
	set.ForEach(func(member string) bool {
		count++
		return true
	})
	if count != 3 {
		t.Errorf("ForEach 应遍历全部 3 个成员，实际遍历 %d 个", count)
	}
	// 测试提前终止
	count = 0
	set.ForEach(func(member string) bool {
		count++
		return count < 2 // 遍历前两个后终止
	})
	if count != 2 {
		t.Errorf("提前终止时 ForEach 应遍历 2 个成员，实际遍历 %d 个", count)
	}
}

// TestMixedDataTypes 测试同时包含整数和字符串成员的集合
func TestMixedDataTypes(t *testing.T) {
	set := NewHashSet()
	// 先添加整数
	set.Add("10")
	set.Add("20")
	// 添加字符串触发编码转换
	set.Add("abc")
	if set.isIntset {
		t.Error("添加非整数后集合应使用哈希表编码")
	}
	// 检查所有值是否保留
	if !set.Contains("10") || !set.Contains("20") || !set.Contains("abc") {
		t.Error("编码转换后集合应包含所有成员")
	}
	// 继续添加混合数据
	set.Add("30")
	set.Add("def")
	// 检查长度
	if set.Len() != 5 {
		t.Errorf("预期有 5 个成员，实际有 %d 个", set.Len())
	}
}

// TestRandomMembers 测试带替换的随机成员获取
func TestRandomMembers(t *testing.T) {
	set := NewHashSet()
	for i := 0; i < 100; i++ {
		set.Add(strconv.Itoa(i))
	}
	// 获取 10 个随机成员（允许重复）
	random := set.RandomMembers(10)
	if len(random) != 10 {
		t.Errorf("预期获取 10 个随机成员，实际获取 %d 个", len(random))
	}
	// 所有返回的成员都应存在于原集合中
	for _, m := range random {
		if !set.Contains(m) {
			t.Errorf("随机成员 %s 在原集合中未找到", m)
		}
	}
	// 获取 0 个随机成员
	random = set.RandomMembers(0)
	if len(random) != 0 {
		t.Errorf("请求 0 个成员时应返回空列表，实际返回 %d 个", len(random))
	}
	// 请求超过集合大小的随机成员
	random = set.RandomMembers(200)
	if len(random) != 200 {
		t.Errorf("预期返回 200 个随机成员，实际返回 %d 个", len(random))
	}
}

// TestRandomDistinctMembers 测试不重复的随机成员获取
func TestRandomDistinctMembers(t *testing.T) {
	set := NewHashSet()
	for i := 0; i < 100; i++ {
		set.Add(strconv.Itoa(i))
	}
	// 获取 50 个不重复的随机成员
	random := set.RandomDistinctMembers(50)
	if len(random) != 50 {
		t.Errorf("预期获取 50 个随机成员，实际获取 %d 个", len(random))
	}
	// 检查唯一性
	uniqueCheck := make(map[string]bool)
	for _, m := range random {
		if uniqueCheck[m] {
			t.Error("RandomDistinctMembers 返回了重复值")
		}
		uniqueCheck[m] = true
		// 验证成员存在于原集合中
		if !set.Contains(m) {
			t.Errorf("随机成员 %s 在原集合中未找到", m)
		}
	}
	// 获取 0 个随机成员
	random = set.RandomDistinctMembers(0)
	if len(random) != 0 {
		t.Errorf("请求 0 个成员时应返回空列表，实际返回 %d 个", len(random))
	}
	// 请求超过集合大小的随机成员
	random = set.RandomDistinctMembers(200)
	if len(random) != 100 {
		t.Errorf("请求数超过集合大小时应返回全部 100 个成员，实际返回 %d 个", len(random))
	}
}

// TestEmptySet 测试空集合上的操作
func TestEmptySet(t *testing.T) {
	set := NewHashSet()
	// 测试长度
	if set.Len() != 0 {
		t.Errorf("空集合长度应为 0，实际为 %d", set.Len())
	}
	// 测试包含性
	if set.Contains("anything") {
		t.Error("空集合不应包含任何成员")
	}
	// 测试成员列表
	members := set.Members()
	if len(members) != 0 {
		t.Errorf("空集合应返回空成员列表，实际返回 %d 个元素", len(members))
	}
	// 测试移除
	result := set.Remove("anything")
	if result != 0 {
		t.Errorf("在空集合上移除应返回 0，实际返回 %d", result)
	}
	// 测试随机成员
	random := set.RandomMembers(10)
	if len(random) != 0 {
		t.Errorf("在空集合上获取随机成员应返回空列表，实际返回 %d 个", len(random))
	}
}

// TestIntSetSpecificBehavior 测试 intset 实现特有的行为
func TestIntSetSpecificBehavior(t *testing.T) {
	set := NewHashSet()
	// 添加应存储在 intset 中的整数
	set.Add("1")
	set.Add("2")
	set.Add("3")
	// 验证仍使用 intset
	if !set.isIntset {
		t.Error("集合仍应使用 intset 编码")
	}
	// 移除一个值
	set.Remove("2")
	// 检查剩余值
	if !set.Contains("1") || set.Contains("2") || !set.Contains("3") {
		t.Error("intset 模式下的移除操作不正确")
	}
	// 添加一个极大整数，仍应兼容 intset
	set.Add("9223372036854775807") // 最大 int64 值
	if !set.isIntset {
		t.Error("包含大整数时集合仍应使用 intset 编码")
	}
	if !set.Contains("9223372036854775807") {
		t.Error("集合应包含最大 int64 值")
	}
	// 添加非整数以强制转换
	set.Add("abc")
	if set.isIntset {
		t.Error("添加非整数后集合应转换为哈希表")
	}
	// 验证所有值在转换后仍然存在
	if !set.Contains("1") || !set.Contains("3") ||
		!set.Contains("9223372036854775807") || !set.Contains("abc") {
		t.Error("从 intset 转换为哈希表时部分值丢失")
	}
}
