package database

import (
	"Godis/aof"
	"Godis/config"
	"Godis/interface/resp"
	"Godis/lib/logger"
	"Godis/resp/reply"
	"strconv"
	"strings"
)

// Database 存储所有 DB 实例
type Database struct {
	dbSet      []*DB           // 所有数据库实例
	aofHandler *aof.AofHandler // AOF 处理器的实例
}

// MakeDatabase 创建数据库, 默认创建 16 个 DB 实例
func NewDatabase() *Database {
	database := &Database{}
	// 若配置文件中未配置，使用默认参数
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
	logger.Info("config.Properties.AppendOnly:" + strconv.FormatBool(config.Properties.AppendOnly))
	// 根据配置文件判断是否要使用 aof
	if config.Properties.AppendOnly {
		aofHandler, err := aof.NewAofHandler(database)
		if err != nil {
			panic(err)
		}
		database.aofHandler = aofHandler
		// 为每个 DB 实例添加 aofHandler
		for _, db := range database.dbSet {
			currentDB := db
			currentDB.addAof = func(cmd CmdLine) {
				database.aofHandler.AddAof(currentDB.index, cmd)
			}
		}
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
		return d.execSelect(c, d, args[1:]) // args 从 1 开始传入，去掉最前面的 SELECT 命令名
	}
	db := d.dbSet[c.GetDBIndex()] // 获取 DB 实例
	return db.Exec(c, args)       // 执行命令
}

func (d *Database) AfterClientClose(c resp.Connection) {
	logger.Info("Database AfterClientClose")
}

func (d *Database) Close() {
	logger.Info("Database Close")
}
