package database

import "strings"

// 存储所有命令

// cmdTable 命令表：命令名 与 命令结构体 的映射
var cmdTable = make(map[string]*command)

// command
type command struct {
	exec  ExecFunc // 命令对应的函数
	arity int      // 命令参数数量, 负数代表最少需要 abs(arity) 个参数, 命令本身也算一个参数
}

// RegisterCommand 注册新命令到 cmdTable
func RegisterCommand(name string, exec ExecFunc, arity int) {
	name = strings.ToLower(name)
	cmdTable[name] = &command{
		exec:  exec,
		arity: arity,
	}
}
