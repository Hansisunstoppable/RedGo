package cluster

import (
	"Godis/interface/resp"
	"Godis/lib/util"
	"Godis/resp/client"
	"Godis/resp/reply"
	"context"
	"errors"
	"strconv"
)

// 集群结点间通信相关的方法
// 其实就是从连接池获取连接，以 客户端 服务端 的模式进行通信

// getPeerClient 根据结点 ID，从对应连接池中获取一个 client 连接对象
func (c *ClusterDatabase) getPeerClient(peer string) (*client.Client, error) {
	// 获取连接池
	pool, ok := c.peerConn[peer]
	if !ok {
		return nil, errors.New("peer not found")
	}

	// 从连接池“借用”对象
	conn, err := pool.BorrowObject(context.Background())
	if err != nil {
		return nil, err
	}
	client, ok := conn.(*client.Client) // 类型断言：连接池获取的对象一定为 client 类型
	if !ok {
		return nil, errors.New("invalid connection type")
	}
	return client, nil
}

// returnPeerClient 将 Client 对象归还对应的连接池
func (c *ClusterDatabase) returnPeerClient(peer string, client *client.Client) {
	pool, ok := c.peerConn[peer]
	if !ok {
		return
	}
	// 归还对象
	pool.ReturnObject(context.Background(), client)
}

// relayExec 将命令转发给指定结点 peer
func (c *ClusterDatabase) relayExec(peer string, conn resp.Connection, args [][]byte) resp.Reply {
	// 无需转发
	if peer == c.self {
		return c.db.Exec(conn, args)
	}

	client, err := c.getPeerClient(peer)
	if err != nil {
		return reply.MakeStandardErrorReply(err.Error())
	}
	defer func() {
		// 结束后归还连接
		c.returnPeerClient(peer, client)
	}()

	// 转发命令，先切换到对应的数据库实例, 再转发实际命令
	client.Send(util.ToCmdLime("SELECT", strconv.Itoa(conn.GetDBIndex())))
	return client.Send(args)
}

// broadcastExec 广播命令, 返回所有节点与其响应 互相映射的 map
func (c *ClusterDatabase) broadcastExec(conn resp.Connection, args [][]byte) map[string]resp.Reply {
	results := make(map[string]resp.Reply)
	for _, peer := range c.nodes {
		// 向每一个节点转发，获取响应
		result := c.relayExec(peer, conn, args)
		// 响应结果存入 map
		results[peer] = result
	}
	return results
}
