package database

// 实现 PING 命令

import (
	"Godis/interface/resp"
	"Godis/resp/reply"
)

// init 函数是 Go 语言中的一个特殊函数，它会在包被导入时自动执行
func init() {
	RegisterCommand("Ping", Ping, noPrepare, 1)
}

func Ping(db *DB, args [][]byte) resp.Reply {
	return reply.MakePongReply()
}
