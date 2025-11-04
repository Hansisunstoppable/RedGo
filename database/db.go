package database

import (
	"Godis/datastruct/dict"
	"Godis/datastruct/hash"
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/resp/reply"
	"strings"
)

// ExecFunc 传入 DB 实例 与 命令进行执行。所有命令处理函数共享相同的签名 ExecFunc
type ExecFunc func(db *DB, args [][]byte) resp.Reply

// CmdLine 存储命令行参数
type CmdLine = [][]byte

// DB 单个数据库实例，一个数据库可能包含多个数据库实例
type DB struct {
	index  int
	data   dict.Dict
	addAof func(CmdLine) // 用于执行 AOF 处理器的 AddAof 方法
}

func MakeDB() *DB {
	return &DB{
		data:   dict.MakeSyncDict(),  // 内存数据库底层数据结构
		addAof: func(cmd CmdLine) {}, // 初始化为空函数，防止第一次调用时出现空指针异常（execSet -> addAof -> NewAofHandler -> execSet -> addAof(未初始化，空指针异常）
	}
}

func (db *DB) Exec(client resp.Connection, cmdLine CmdLine) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0])) // 读取命令名并转为小写
	cmd, ok := cmdTable[cmdName]                   // 获取命令处理函数与参数需求
	if !ok {
		return reply.MakeStandardErrorReply("ERR unknown command '" + cmdName + "'")
	}
	if !ValidateArity(cmd.arity, cmdLine) {
		return reply.MakeArgNumErrReply(cmdName)
	}

	// 从 1 开始，去掉命令名
	return cmd.exec(db, cmdLine[1:])
}

// ValidateArity 验证传入的参数个数是否合法
func ValidateArity(arity int, args [][]byte) bool {
	if arity >= 0 {
		return arity == len(args)
	} else {
		return -arity <= len(args)
	}
}

func (db *DB) AfterClientClose(client resp.Connection) {

}
func (db *DB) Close() {

}

// 下面这一系列方法，是对底层 data  dict.Dict 的封装
// GetEntity returns DataEntity bind to the given key
func (db *DB) GetEntity(key string) (*database.DataEntity, bool) {
	raw, ok := db.data.Get(key)
	if !ok {
		return nil, false
	}
	entity, _ := raw.(*database.DataEntity)
	return entity, true
}

// PutEntity stores the given DataEntity in the database
func (db *DB) PutEntity(key string, entity *database.DataEntity) int {
	return db.data.Put(key, entity)
}

// PutIfExists stores the given DataEntity in the database if the key already exists
func (db *DB) PutIfExists(key string, entity *database.DataEntity) int {
	return db.data.PutIfExists(key, entity)
}

// PutIfAbsent stores the given DataEntity in the database if the key does not exist
func (db *DB) PutIfAbsent(key string, entity *database.DataEntity) int {
	return db.data.PutIfAbsent(key, entity)
}

// Remove removes the given key from database
func (db *DB) Remove(key string) int {
	return db.data.Remove(key)
}

// Removes removes the given keys from database
func (db *DB) Removes(key ...string) int {
	deleted := 0
	for _, key := range key {
		_, ok := db.data.Get(key)
		if ok {
			db.data.Remove(key)
			deleted++
		}
	}
	return deleted
}

// Flush removes all data in database
func (db *DB) Flush() {
	db.data.Clear()
}

// getAsHash 返回指定 key 对应的哈希表对象，如果 key 不存在则返回 nil
func (db *DB) getAsHash(key string) (*hash.Hash, bool) {
	entity, exists := db.GetEntity(key)
	if !exists {
		return nil, false // key 不存在
	}
	hashObj, ok := entity.Data.(*hash.Hash)
	if !ok {
		return nil, true // key 存在但不是哈希类型
	}
	return hashObj, true // 成功返回哈希对象
}

// getOrCreateHash 获取或创建哈希表对象
// 如果 key 对应的值已存在且为哈希类型，则返回已有对象；
// 如果 key 对应的值不存在，则创建新哈希并存入数据库；
// 返回哈希对象和一个布尔值表示 key 是否已存在
func (db *DB) getOrCreateHash(key string) (*hash.Hash, bool) {
	hashObj, exists := db.getAsHash(key)
	if exists {
		return hashObj, true // key 已存在，直接返回
	}
	// key 不存在，创建新的哈希表对象
	hashObj = hash.MakeHash()
	db.PutEntity(key, &database.DataEntity{Data: hashObj})
	return hashObj, false // 新创建的哈希表
}
