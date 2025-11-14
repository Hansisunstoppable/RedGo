package util

// ToCmdLine 把 strings 转换为 [][]byte
func ToCmdLine(cmd ...string) [][]byte {
	args := make([][]byte, len(cmd))
	for i, s := range cmd {
		args[i] = []byte(s)
	}
	return args
}

// ToCmdLineWithName 把  string(命令名) 与 args(参数) 转换为 [][]byte
func ToCmdLineWithName(cmdName string, args ...[]byte) [][]byte {
	cmd := make([][]byte, len(args)+1)
	cmd[0] = []byte(cmdName)
	for i, s := range args {
		cmd[i+1] = s
	}
	return cmd
}
