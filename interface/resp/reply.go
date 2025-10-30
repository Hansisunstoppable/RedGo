package resp

// Reply 所有回复都需要转化为字节切片，因此都需要实现以下方法
type Reply interface {
	ToBytes() []byte // 将回复转换为字节切片
}
