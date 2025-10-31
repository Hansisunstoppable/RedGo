package database

import "Godis/interface/resp"

// CmdLine 是 [][]byte 类型的别名,方便使用。表示 Redis 命令行
type CmdLine = [][]byte

// Database 数据库基本操作，向 Handler 提供统一的数据库操作方法
type Database interface {
	Exec(client resp.Connection, args CmdLine) resp.Reply // 执行 Redis 命令，响应通过客户端连接返回
	AfterClientClose(client resp.Connection)              // 客户端关闭连接后的处理
	Close()                                               // 关闭数据库
}

// DataEntity 将任意类型的数据，封装为 DataEntity
type DataEntity struct {
	Data interface{}
}
