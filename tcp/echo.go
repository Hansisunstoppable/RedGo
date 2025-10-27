package tcp

import (
	"Godis/lib/logger"
	"Godis/lib/sync/atomic"
	"Godis/lib/sync/wait"
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"time"
)

/*
	EchoHandler：统一管理多个 EchoClient 连接实例，标记关闭状态
	EchoClient：接收客户端发送的消息，然后返回信息给客户端
*/

// EchoHandler 管理多个连接实例，实现 Handler 接口
type EchoHandler struct {
	activeConn sync.Map       // 存储所有客户端连接
	closing    atomic.Boolean // 标记服务器是否 正在关闭，正在关闭时不接受新的连接
}

// MakeHandler 工厂模式 creates EchoHandler
func MakeHandler() *EchoHandler {
	return &EchoHandler{}
}

// EchoClient 单个连接实例，Waiting 用于控制超时
type EchoClient struct {
	Conn    net.Conn
	Waiting wait.Wait
}

// Close 关闭单个客户端连接
func (c *EchoClient) Close() error {
	c.Waiting.WaitWithTimeout(10 * time.Second) // WaitGroup 实现超时，超时后强制 return ，继续执行下面的代码
	err := c.Conn.Close()                       // 最多 10 秒，一定会执行到这
	if err != nil {
		return err
	}
	return nil
}

// Handle echos received line to client
func (h *EchoHandler) Handle(ctx context.Context, conn net.Conn) {
	// 如何客户端 正在关闭
	if h.closing.Get() {
		// 拒绝新的连接
		_ = conn.Close()
	}

	// 新的客户端连接
	client := &EchoClient{
		Conn: conn,
	}
	// 在 EchoHandler 中保存客户端连接
	h.activeConn.Store(client, struct{}{})

	reader := bufio.NewReader(conn)
	// 循环接收客户端发送的消息
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// 已经在上面 close 了，这里记录日志即可
				logger.Info("connection closed: ")
				h.activeConn.Delete(client)
			} else {
				logger.Error(err)
			}
			return
		}

		// 该客户端正在处理的消息数量加一(waitGroup 的灵活使用，不一定是控制协程，这里就是控制未处理的信息)
		client.Waiting.Add(1)
		b := []byte(msg)
		// 将接收到的消息原样返回客户端
		_, _ = conn.Write(b)
		// 该客户端正在处理的消息数量减一
		client.Waiting.Done()
	}
}

// Close stops echo handler, 关闭所有客户端连接
func (h *EchoHandler) Close() error {
	logger.Info("handler is closing...")
	h.closing.Set(true)
	h.activeConn.Range(func(key interface{}, val interface{}) bool {
		client := key.(*EchoClient)
		_ = client.Close() // 关闭单个连接实例
		return true
	})
	return nil
}
