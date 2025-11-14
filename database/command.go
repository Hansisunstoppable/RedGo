package database

import "strings"

// 存储所有命令

// cmdTable 命令表：命令名 与 命令结构体 的映射
var cmdTable = make(map[string]*command)

// command
type command struct {
	exec    ExecFunc // 命令对应的函数
	prepare PreFunc  // 命令预处理函数，返回要加锁的 key
	arity   int      // 命令参数数量, 负数代表最少需要 abs(arity) 个参数, 命令本身也算一个参数
}

// RegisterCommand 注册新命令到 cmdTable
func RegisterCommand(name string, exec ExecFunc, prepare PreFunc, arity int) {
	name = strings.ToLower(name)
	cmdTable[name] = &command{
		exec:    exec,
		prepare: prepare,
		arity:   arity,
	}
}
