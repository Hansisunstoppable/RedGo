package hash

import (
	"strconv"
	"testing"
)

// TestMakeHash 测试新建哈希结构
func TestMakeHash(t *testing.T) {
	h := MakeHash()
	if h == nil {
		t.Fatal("创建哈希失败")
	}
	if h.encoding != encodingListpack {
		t.Errorf("新建哈希默认应使用 listpack 编码，实际得到 %d", h.encoding)
	}
	if len(h.listpack) != 0 {
		t.Errorf("新建哈希的 listpack 应为空，实际包含 %d 个条目", len(h.listpack))
	}
}

// TestSetAndGet 测试基本的 Set 和 Get 操作
func TestSetAndGet(t *testing.T) {
	h := MakeHash()

	// 测试设置新字段
	result := h.Set("name", "test")
	if result != 1 {
		t.Errorf("新增字段时 Set 应返回 1，实际返回 %d", result)
	}

	// 测试获取已存在的字段
	value, exists := h.Get("name")
	if !exists {
		t.Error("Set 后字段应存在")
	}
	if value != "test" {
		t.Errorf("期望值为 'test'，实际得到 '%s'", value)
	}

	// 测试更新已存在的字段
	result = h.Set("name", "updated")
	if result != 0 {
		t.Errorf("更新已有字段时 Set 应返回 0，实际返回 %d", result)
	}
	value, exists = h.Get("name")
	if !exists {
		t.Error("更新后字段仍应存在")
	}
	if value != "updated" {
		t.Errorf("期望值为 'updated'，实际得到 '%s'", value)
	}

	// 测试获取不存在的字段
	_, exists = h.Get("nonexistent")
	if exists {
		t.Error("不存在的字段应返回 false")
	}
}

// TestDelete 测试 Delete 操作
func TestDelete(t *testing.T) {
	h := MakeHash()
	h.Set("field1", "value1")
	h.Set("field2", "value2")

	// 测试删除存在的字段
	count := h.Delete("field1")
	if count != 1 {
		t.Errorf("删除存在的字段应返回 1，实际返回 %d", count)
	}

	// 验证字段已被删除
	_, exists := h.Get("field1")
	if exists {
		t.Error("删除后字段不应存在")
	}

	// 测试删除不存在的字段
	count = h.Delete("nonexistent")
	if count != 0 {
		t.Errorf("删除不存在的字段应返回 0，实际返回 %d", count)
	}
}

// TestEncoding 测试从 listpack 到 hashtable 的编码转换
func TestEncoding(t *testing.T) {
	h := MakeHash()

	// 初始编码应为 listpack
	if h.Encoding() != encodingListpack {
		t.Errorf("初始编码应为 listpack，实际为 %d", h.Encoding())
	}

	// 添加少量条目（低于阈值）
	for i := 0; i < 10; i++ {
		h.Set("key"+strconv.Itoa(i), "value")
	}
	// 编码仍应为 listpack
	if h.Encoding() != encodingListpack {
		t.Errorf("条目数较少时编码应仍为 listpack，实际为 %d", h.Encoding())
	}

	// 添加超大值触发编码转换
	largeValue := string(make([]byte, hashMaxListpackValue+1))
	h.Set("largeKey", largeValue)
	// 编码应变为 hashtable
	if h.Encoding() != encodingHashTable {
		t.Errorf("值过大时编码应转为 hashtable，实际为 %d", h.Encoding())
	}

	// 验证编码转换后数据完整性
	for i := 0; i < 10; i++ {
		val, exists := h.Get("key" + strconv.Itoa(i))
		if !exists || val != "value" {
			t.Errorf("编码转换后数据出现损坏")
		}
	}
}

// TestLargeNumberOfEntries 测试因条目数过多导致的编码转换
func TestLargeNumberOfEntries(t *testing.T) {
	h := MakeHash()

	// 添加超过阈值的条目
	for i := 0; i < hashMaxListpackEntries+1; i++ {
		h.Set("key"+strconv.Itoa(i), "value")
	}

	// 编码应转为 hashtable
	if h.Encoding() != encodingHashTable {
		t.Errorf("超过条目限制后编码应为 hashtable，实际为 %d", h.Encoding())
	}

	// 验证部分条目数据
	for i := 0; i < 10; i++ {
		val, exists := h.Get("key" + strconv.Itoa(i))
		if !exists || val != "value" {
			t.Errorf("编码转换后数据出现损坏")
		}
	}
}

// TestOtherOperations 测试其他哈希操作
func TestOtherOperations(t *testing.T) {
	h := MakeHash()
	h.Set("field1", "value1")
	h.Set("field2", "value2")

	// 测试 Len
	if h.Len() != 2 {
		t.Errorf("期望长度为 2，实际为 %d", h.Len())
	}

	// 测试 Exists
	if !h.Exists("field1") {
		t.Error("field1 应存在")
	}
	if h.Exists("nonexistent") {
		t.Error("不存在的字段不应存在")
	}

	// 测试 GetAll
	all := h.GetAll()
	if len(all) != 2 || all["field1"] != "value1" || all["field2"] != "value2" {
		t.Error("GetAll 返回数据不正确")
	}

	// 测试 Fields
	fields := h.Fields()
	if len(fields) != 2 {
		t.Errorf("期望 2 个字段，实际得到 %d 个", len(fields))
	}
	// 检查字段内容（顺序可能不同）
	fieldMap := make(map[string]bool)
	for _, f := range fields {
		fieldMap[f] = true
	}
	if !fieldMap["field1"] || !fieldMap["field2"] {
		t.Error("Fields 返回字段不正确")
	}

	// 测试 Values
	values := h.Values()
	if len(values) != 2 {
		t.Errorf("期望 2 个值，实际得到 %d 个", len(values))
	}
	// 检查值内容（顺序可能不同）
	valueMap := make(map[string]bool)
	for _, v := range values {
		valueMap[v] = true
	}
	if !valueMap["value1"] || !valueMap["value2"] {
		t.Error("Values 返回值不正确")
	}
}
