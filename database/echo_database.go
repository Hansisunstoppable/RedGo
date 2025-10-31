package database

import (
	"Godis/interface/resp"
	"Godis/lib/logger"
	"Godis/resp/reply"
)

// 回显类数据库
type EchoDatabase struct{}

func NewEchoDatabase() *EchoDatabase {
	return &EchoDatabase{}
}

// Exec 直接 echo 原样回显
func (e *EchoDatabase) Exec(c resp.Connection, args [][]byte) resp.Reply {
	return reply.MakeMultiBulkReply(args)
}

func (e *EchoDatabase) AfterClientClose(c resp.Connection) {
	logger.Info("EchoDatabase AfterClientClose")
}

func (e *EchoDatabase) Close() {
	logger.Info("EchoDatabase Close")
}
