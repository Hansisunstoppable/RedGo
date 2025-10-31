package database

// 实现 KEYS 命令集
// DEL
// EXISTS
// FLUSHDB
// TYPE
// RENAME
// RENAMENX
// KEYS

import (
	"Godis/interface/resp"
	"Godis/lib/wildcard"
	"Godis/resp/reply"
)

// init 方法，导入包时自动执行，注册方法到 cmdTable
func init() {
	RegisterCommand("DEL", execDel, -2)
	RegisterCommand("EXIST", execExist, -2)
	RegisterCommand("FLUSHDB", execFlushDB, 1)
	RegisterCommand("TYPE", execType, 2)
	RegisterCommand("RENAME", execRename, 3)
	RegisterCommand("RENAMENX", execRenameNX, 3)
	RegisterCommand("KEYS", execKeys, 2)
}

// execDel Handle the DEL command
func execDel(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	for i, arg := range args {
		keys[i] = string(arg)
	}
	deleted := db.Removes(keys...)
	return reply.MakeIntReply(int64(deleted))
}

// execExist Handle the EXIST command
func execExist(db *DB, args [][]byte) resp.Reply {
	result := int64(0)
	for _, arg := range args {
		key := string(arg)
		if _, ok := db.GetEntity(key); ok {
			result++
		}
	}
	return reply.MakeIntReply(result)
}

// execFlushDB Handle the FLUSHDB command, delete all keys in current DB
func execFlushDB(db *DB, args [][]byte) resp.Reply {
	db.Flush()
	return reply.MakeOKReply()
}

// execType Handle the TYPE command
func execType(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	if entity, ok := db.GetEntity(key); ok {
		switch entity.Data.(type) {
		// If the entity is []byte, return the type as "string"
		case []byte:
			return reply.MakeBulkReply([]byte("string"))
		}
		// TODO Add more types as needed
	} else {
		return reply.MakeStatusReply("none")
	}
	return reply.MakeUnknownReply()
}

// execRename Handle the RENAME command, RENAME key1 key2, 将 key1 改名为 key2
func execRename(db *DB, args [][]byte) resp.Reply {
	scr := string(args[0])
	dst := string(args[1])
	if entity, ok := db.GetEntity(scr); ok {
		db.PutEntity(dst, entity) // 若 key2 存在，则会覆盖
		db.Remove(scr)
		return reply.MakeOKReply()
	} else {
		return reply.MakeStandardErrorReply("ERR no such key")
	}
}

// execRenameNX Handle the RENAMENX command, RENAMENX key1 key2, 若 key2 not exist, 才会插入。否则不执行
func execRenameNX(db *DB, args [][]byte) resp.Reply {
	scr := string(args[0])
	dst := string(args[1])
	entity, ok := db.GetEntity(scr)
	if !ok {
		return reply.MakeStandardErrorReply("ERR no such key")
	}
	if _, ok := db.GetEntity(dst); ok {
		return reply.MakeIntReply(0) // 若 key2 存在，则不执行
	}
	db.PutEntity(dst, entity)
	db.Remove(scr)
	return reply.MakeIntReply(1)
}

// execKeys Handle the KEYS command, return all keys satisfied given pattern
// *：匹配任意长度的任意字符（包括空字符）
// ?：匹配任意单个字符
// [abc]：匹配集合中的任意一个字符
// [a-z]：匹配范围内的任意一个字符
// [^abc]：不匹配集合中的任意一个字符
// 可以通过 \ 进行字符转义
// 举例：
// user:* 匹配所有以 user: 开头的 key
// session:?? 匹配如 session:ab, session:xy
// file[0-9].log 匹配如 file1.log, file9.log
// file[^0-9].log 表示不匹配任何以数字结尾的文件名，例如 fileA.log, fileB.log
func execKeys(db *DB, args [][]byte) resp.Reply {
	pattern := wildcard.CompilePattern(string(args[0]))
	result := make([][]byte, 0) // Store all matching keys
	db.data.ForEach(func(key string, val interface{}) bool {
		if pattern.IsMatch(key) {
			result = append(result, []byte(key)) // []byte(xxx): 将 xxx 转换为 []byte 类型
		}
		return true
	})
	return reply.MakeMultiBulkReply(result)
}
