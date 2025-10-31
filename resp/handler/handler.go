package handler

import (
	"Godis/database"
	databaseface "Godis/interface/database"
	"Godis/lib/logger"
	"Godis/lib/sync/atomic"
	"Godis/resp/connection"
	"Godis/resp/parser"
	"Godis/resp/reply"
	"context"
	"io"
	"net"
	"strings"
	"sync"
)

/*

 */

var (
	unknownErrReplyBytes = []byte("-ERR unknown\r\n")
)

// RespHandler implements tcp.Handler and serves as a redis handler
type RespHandler struct {
	activeConn sync.Map              // 存储所有客户端连接
	db         databaseface.Database // 连接的数据库
	closing    atomic.Boolean        // 标记服务器是否 正在关闭，正在关闭时不接受新的连接
}

// MakeRespHandler 创建一个 RespHandler 实例
func MakeRespHandler() *RespHandler {
	var db databaseface.Database
	db = database.NewDatabase()
	return &RespHandler{
		db: db,
	}
}

func (h *RespHandler) Handle(ctx context.Context, conn net.Conn) {
	if h.closing.Get() {
		_ = conn.Close()
	}

	client := connection.NewConnection(conn)
	h.activeConn.Store(client, 1)

	ch := parser.ParseStream(conn)
	// 轮询读取解析出来的的数据
	for payload := range ch {
		// fmt.Println("payload:", payload)
		if payload.Err != nil {
			if payload.Err == io.EOF || // io.Reader 已经读到了数据的末尾, 客户端正常关闭了连接
				payload.Err == io.ErrUnexpectedEOF || // io.Reader 已经读到了数据的末尾, 但解析器好气哦王
				strings.Contains(payload.Err.Error(), "use of closed network connection") {
				// connection closed
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			// protocol err
			errReply := reply.MakeStandardErrorReply(payload.Err.Error())
			err := client.Write(errReply.ToBytes())
			if err != nil {
				h.closeClient(client)
				logger.Info("connection closed: " + client.RemoteAddr().String())
				return
			}
			continue
		}

		if payload.Data == nil {
			logger.Error("empty payload")
			continue
		}
		r, ok := payload.Data.(*reply.MultiBulkReply) // 类型断言
		if !ok {
			logger.Error("require multi bulk protocol")
			continue
		}
		result := h.db.Exec(client, r.Args)
		if result != nil {
			_ = client.Write(result.ToBytes())
		} else {
			_ = client.Write(unknownErrReplyBytes)
		}
	}
}

// Close 关掉整个连接处理器（要关掉所有连接实例）
func (h *RespHandler) Close() error {
	logger.Info("handler is closing...")
	h.closing.Set(true)
	// TODO concurrent wait
	h.activeConn.Range(func(key interface{}, val interface{}) bool {
		client := key.(*connection.Connection)
		h.closeClient(client)
		return true
	})
	h.db.Close()
	return nil
}

// CloseClient 删除单个连接实例
func (h *RespHandler) closeClient(client *connection.Connection) {
	_ = client.Close()
	h.activeConn.Delete(client)   // 从 map 删除连接
	h.db.AfterClientClose(client) // 访问数据库的客户端关闭后的处理
}
