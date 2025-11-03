package reply

import (
	"Godis/interface/resp"
	"bytes"
	"strconv"
)

var (
	nullBUlkReplyBytes = []byte("$-1") // -1，表示 nil 值
	CRLF               = "\r\n"
)

// ErrorReply 错误回复，实现了 Reply 的 ToBytes 方法，也实现了系统的 error 接口
// 使用了接口组合，将 error 接口和 Reply 接口组合在一起
type ErrorReply interface {
	Error() string
	ToBytes() []byte
}

// BulkReply 字符串回复
type BulkReply struct {
	Arg []byte // 回复内容
}

func (r *BulkReply) ToBytes() []byte {
	// 若回复内容为空，则返回空字符串
	if r.Arg == nil {
		return nullBUlkReplyBytes
	}
	// 将 BulkReply 转换为符合 RESP 协议的字节数组, strconv.Itoa 将十进制整数转化为字符串
	return []byte("$" + strconv.Itoa(len(r.Arg)) + CRLF + string(r.Arg) + CRLF)
}
func MakeBulkReply(arg []byte) *BulkReply {
	return &BulkReply{Arg: arg}
}

// MultiBulkReply 多个字符串回复
type MultiBulkReply struct {
	Args [][]byte
}

// ToBytes 遍历数组，将每个字符串都转为 RESP 字节切片，然后拼接
func (r *MultiBulkReply) ToBytes() []byte {
	argLen := len(r.Args)
	var buf bytes.Buffer                               // 临时存放字节切片
	buf.WriteString("*" + strconv.Itoa(argLen) + CRLF) // 数组开头为 *
	for _, arg := range r.Args {
		if arg == nil {
			buf.WriteString(string(nullBUlkReplyBytes) + CRLF)
		} else {
			buf.WriteString("$" + strconv.Itoa(len(arg)) + CRLF + string(arg) + CRLF)
		}
	}
	return buf.Bytes()
}

func MakeMultiBulkReply(args [][]byte) *MultiBulkReply {
	return &MultiBulkReply{Args: args}
}

// StatusReply 状态回复
type StatusReply struct {
	Status string
}

func (r *StatusReply) ToBytes() []byte {
	return []byte("+" + r.Status + CRLF)
}

func MakeStatusReply(status string) *StatusReply {
	return &StatusReply{Status: status}
}

// StandardErrorReply 状态回复(通用错误回复)
type StandardErrorReply struct {
	Status string
}

func (r *StandardErrorReply) ToBytes() []byte {
	return []byte("-" + r.Status + CRLF)
}

func (r *StandardErrorReply) Error() string {
	return r.Status
}

func MakeStandardErrorReply(status string) *StandardErrorReply {
	return &StandardErrorReply{Status: status}
}

// IntReply 整数回复
type IntReply struct {
	Code int64
}

func (r *IntReply) ToBytes() []byte {
	return []byte(":" + strconv.FormatInt(r.Code, 10) + CRLF)
}

func MakeIntReply(code int64) *IntReply {
	return &IntReply{Code: code}
}

// IsError 如果第一个字符是 -，则表示是错误回复
func IsError(reply resp.Reply) bool {
	return reply.ToBytes()[0] == '-'
}

// IsMultiBulkReply 如果第一个字符是 *，则表示是错误回复是“多条回复”
func IsMultiBulkReply(reply resp.Reply) bool {
	return reply.ToBytes()[0] == '*'
}
