package database

import (
	"Godis/config"
	"Godis/interface/resp"
	"Godis/lib/logger"
	"Godis/resp/reply"
	"strconv"
	"strings"
)

// Database 存储所有 DB 实例
type Database struct {
	dbSet []*DB
}

// MakeDatabase 创建数据库, 默认创建 16 个 DB 实例
func NewDatabase() *Database {
	database := &Database{}
	// 若配置文件中为配置，使用默认参数
	if config.Properties.Databases == 0 {
		config.Properties.Databases = 16
	}
	// 一个数据库默认对应 16 DB 实例
	database.dbSet = make([]*DB, config.Properties.Databases)
	for i := range database.dbSet {
		db := MakeDB()
		db.index = i           // 设置 DB 实例编号
		database.dbSet[i] = db // 加入到数据库集合中
	}
	return database
}

// execSelect 根据 args[0] 去设置 Connection.selected, 即对应的 DB 实例
func (d *Database) execSelect(c resp.Connection, database *Database, args [][]byte) resp.Reply {
	dbIndex, err := strconv.Atoi(string(args[0]))
	if err != nil {
		return reply.MakeStandardErrorReply("ERR invalid DB index")
	}
	if dbIndex < 0 || dbIndex >= len(database.dbSet) {
		return reply.MakeStandardErrorReply("ERR DB index out of range")
	}
	c.SelectDB(dbIndex)
	return reply.MakeOKReply()
}

// Exec 提供给 handler 调用，所有命令都通过 Exec 统一处理
func (d *Database) Exec(c resp.Connection, args [][]byte) resp.Reply {
	defer func() { // 捕获 panic, 进行 recover 记录日志
		if err := recover(); err != nil {
			logger.Error("Database Exec panic:" + err.(error).Error())
		}
	}()
	cmdName := strings.ToLower(string(args[0]))
	if cmdName == "select" { // 若为 select 命令，则进行选择 DB 实例
		if len(args) != 2 {
			return reply.MakeArgNumErrReply("select")
		}
		return d.execSelect(c, d, args)
	}
	db := d.dbSet[c.GetDBIndex()] // 获取 DB 实例
	return db.Exec(c, args)       // 执行命令
}

func (d *Database) AfterClientClose(c resp.Connection) {
	logger.Info("EchoDatabase AfterClientClose")
}

func (d *Database) Close() {
	logger.Info("EchoDatabase Close")
}
