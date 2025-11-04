package database

// 实现 LIST 相关命令
// List 是 Redis 中最基础的数据结构之一，它是一个有序的字符串集合
// LPUSH：将一个或多个值插入到列表的头部
// RPUSH：将一个或多个值插入到列表的尾部
// LPOP：移除并返回列表的第一个元素
// RPOP：移除并返回列表的最后一个元素
// LRANGE：返回列表中指定区间内的元素
// LLEN：返回列表的长度
// LINDEX：返回列表中指定索引的元素
// LSET：设置列表中指定索引的元素的值

import (
	// 使用 Go 标准库的 list 包
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/lib/util"
	"Godis/resp/reply"
	"container/list"
	"strconv"
)

// getAsList 获取指定 key 对应的列表，如果不存在则创建一个新列表。
// 返回列表和一个布尔值，表示 key 是否已存在。
func getAsList(db *DB, key string) (*list.List, bool) {
	entity, ok := db.GetEntity(key)
	if !ok {
		// key 不存在，创建新列表
		return list.New(), false
	}
	// key 存在，检查是否为 list 类型
	lst, ok := entity.Data.(*list.List)
	if !ok {
		// key 存在但类型不是 list
		return nil, true // 表示 key 存在但类型错误
	}
	return lst, true
}

// execLPush 实现 LPUSH 命令：将一个或多个值插入到列表头部（左端）
// LPUSH key value [value ...]
func execLPush(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]

	// 获取或创建列表
	lst, exists := getAsList(db, key)
	if lst == nil && exists {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	// 插入值到列表头部
	for _, value := range values {
		lst.PushFront(value) // 添加到头部（左侧）
	}

	// 更新数据库中的列表
	db.PutEntity(key, &database.DataEntity{Data: lst})
	db.addAof(util.ToCmdLineWithName("LPUSH", args...))

	// 返回列表当前长度
	return reply.MakeIntReply(int64(lst.Len()))
}

// execRPush 实现 RPUSH 命令：将一个或多个值追加到列表尾部（右端）
// RPUSH key value [value ...]
func execRPush(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	values := args[1:]

	// 获取或创建列表
	lst, exists := getAsList(db, key)
	if lst == nil && exists {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	// 追加值到列表尾部
	for _, value := range values {
		lst.PushBack(value) // 添加到尾部（右侧）
	}

	// 更新数据库中的列表
	db.PutEntity(key, &database.DataEntity{Data: lst})
	db.addAof(util.ToCmdLineWithName("RPUSH", args...))

	// 返回列表当前长度
	return reply.MakeIntReply(int64(lst.Len()))
}

// execLPop 实现 LPOP 命令：移除并返回列表的第一个元素（头部）
// LPOP key
func execLPop(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	// 检查列表是否为空
	if lst.Len() == 0 {
		return reply.MakeNullBulkReply()
	}

	// 移除并获取头部元素
	element := lst.Front()
	lst.Remove(element)
	value := element.Value.([]byte)

	// 如果列表变空，则删除 key
	if lst.Len() == 0 {
		db.Remove(key)
	} else {
		// 否则更新数据库中的列表
		db.PutEntity(key, &database.DataEntity{Data: lst})
	}

	db.addAof(util.ToCmdLineWithName("LPOP", args...))
	return reply.MakeBulkReply(value)
}

// execRPop 实现 RPOP 命令：移除并返回列表的最后一个元素（尾部）
// RPOP key
func execRPop(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	// 检查列表是否为空
	if lst.Len() == 0 {
		return reply.MakeNullBulkReply()
	}

	// 移除并获取尾部元素
	element := lst.Back()
	lst.Remove(element)
	value := element.Value.([]byte)

	// 如果列表变空，则删除 key
	if lst.Len() == 0 {
		db.Remove(key)
	} else {
		// 否则更新数据库中的列表
		db.PutEntity(key, &database.DataEntity{Data: lst})
	}

	db.addAof(util.ToCmdLineWithName("RPOP", args...))
	return reply.MakeBulkReply(value)
}

// execLRange 实现 LRANGE 命令：返回列表中指定区间内的元素
// LRANGE key start stop
func execLRange(db *DB, args [][]byte) resp.Reply {
	// 解析参数
	key := string(args[0])
	start, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}
	stop, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeEmptyMultiBulkReply()
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	// 处理负数索引
	size := int64(lst.Len())
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
	if start > stop {
		return reply.MakeEmptyMultiBulkReply()
	}

	// 收集指定范围内的元素
	elements := make([][]byte, 0, stop-start+1)
	index := int64(0)
	for e := lst.Front(); e != nil; e = e.Next() {
		if index >= start && index <= stop {
			elements = append(elements, e.Value.([]byte))
		} else if index > stop {
			break
		}
		index++
	}
	return reply.MakeMultiBulkReply(elements)
}

// execLLen 实现 LLEN 命令：返回列表的长度
// LLEN key
func execLLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeIntReply(0)
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}
	return reply.MakeIntReply(int64(lst.Len()))
}

// execLIndex 实现 LINDEX 命令：返回列表中指定索引位置的元素
// LINDEX key index
func execLIndex(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	index, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	size := int64(lst.Len())
	if index < 0 {
		index = size + index
	}
	if index < 0 || index >= size {
		return reply.MakeNullBulkReply()
	}

	// 查找指定索引的元素
	var element *list.Element
	if index < size/2 {
		// 索引在前半部分，从头部遍历
		element = lst.Front()
		for i := int64(0); i < index; i++ {
			element = element.Next()
		}
	} else {
		// 索引在后半部分，从尾部遍历
		element = lst.Back()
		for i := size - 1; i > index; i-- {
			element = element.Prev()
		}
	}
	return reply.MakeBulkReply(element.Value.([]byte))
}

// execLSet 实现 LSET 命令：将列表指定索引位置的元素设置为新值
// LSET key index value
func execLSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	index, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}
	value := args[2]

	// 获取列表
	lst, exists := getAsList(db, key)
	if !exists {
		return reply.MakeStandardErrorReply("no such key")
	}
	if lst == nil {
		// key 存在但不是 list 类型
		return reply.MakeWrongTypeErrReply()
	}

	size := int64(lst.Len())
	if index < 0 {
		index = size + index
	}
	if index < 0 || index >= size {
		return reply.MakeStandardErrorReply("index out of range")
	}

	// 查找并更新指定索引的元素
	var element *list.Element
	if index < size/2 {
		element = lst.Front()
		for i := int64(0); i < index; i++ {
			element = element.Next()
		}
	} else {
		element = lst.Back()
		for i := size - 1; i > index; i-- {
			element = element.Prev()
		}
	}
	element.Value = value

	db.PutEntity(key, &database.DataEntity{Data: lst})
	db.addAof(util.ToCmdLineWithName("LSET", args...))
	return reply.MakeOKReply()
}

func init() {
	// 注册列表相关命令
	// 参数数量为负数表示可变参数（key + 至少一个 value）
	RegisterCommand("LPUSH", execLPush, -3)  // key value [value ...] -> 至少 3 个参数
	RegisterCommand("RPUSH", execRPush, -3)  // key value [value ...] -> 至少 3 个参数
	RegisterCommand("LPOP", execLPop, 2)     // key
	RegisterCommand("RPOP", execRPop, 2)     // key
	RegisterCommand("LRANGE", execLRange, 4) // key start stop
	RegisterCommand("LLEN", execLLen, 2)     // LLEN key -> 恰好 2 个参数
	RegisterCommand("LINDEX", execLIndex, 3) // LINDEX key index -> 恰好 3 个参数
	RegisterCommand("LSET", execLSet, 4)     // LSET key index value -> 恰好 4 个参数
}
