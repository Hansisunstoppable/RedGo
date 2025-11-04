package cluster

import (
	"Godis/interface/resp"
	"Godis/resp/reply"
)

// 不同的命令，采用不同的转发逻辑。这就是路由

func makeRouter() map[string]CmdFunc {
	routerMap := make(map[string]CmdFunc) // 不同的命令用不同的转发逻辑

	// KEYS 命令 与 STRINGS 命令的路由
	routerMap["exists"] = defaultFunc
	routerMap["type"] = defaultFunc
	routerMap["set"] = defaultFunc
	routerMap["get"] = defaultFunc
	routerMap["setnx"] = defaultFunc
	routerMap["getset"] = defaultFunc

	routerMap["ping"] = pingFunc
	routerMap["select"] = selectFunc

	routerMap["rename"] = renameFunc
	routerMap["renamex"] = renameFunc

	routerMap["del"] = delFunc

	routerMap["flushdb"] = flushDBFunc

	// LIST 命令的路由
	routerMap["lpush"] = defaultFunc
	routerMap["rpush"] = defaultFunc
	routerMap["lpop"] = defaultFunc
	routerMap["rpop"] = defaultFunc
	routerMap["lrange"] = defaultFunc
	routerMap["llen"] = defaultFunc
	routerMap["lindex"] = defaultFunc
	routerMap["lset"] = defaultFunc

	return routerMap
}

// defaultFunc 默认路由转发函数，根据 agrs[1],即 SET key 等命令的 key 参数, 确认要转发的结点
func defaultFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	key := string(args[1])                   // args[1] 对应的是 SET key 等命令的 key 参数
	peer := cluster.peerPicker.PickNode(key) // 一致性哈希确认转发节点
	return cluster.relayExec(peer, conn, args)
}

// pingFunc PING 命令转发逻辑，即无需转发
func pingFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	return cluster.db.Exec(conn, args)
}

// selectFunc SELECT 命令转发逻辑，即无需转发
func selectFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	return cluster.db.Exec(conn, args)
}

// renameFunc 跨界点 RENAME 操作无法保证原子性，发现跨节点直接返回错误
func renameFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) != 3 {
		return reply.MakeStandardErrorReply("ERR wrong number of arguments for 'rename' command")
	}
	src := string(args[1])
	dest := string(args[2])

	srcPeer := cluster.peerPicker.PickNode(src)
	destPeer := cluster.peerPicker.PickNode(dest)

	// key1 与 key2 在不同的节点，返回错误
	if srcPeer != destPeer {
		return reply.MakeStandardErrorReply("ERR source and destination keys are on different nodes")
	}
	// 执行 RENAME
	return cluster.relayExec(srcPeer, conn, args)
}

// flushDBFunc 判断所有节点的响应中是否有 err。有 err 则返回 err; 否则返回 ok
func flushDBFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	replies := cluster.broadcastExec(conn, args) // 广播转发命令

	// 遍历所有节点的响应，如果有错误，则返回错误
	var errReply reply.ErrorReply
	for _, r := range replies {
		if reply.IsError(r) {
			errReply = r.(reply.ErrorReply)
			break
		}
	}
	// 没错误，返回 ok
	if errReply == nil {
		return reply.MakeOKReply()
	}
	return reply.MakeStandardErrorReply("error: " + errReply.Error())
}

// delFunc 由于 DEL 命令可能一次删除多个 key，
func delFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 2 {
		return reply.MakeArgNumErrReply("del")
	}

	// 如果只需要删除一个 key，则直接转发到对应结点
	if len(args) == 2 {
		key := string(args[1])
		peer := cluster.peerPicker.PickNode(key)
		// TODO 能否直接写 return cluster.relayExec(peer, conn, args) ？
		fullArgs := make([][]byte, 2)
		fullArgs[0] = []byte("DEL")
		fullArgs[1] = args[1]
		return cluster.relayExec(peer, conn, fullArgs)
	}

	// --- 删除开始 ---
	// 处理多个键：按节点来对 key 分组
	groupedKeys := make(map[string][][]byte) // 键：节点地址，值：该节点处理的键列表
	for i := 1; i < len(args); i++ {         // 从索引 1 开始遍历所有要删除的键，索引 0 是 DEL 命令
		key := string(args[i])
		peer := cluster.peerPicker.PickNode(key)
		if _, ok := groupedKeys[peer]; !ok {
			groupedKeys[peer] = make([][]byte, 0)
		}
		groupedKeys[peer] = append(groupedKeys[peer], args[i]) // 将原始的 []byte 键添加到列表中
	}

	// 对每个节点执行删除操作
	var deleted int64 = 0
	var firstErrReply reply.ErrorReply // 保存遇到的第一个错误

	for peer, keys := range groupedKeys {
		// 为当前节点构造 DEL 命令：["DEL", key1, key2, ...]，后续传入执行
		nodeArgs := make([][]byte, len(keys)+1)
		nodeArgs[0] = []byte("DEL") // 命令本身
		copy(nodeArgs[1:], keys)    // 复制该节点处理的键列表

		// 向指定节点转发 DEL 命令
		nodeReply := cluster.relayExec(peer, conn, nodeArgs)

		// 处理响应
		if reply.IsError(nodeReply) {
			// 如果是错误响应，记录第一个错误并停止处理其他节点（可选，也可以选择继续处理其他节点）
			if firstErrReply == nil {
				if errReply, ok := nodeReply.(reply.ErrorReply); ok {
					firstErrReply = errReply
				} else {
					firstErrReply = reply.MakeStandardErrorReply("unknown error from peer")
				}
			}
			// 这里可以选择 break 或 continue，取决于你希望一个节点失败时整个操作是否立即失败
			// break // 如果一个节点失败则停止并返回错误
			continue // 继续尝试删除其他节点上的键，最后汇总结果或返回第一个错误
		}

		// 如果是整数响应，累加已删除的键数量
		if intReply, ok := nodeReply.(*reply.IntReply); ok {
			deleted += intReply.Code
		} else {
			// 如果响应既不是预期的整数也不是错误，则视为错误
			if firstErrReply == nil {
				firstErrReply = reply.MakeStandardErrorReply("unexpected reply type from peer")
			}
			continue
		}
	}

	// 如果处理过程中遇到错误，返回第一个错误
	if firstErrReply != nil {
		// 可以选择返回更详细的错误信息或仅返回第一个错误
		return reply.MakeStandardErrorReply("error occurs during multi-key delete: " + firstErrReply.Error())
	}

	// 如果所有节点都成功（或部分错误被忽略），返回总共删除的键数量
	return reply.MakeIntReply(deleted)
}
