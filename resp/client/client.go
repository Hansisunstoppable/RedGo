package client

import (
	"Godis/interface/resp"
	"Godis/lib/logger"
	"Godis/lib/sync/wait"
	"Godis/resp/parser"
	"Godis/resp/reply"
	"net"
	"runtime/debug"
	"sync"
	"time"
)

const (
	chanSize = 256
	maxWait  = 3 * time.Second
)

type Client struct {
	conn        net.Conn
	pendingReqs chan *request // 待发出的请求
	waitingReqs chan *request // 等待响应的请求
	ticker      *time.Ticker
	addr        string
	working     *sync.WaitGroup // 管理未处理完的请求（pending 和 waiting）
}

// request 客户端发出的每一条请求
type request struct {
	id        uint64
	args      [][]byte
	reply     resp.Reply
	heartbeat bool       // 是否为节点间相互验证的心跳命令
	waiting   *wait.Wait // 用于管理当前这条请求，是否阻塞时间过长已经超时
	err       error
}

// MakeClient 建立 TCP 连接，并初始化两个请求队列
func MakeClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr) // 连接到指定地址
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:        conn,
		pendingReqs: make(chan *request, chanSize), // 带缓冲的 channel
		waitingReqs: make(chan *request, chanSize), // 带缓冲的 channel
		addr:        addr,
		working:     &sync.WaitGroup{},
	}, nil
}

// Start 启动客户端，包含三个协程并发处理。全部是异步，互不干扰。这样不管请求阻塞还是服务器响应慢，都不会让客户端卡死。
func (client *Client) Start() {
	client.ticker = time.NewTicker(10 * time.Second)
	go client.handWrite() // 启动写协程，负责发请求
	go func() {
		err := client.handleRead() // 启动读协程，读取响应
		if err != nil {
			logger.Error(err)
		}
	}()
	go client.heartbeat() // 心跳协程，定时 ping
}

// Send 发送请求，将用户的请求写入到 pendingReqs
func (client *Client) Send(args [][]byte) resp.Reply {
	request := &request{
		args:      args,
		heartbeat: false,
		waiting:   &wait.Wait{},
	}
	request.waiting.Add(1)                              // 表示当前请求开始等待
	client.working.Add(1)                               // 当前客户端正在处理的请求 + 1
	defer client.working.Done()                         // 当前客户端正在处理的请求 - 1
	client.pendingReqs <- request                       // 请求写入 pendingReqs
	timeout := request.waiting.WaitWithTimeout(maxWait) // 判断当前请求是否已经超时
	if timeout {
		return reply.MakeStandardErrorReply("server time out") // 请求超过 maxWait 还未收到服务器响应，超时，返回错误
	}
	if request.err != nil {
		return reply.MakeStandardErrorReply("request failed")
	}
	return request.reply
}

// handleRead 读取并解析服务器响应
func (client *Client) handleRead() error {
	ch := parser.ParseStream(client.conn) // 连接里接收到的数据，进行异步解析后写入 ch
	for payload := range ch {
		if payload.Err != nil {
			client.finishRequest(reply.MakeStandardErrorReply(payload.Err.Error())) // 错误响应，也要处理
			continue
		}
		client.finishRequest(payload.Data) // 该请求已经取得响应，做最后的处理
	}
	return nil
}

// handWrite 从 pendingReqs 读出新请求，调用 doRequest 传递给服务器
func (client *Client) handWrite() {
	for req := range client.pendingReqs {
		client.doRequest(req)
	}
}

// heartbeat 定时执行 doHeartbeat
func (client *Client) heartbeat() {
	for range client.ticker.C {
		client.doHeartbeat()
	}
}

// handleConnectionError 重连建立 tcp 连接
func (client *Client) handleConnectionError(err error) error {
	err1 := client.conn.Close()
	if err1 != nil {
		if opErr, ok := err1.(*net.OpError); ok {
			if opErr.Err.Error() != "use of closed network connection" {
				return err1
			}
		} else {
			return err1
		}
	}
	conn, err1 := net.Dial("tcp", client.addr)
	if err1 != nil {
		logger.Error(err1)
		return err1
	}
	client.conn = conn
	go func() {
		_ = client.handleRead()
	}()
	return nil
}

// doRequest 将 pendingReqs 读出的请求序列化为 RESP 协议 格式，通过 tcp 发送给服务器
func (client *Client) doRequest(req *request) {
	if req == nil || len(req.args) == 0 {
		return
	}
	rep := reply.MakeMultiBulkReply(req.args)
	bytes := rep.ToBytes()             // 序列化为 RESP 协议格式
	_, err := client.conn.Write(bytes) // 发送请求

	// 如果发送失败，会尝试重连（最多 3 次），重连成功后继续发
	// 心跳请求也通过这里进行三次重连
	// 如果最终写还是失败，就直接把错误反馈回去
	i := 0
	for err != nil && i < 3 {
		err = client.handleConnectionError(err)
		if err == nil {
			_, err = client.conn.Write(bytes)
		}
		i++
	}
	if err == nil {
		client.waitingReqs <- req // 成功发送，将请求写入 waitingReqs（表示正在等待响应）
	} else {
		req.err = err
		req.waiting.Done()
	}
}

func (client *Client) finishRequest(reply resp.Reply) {
	defer func() {
		if err := recover(); err != nil {
			debug.PrintStack()
			logger.Error(err)
		}
	}()

	request := <-client.waitingReqs // 获取到等待响应的请求
	if request == nil {
		return
	}
	request.reply = reply // 将这条请求的响应写入
	if request.waiting != nil {
		request.waiting.Done() // 当前请求处理完成
	}
}

// doHeartbeat 发送心跳 PING 请求，判断连接是否可用，如果发现断开了，会自动重连
func (client *Client) doHeartbeat() {
	request := &request{
		args:      [][]byte{[]byte("PING")},
		heartbeat: true,
		waiting:   &wait.Wait{},
	}
	request.waiting.Add(1)
	client.working.Add(1) // 心跳请求也算作正在处理的请求
	defer client.working.Done()
	client.pendingReqs <- request
	request.waiting.WaitWithTimeout(3 * time.Second)
}

// Close 关闭连接
func (client *Client) Close() {
	client.ticker.Stop()
	close(client.pendingReqs) // 关掉发送队列，不让新的请求写入
	client.working.Wait()     // 所有请求都完成
	close(client.waitingReqs) // 再关掉等待响应队列
	_ = client.conn.Close()   // 最后关掉底层连接
}
