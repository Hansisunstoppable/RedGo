package connection

import (
	"Godis/lib/sync/wait"
	"net"
	"sync"
	"time"
)

// Connection 表示客户端和服务端的连接
type Connection struct {
	conn         net.Conn   // 底层的网络连接
	waitingReply wait.Wait  // 等待完成响应的同步器
	mu           sync.Mutex // 发送响应时的互斥锁
	selectedDB   int        // 选择的数据库的编号
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		conn: conn,
	}
}

// RemoteAddr 获取 客户端 地址
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Close 关闭连接
func (c *Connection) Close() error {
	// 超过超时时间 10 s 强制关闭连接，10 秒内未发送完，代码会被阻塞，不会执行下一行
	c.waitingReply.WaitWithTimeout(1000 * time.Second)
	_ = c.conn.Close()
	return nil
}

// Write 响应客户端（写入数据到 conn）
func (c *Connection) Write(bytes []byte) error {
	if len(bytes) == 0 {
		return nil
	}
	c.mu.Lock()           // 互斥访问
	c.waitingReply.Add(1) // 同步器加 1，添加一个正在写入的回复
	defer func() {
		c.mu.Unlock()
		c.waitingReply.Done()
	}()

	_, err := c.conn.Write(bytes)
	return err
}

// GetDBIndex 获取当前数据库编号
func (c *Connection) GetDBIndex() int {
	return c.selectedDB
}

// SelectDB 设置数据库
func (c *Connection) SelectDB(db int) {
	c.selectedDB = db
}
