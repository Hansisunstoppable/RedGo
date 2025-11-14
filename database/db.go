package database

import (
	"Godis/datastruct/dict"
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/resp/reply"
	"strings"
	"sync"
)

const (
	dataDictSize = 1 << 16
)

// ExecFunc 传入 DB 实例 与 命令进行执行。所有命令处理函数共享相同的签名 ExecFunc
type ExecFunc func(db *DB, args [][]byte) resp.Reply

// PreFunc 返回需要 write 或 read 的 key
type PreFunc func(args [][]byte) ([]string, []string)

// CmdLine 存储命令行参数
type CmdLine = [][]byte

// DB 单个数据库实例，一个数据库可能包含多个数据库实例
type DB struct {
	index  int
	data   *dict.ConcurrentDict
	addAof func(CmdLine) // 用于执行 AOF 处理器的 AddAof 方法
	// lockMgr *KeyLockManager // key 级别锁管理器
}

func MakeDB() *DB {
	return &DB{
		data:   dict.MakeConcurrentDict(dataDictSize), // 内存数据库底层数据结构
		addAof: func(cmd CmdLine) {},                  // 初始化为空函数，防止第一次调用时出现空指针异常（execSet -> addAof -> NewAofHandler -> execSet -> addAof(未初始化，空指针异常）
		// lockMgr: NewKeyLockManager(),
	}
}

// KeyLockManager 管理所有 key 对应的锁
type KeyLockManager struct {
	locks sync.Map
}

func NewKeyLockManager() *KeyLockManager {
	return &KeyLockManager{}
}

func (db *DB) Exec(client resp.Connection, cmdLine CmdLine) resp.Reply {
	// TODO multi 事务模式的处理逻辑

	// 正常模式的处理逻辑
	return db.execNormalCommand(cmdLine)
}

func (db *DB) execNormalCommand(cmdLine [][]byte) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0])) // 读取命令名并转为小写
	cmd, ok := cmdTable[cmdName]                   // 获取命令处理函数与参数需求
	if !ok {
		return reply.MakeStandardErrorReply("ERR unknown command '" + cmdName + "'")
	}
	if !ValidateArity(cmd.arity, cmdLine) {
		return reply.MakeArgNumErrReply(cmdName)
	}
	// 获取 prepare 函数，加相关 key 锁
	prepare := cmd.prepare
	write, read := prepare(cmdLine[1:])
	db.RWLocks(write, read)
	defer db.RWUnLocks(write, read)
	fun := cmd.exec
	return fun(db, cmdLine[1:]) // // 从 1 开始，去掉命令名
}

// execWithLock 内部不加锁，调用者加锁
func (db *DB) execWithLock(cmdLine [][]byte) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0]))
	cmd, ok := cmdTable[cmdName]
	if !ok {
		return reply.MakeStandardErrorReply("ERR unknown command '" + cmdName + "'")
	}
	if !ValidateArity(cmd.arity, cmdLine) {
		return reply.MakeStandardErrorReply(cmdName)
	}
	fun := cmd.exec
	return fun(db, cmdLine[1:])
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
// 外部调用者调用这些方法时，已经上锁。因此这些方法内部无需上锁。
// GetEntity returns DataEntity bind to the given key
func (db *DB) GetEntity(key string) (*database.DataEntity, bool) {
	raw, ok := db.data.GetWithLock(key)
	if !ok {
		return nil, false
	}
	entity, _ := raw.(*database.DataEntity)
	return entity, true
}

// PutEntity stores the given DataEntity in the database
func (db *DB) PutEntity(key string, entity *database.DataEntity) int {
	return db.data.PutWithLock(key, entity)
}

// PutIfExists stores the given DataEntity in the database if the key already exists
func (db *DB) PutIfExists(key string, entity *database.DataEntity) int {
	return db.data.PutIfExistsWithLock(key, entity)
}

// PutIfAbsent stores the given DataEntity in the database if the key does not exist
func (db *DB) PutIfAbsent(key string, entity *database.DataEntity) int {
	return db.data.PutIfAbsentWithLock(key, entity)
}

// Remove 根据给定 key，移除元素
func (db *DB) Remove(key string) int {
	result := db.data.RemoveWithLock(key) // 内部不加锁，锁已经由外部调用者管理

	return result
}

// Removes 移除若干元素
func (db *DB) Removes(key ...string) int {
	deleted := 0
	for _, key := range key {
		_, ok := db.data.GetWithLock(key)
		if ok {
			db.Remove(key)
			deleted++
		}
	}
	return deleted
}

// Flush 移除所有元素
func (db *DB) Flush() {
	db.data.Clear()
}

/* ---- Lock Function ----- */

// RWLocks 对相关 key 加写锁与读锁
func (db *DB) RWLocks(writeKeys []string, readKeys []string) {
	db.data.RWLocks(writeKeys, readKeys)
}

// RWUnLocks 对相关 key 解开写锁与读锁
func (db *DB) RWUnLocks(writeKeys []string, readKeys []string) {
	db.data.RWUnLocks(writeKeys, readKeys)
}
