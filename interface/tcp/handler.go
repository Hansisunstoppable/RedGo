package tcp

import (
	"context"
	"net"
)

// Handler 处理连接，优雅地关闭连接，处理客户端请求，返回响应等
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
	Close() error
}
