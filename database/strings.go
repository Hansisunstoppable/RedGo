package database

import (
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/resp/reply"
)

// 实现 Strings 命令集
// GET
// SET
// SETNX
// GETSET
// STRLEN

func init() {
	RegisterCommand("GET", execGet, 2)
	RegisterCommand("SET", execSet, 3)
	RegisterCommand("SETNX", execSetNX, 3)
	RegisterCommand("GETSET", execGetSet, 3)
	RegisterCommand("STRLEN", execStrlen, 2)
}

// execGet handle the GET command
func execGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	if entity, ok := db.GetEntity(key); ok {
		// TODO: If we have multiple types, we need to check the conversion if it's not []byte
		return reply.MakeBulkReply(entity.Data.([]byte))
	}
	return reply.MakeNullBulkReply()
}

// execSet handle the SET command
func execSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{
		Data: value,
	}
	db.PutEntity(key, entity)
	return reply.MakeOKReply()
}

// execSetNX handle the SETNX command
func execSetNX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{
		Data: value,
	}
	result := db.PutIfAbsent(key, entity)
	return reply.MakeIntReply(int64(result))
}

// execGetSet handle the GETSET command, 设置键值对，并返回旧值
func execGetSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity, ok := db.GetEntity(key)
	db.PutEntity(key, &database.DataEntity{Data: value})
	if !ok { // 判断旧值是否存在
		return reply.MakeNullBulkReply()
	}
	return reply.MakeBulkReply(entity.Data.([]byte))
}

// execStrlen handle the STRLEN command
func execStrlen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	entity, ok := db.GetEntity(key)
	if !ok {
		return reply.MakeNullBulkReply()
	}
	return reply.MakeIntReply(int64(len(entity.Data.([]byte))))
}
