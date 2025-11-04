package database

import (
	"Godis/interface/resp"
	"Godis/lib/util"
	"Godis/resp/reply"
)

// HSet 设置哈希中指定字段的值
// HSET key field value
func execHSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	field := string(args[1])
	value := string(args[2])

	hashObj, _ := db.getOrCreateHash(key)
	result := hashObj.Set(field, value)

	db.addAof(util.ToCmdLineWithName("HSET", args...))
	return reply.MakeIntReply(int64(result)) // 返回 1 表示新增，0 表示更新
}

// HGet 获取哈希中指定字段的值
// HGET key field
func execHGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	field := string(args[1])

	hash, exists := db.getAsHash(key) // 获取哈希表对象
	if !exists {
		return reply.MakeNullBulkReply() // key 不存在
	}

	value, exists := hash.Get(field)
	if !exists {
		return reply.MakeNullBulkReply() // field 不存在
	}
	return reply.MakeBulkReply([]byte(value))
}

// HExists 检查哈希中指定字段是否存在
// HEXISTS key field
func execHExists(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	field := string(args[1])

	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeIntReply(0) // key 不存在
	}

	if hash.Exists(field) {
		return reply.MakeIntReply(1) // 字段存在
	}
	return reply.MakeIntReply(0) // 字段不存在
}

// HDel 从哈希中删除一个或多个字段
// HDEL key field [field ...]
func execHDel(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeIntReply(0) // key 不存在
	}

	deleted := 0
	for _, field := range args[1:] {
		deleted += hash.Delete(string(field))
	}

	// 如果哈希变为空，则删除整个 key
	if hash.Len() == 0 {
		db.Remove(key)
	}

	if deleted > 0 {
		db.addAof(util.ToCmdLineWithName("hdel", args...))
	}
	return reply.MakeIntReply(int64(deleted))
}

// HLen 返回哈希中的字段数量
// HLEN key
func execHLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeIntReply(0)
	}
	return reply.MakeIntReply(int64(hash.Len()))
}

// HGetAll 返回哈希中所有的字段和值（交替排列）
// HGETALL key → [field1, value1, field2, value2, ...]
func execHGetAll(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeEmptyMultiBulkReply()
	}

	allMap := hash.GetAll()
	result := make([][]byte, 0, len(allMap)*2)
	for field, value := range allMap {
		result = append(result, []byte(field))
		result = append(result, []byte(value))
	}
	return reply.MakeMultiBulkReply(result)
}

// HKeys 返回哈希中所有的字段
// HKEYS key
func execHKeys(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeEmptyMultiBulkReply()
	}

	fields := hash.Fields()
	result := make([][]byte, len(fields))
	for i, field := range fields {
		result[i] = []byte(field)
	}
	return reply.MakeMultiBulkReply(result)
}

// HVals 返回哈希中所有的值
// HVALS key
func execHVals(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeEmptyMultiBulkReply()
	}

	values := hash.Values()
	result := make([][]byte, len(values))
	for i, value := range values {
		result[i] = []byte(value)
	}
	return reply.MakeMultiBulkReply(result)
}

// HMGet 批量获取哈希中多个字段的值
// HMGET key field1 field2 ... → [value1, value2, ...]，不存在的字段返回 nil
func execHMGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		// key 不存在，所有字段返回 nil
		results := make([][]byte, len(args)-1)
		for i := range results {
			results[i] = nil
		}
		return reply.MakeMultiBulkReply(results)
	}

	results := make([][]byte, len(args)-1)
	for i, field := range args[1:] {
		value, exists := hash.Get(string(field))
		if exists {
			results[i] = []byte(value)
		} else {
			results[i] = nil
		}
	}
	return reply.MakeMultiBulkReply(results)
}

// HMSet 批量设置哈希中的多个字段值
// HMSET key field1 value1 [field2 value2 ...]
func execHMSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	if len(args)%2 == 0 {
		return reply.MakeStandardErrorReply("ERR wrong number of arguments for 'hmset' command")
	}

	hash, _ := db.getOrCreateHash(key)
	for i := 1; i < len(args); i += 2 {
		field := string(args[i])
		value := string(args[i+1])
		hash.Set(field, value)
	}

	db.addAof(util.ToCmdLineWithName("hmset", args...))
	return reply.MakeOKReply()
}

// HEncoding 返回哈希的内部编码类型（自定义调试命令）
// 0 表示 listpack，1 表示 hashtable
func execHEncoding(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	hash, exists := db.getAsHash(key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	return reply.MakeIntReply(int64(hash.Encoding()))
}

// HSetNX 仅当字段不存在时才设置值
// HSETNX key field value → 1 表示设置成功，0 表示字段已存在
func execHSetNX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	field := string(args[1])
	value := string(args[2])

	hash, _ := db.getOrCreateHash(key)
	_, exists := hash.Get(field)
	if exists {
		return reply.MakeIntReply(0) // 字段已存在，不修改
	}

	hash.Set(field, value)
	db.addAof(util.ToCmdLineWithName("HSETNX", args...))
	return reply.MakeIntReply(1) // 设置成功
}

func init() {
	// 注册哈希相关命令
	RegisterCommand("HSET", execHSet, 4)           // HSET key field value
	RegisterCommand("HGET", execHGet, 3)           // HGET key field
	RegisterCommand("HEXISTS", execHExists, 3)     // HEXISTS key field
	RegisterCommand("HDEL", execHDel, -3)          // HDEL key field [field ...]（至少 3 个参数）
	RegisterCommand("HLEN", execHLen, 2)           // HLEN key
	RegisterCommand("HGETALL", execHGetAll, 2)     // HGETALL key
	RegisterCommand("HKEYS", execHKeys, 2)         // HKEYS key
	RegisterCommand("HVALS", execHVals, 2)         // HVALS key
	RegisterCommand("HMGET", execHMGet, -3)        // HMGET key field [field ...]（至少 3 个参数）
	RegisterCommand("HMSET", execHMSet, -4)        // HMSET key field value [field value ...]（至少 4 个参数）
	RegisterCommand("HENCODING", execHEncoding, 2) // HENCODING key（自定义）
	RegisterCommand("HSETNX", execHSetNX, 4)       // HSETNX key field value
}
