package cluster

import (
	"Godis/config"
	databaseinstance "Godis/database"
	"Godis/interface/database"
	"Godis/interface/resp"
	consistenthash "Godis/lib/consistent_hash"
	"Godis/lib/logger"
	"Godis/resp/reply"
	"context"
	"strings"

	pool "github.com/jolestar/go-commons-pool/v2"
)

// CmdFunc 类型，集群数据库的命令执行函数
type CmdFunc func(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply

var routerMap = makeRouter() // 转发路由

// ClusterDatabase 集群状态下的一个数据库结点
type ClusterDatabase struct {
	self       string                      // 当前节点的 ID
	nodes      []string                    // 集群中所有节点的 ID
	peerPicker *consistenthash.NodeMap     // 一致性哈希，用于为数据 key 选择结点
	peerConn   map[string]*pool.ObjectPool // 连接池，用来管理连接到其他节点的客户端
	db         database.Database           // 当前节点的数据库实例
}

// NewClusterDatabase 创建一个集群结点实例
func MakeClusterDatabase() *ClusterDatabase {
	cluster := &ClusterDatabase{
		self:       config.Properties.Self,                   // 配置文件获取自身节点的 ID
		db:         databaseinstance.NewStandaloneDatabase(), // 创建数据库实例
		peerPicker: consistenthash.NewNodeMap(nil),           // 创建一致性哈希, 采用默认哈希函数
		peerConn:   make(map[string]*pool.ObjectPool),        // 创建 节点名 与 连接池 映射 map
	}
	// 从配置文件中获取所有节点 ID
	nodes := make([]string, 0, len(config.Properties.Peers)+1)
	nodes = append(nodes, config.Properties.Peers...)
	nodes = append(nodes, config.Properties.Self)

	// 将所有节点添加到一致性哈希（哈希环）上
	cluster.peerPicker.AddNodes(nodes...)
	cluster.nodes = nodes

	// 为连接至每一个结点的 client（连接到自身的不需要） 都创建一个连接池，并与 peerId 映射
	ctx := context.Background()
	// 创建若干个连接池，每个连接池管理连接到不同 peer 的 client
	for _, peer := range config.Properties.Peers {
		cluster.peerConn[peer] = pool.NewObjectPoolWithDefaultConfig(ctx, &connectionFactory{Peer: peer})
	}
	return cluster
}

// Exec executes a command on the cluster database
func (c *ClusterDatabase) Exec(client resp.Connection, args [][]byte) (result resp.Reply) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("ClusterDatabase Exec panic:" + err.(error).Error())
			result = reply.MakeUnknownReply()
		}
	}()

	cmdName := strings.ToLower(string(args[0])) // 获取命令名，转换为小写字母
	if cmdFunc, ok := routerMap[cmdName]; ok {  // 根据命令名，获取对应的命令转发逻辑
		return cmdFunc(c, client, args) // 执行命令（不转发/转发/群发）
	} else {
		result = reply.MakeStandardErrorReply("ERR unknown command '" + cmdName + "'")
	}
	return
}

// Close closes the cluster database
func (c *ClusterDatabase) Close() {
	c.db.Close()
}

// AfterClientClose is called after a client closes
func (c *ClusterDatabase) AfterClientClose(client resp.Connection) {
	c.db.AfterClientClose(client)
}
