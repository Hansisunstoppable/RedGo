package reply

// reply.go 包含那些固定的 reply

// PongReply 客户端发送 PING 时固定回复 PONG
type PongReply struct {
}

func (r *PongReply) ToBytes() []byte {
	return []byte("+PONG\r\n")
}

// MakePongReply 创建一个 PONG 回复
// 工厂模式，将 pongReply 的构造函数隐藏起来
func MakePongReply() *PongReply {
	return &PongReply{}
}

// OKReply 客户端发送 SET 时固定回复 OK
type OKReply struct {
}

func (r *OKReply) ToBytes() []byte {
	return []byte("+OK\r\n")
}

func MakeOKReply() *OKReply {
	return &OKReply{}
}

// NullBulkReply 空的 Bulk 回复(字符串为 nil)
type NullBulkReply struct {
}

func (r *NullBulkReply) ToBytes() []byte {
	return []byte("$-1\r\n") // -1 代表 nil
}

func MakeNullBulkReply() *NullBulkReply {
	return &NullBulkReply{}
}

// EmptyBulkReply 空的 Bulk 回复(字符串为空)
type EmptyBulkReply struct {
}

func (r *EmptyBulkReply) ToBytes() []byte {
	return []byte("$0\r\n\r\n") // 0，表示空字符串
}

func MakeEmptyBulkReply() *NullBulkReply {
	return &NullBulkReply{}
}

// EmptyMultiBulkReply 空的 MultiBulk 回复(空数组)
type EmptyMultiBulkReply struct{}

func (r *EmptyMultiBulkReply) ToBytes() []byte {
	return []byte("*0\r\n")
}

func MakeEmptyMultiBulkReply() *EmptyMultiBulkReply {
	return &EmptyMultiBulkReply{}
}

// NoReply 无回复
type NoReply struct{}

func (r *NoReply) ToBytes() []byte {
	return []byte("")
}

func MakeNoReply() *NoReply {
	return &NoReply{}
}
