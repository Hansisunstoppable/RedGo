package cluster

import (
	"Godis/datastruct/set"
	"Godis/interface/resp"
	"Godis/resp/reply"
)

// 不同的命令，采用不同的转发逻辑。这就是路由

func makeRouter() map[string]CmdFunc {
	routerMap := make(map[string]CmdFunc) // 不同的命令用不同的转发逻辑

	// KEYS and STRINGS operations
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

	// List operations
	routerMap["lpush"] = defaultFunc
	routerMap["rpush"] = defaultFunc
	routerMap["lpop"] = defaultFunc
	routerMap["rpop"] = defaultFunc
	routerMap["lrange"] = defaultFunc
	routerMap["llen"] = defaultFunc
	routerMap["lindex"] = defaultFunc
	routerMap["lset"] = defaultFunc

	// Hash operations
	routerMap["hset"] = defaultFunc       // hset key field value
	routerMap["hsetnx"] = defaultFunc     // hsetnx key field value
	routerMap["hget"] = defaultFunc       // hget key field
	routerMap["hexists"] = defaultFunc    // hexists key field
	routerMap["hdel"] = defaultFunc       // hdel key field [field ...]
	routerMap["hlen"] = defaultFunc       // hlen key
	routerMap["hgetall"] = defaultFunc    // hgetall key
	routerMap["hkeys"] = defaultFunc      // hkeys key
	routerMap["hvals"] = defaultFunc      // hvals key
	routerMap["hmget"] = defaultFunc      // hmget key field [field ...]
	routerMap["hmset"] = defaultFunc      // hmset key field value [field value ...]
	routerMap["hrandfield"] = defaultFunc // hrandfield key [count]
	routerMap["hencoding"] = defaultFunc  // hencoding key (custom command)

	// set operation
	// 单 key 操作，只涉及到转发到一个服务器
	routerMap["sadd"] = defaultFunc        // sadd key member [member ...]
	routerMap["scard"] = defaultFunc       // scard key
	routerMap["sismember"] = defaultFunc   // sismember key member
	routerMap["smembers"] = defaultFunc    // smembers key
	routerMap["srem"] = defaultFunc        // srem key member [member ...]
	routerMap["spop"] = defaultFunc        // spop key [count]
	routerMap["srandmember"] = defaultFunc // srandmember key [count]

	// 多 key 操作，多个 key 可能分布在不同服务器上
	routerMap["sunion"] = setUnionFunc               // sunion key [key ...]
	routerMap["sunionstore"] = setUnionStoreFunc     // sunionstore destination key [key ...]
	routerMap["sinter"] = setIntersectFunc           // sinter key [key ...]
	routerMap["sinterstore"] = setIntersectStoreFunc // sinterstore destination key [key ...]
	routerMap["sdiff"] = setDiffFunc                 // sdiff key [key ...]
	routerMap["sdiffstore"] = setDiffStoreFunc       // sdiffstore destination key [key ...]

	// ZSet operations
	routerMap["zadd"] = defaultFunc   // zadd key score member [score member ...]
	routerMap["zscore"] = defaultFunc // zscore key member
	routerMap["zcard"] = defaultFunc  // zcard key
	routerMap["zrange"] = defaultFunc // zrange key start stop [WITHSCORES]
	routerMap["zrem"] = defaultFunc   // zrem key member [member ...]
	routerMap["zcount"] = defaultFunc // zcount key min max
	routerMap["zrank"] = defaultFunc  // zrank key member

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

// setUnionFunc 在集群模式下处理 SUNION 命令
func setUnionFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 2 {
		return reply.MakeArgNumErrReply("sunion")
	}

	// 创建一个集合来保存并集结果
	result := set.NewHashSet()

	// 单独处理每个键
	for i := 1; i < len(args); i++ {
		key := string(args[i])
		peer := cluster.peerPicker.PickNode(key)

		// 为此键创建 SMEMBERS 命令
		smembersArgs := make([][]byte, 2)
		smembersArgs[0] = []byte("SMEMBERS")
		smembersArgs[1] = args[i]

		// 在适当的节点上执行 SMEMBERS
		nodeReply := cluster.relayExec(peer, conn, smembersArgs)

		// 处理回复
		if mbReply, ok := nodeReply.(*reply.MultiBulkReply); ok {
			// 将每个成员添加到我们的结果集合中
			for _, member := range mbReply.Args {
				result.Add(string(member))
			}
		} else if reply.IsError(nodeReply) {
			return nodeReply // 转发任何错误
		}
	}

	// 将结果集合转换为 [][]byte 格式用于响应
	members := result.Members()
	resultBytes := make([][]byte, len(members))
	for i, member := range members {
		resultBytes[i] = []byte(member)
	}

	return reply.MakeMultiBulkReply(resultBytes)
}

// setUnionStoreFunc 在集群模式下处理 SUNIONSTORE 命令
// 使用 setUnionFunc 获取并集，然后将其存储到目标键中
func setUnionStoreFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 3 {
		return reply.MakeArgNumErrReply("sunionstore")
	}
	// 获取最终要存储的 result 集合对应的节点
	destKey := string(args[1])
	destPeer := cluster.peerPicker.PickNode(destKey)

	// 获取后续集合的并集
	sourceArgs := make([][]byte, len(args)-1)
	sourceArgs[0] = []byte("SUNION")
	copy(sourceArgs[1:], args[2:])
	// 使用上面的 SUNION 函数获取并集
	unionReply := setUnionFunc(cluster, conn, sourceArgs)
	if mbReply, ok := unionReply.(*reply.MultiBulkReply); ok {
		// 如果目标结点上已经有 result 集合，就先删掉
		delArgs := make([][]byte, 2)
		delArgs[0] = []byte("DEL")
		delArgs[1] = args[1]
		cluster.relayExec(destPeer, conn, delArgs)
		if len(mbReply.Args) > 0 {
			// 在目标节点上创建一个新集合
			storeArgs := make([][]byte, len(mbReply.Args)+2)
			storeArgs[0] = []byte("SADD")
			storeArgs[1] = args[1]
			copy(storeArgs[2:], mbReply.Args)
			reply := cluster.relayExec(destPeer, conn, storeArgs)
			return reply
		}
		// 如果并集为空，返回 0
		return reply.MakeIntReply(0)
	}
	// 返回错误
	return unionReply
}

// setIntersectFunc 在集群模式下处理 SINTER 命令
// 单独处理每个键并计算交集
func setIntersectFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 2 {
		return reply.MakeArgNumErrReply("sinter")
	}
	// 如果只有一个键，直接返回其成员
	if len(args) == 2 {
		key := string(args[1])
		peer := cluster.peerPicker.PickNode(key)
		// 为该键创建 SMEMBERS 命令
		smembersArgs := make([][]byte, 2)
		smembersArgs[0] = []byte("SMEMBERS")
		smembersArgs[1] = args[1]
		return cluster.relayExec(peer, conn, smembersArgs)
	}
	// 存储从每个键获取的集合成员
	var allSets []map[string]bool
	// 单独处理每个键
	for i := 1; i < len(args); i++ {
		key := string(args[i])
		peer := cluster.peerPicker.PickNode(key)
		// 为该键创建 SMEMBERS 命令
		smembersArgs := make([][]byte, 2)
		smembersArgs[0] = []byte("SMEMBERS")
		smembersArgs[1] = args[i]
		// 在相应的节点上执行 SMEMBERS 命令
		nodeReply := cluster.relayExec(peer, conn, smembersArgs)
		if mbReply, ok := nodeReply.(*reply.MultiBulkReply); ok {
			// 将响应转换为集合以便求交集
			memberSet := make(map[string]bool)
			for _, member := range mbReply.Args {
				memberSet[string(member)] = true
			}
			// 如果任意集合为空，则交集为空
			if len(memberSet) == 0 {
				return reply.MakeMultiBulkReply([][]byte{})
			}
			allSets = append(allSets, memberSet)
		} else if reply.IsError(nodeReply) {
			return nodeReply
		}
	}
	// 如果没有获取到任何集合，返回空结果
	if len(allSets) == 0 {
		return reply.MakeMultiBulkReply([][]byte{})
	}
	// 计算交集
	result := make(map[string]bool)
	// 使用第一个集合的所有元素初始化结果
	for member := range allSets[0] {
		result[member] = true
	}
	// 与后续集合取交集
	for i := 1; i < len(allSets); i++ {
		nextSet := allSets[i]
		for member := range result {
			if !nextSet[member] {
				delete(result, member)
			}
		}
		// 如果交集已为空，提前返回
		if len(result) == 0 {
			break
		}
	}
	// 将结果转换为响应格式
	members := make([][]byte, 0, len(result))
	for member := range result {
		members = append(members, []byte(member))
	}
	return reply.MakeMultiBulkReply(members)
}

// setIntersectStoreFunc 在集群模式下处理 SINTERSTORE 命令
// 使用 setIntersectFunc 获取交集，然后将其存储到目标键中
func setIntersectStoreFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 3 {
		return reply.MakeArgNumErrReply("sinterstore")
	}
	// 获取目标键及其所在节点
	destKey := string(args[1])
	destPeer := cluster.peerPicker.PickNode(destKey)
	// 获取源集合的交集
	sourceArgs := make([][]byte, len(args)-1)
	sourceArgs[0] = []byte("SINTER")
	copy(sourceArgs[1:], args[2:])
	// 使用 setIntersectFunc 获取交集
	intersectReply := setIntersectFunc(cluster, conn, sourceArgs)
	if mbReply, ok := intersectReply.(*reply.MultiBulkReply); ok {
		// 首先删除目标键（如果存在）
		delArgs := make([][]byte, 2)
		delArgs[0] = []byte("DEL")
		delArgs[1] = args[1]
		cluster.relayExec(destPeer, conn, delArgs)
		if len(mbReply.Args) > 0 {
			// 在目标节点上创建新集合
			storeArgs := make([][]byte, len(mbReply.Args)+2)
			storeArgs[0] = []byte("SADD")
			storeArgs[1] = args[1]
			copy(storeArgs[2:], mbReply.Args)
			rep := cluster.relayExec(destPeer, conn, storeArgs)
			// 对于 SINTERSTORE，需要返回结果的基数（元素个数）
			if intReply, ok := rep.(*reply.IntReply); ok {
				return reply.MakeIntReply(intReply.Code)
			}
			return rep
		}
		// 如果交集为空，返回 0
		return reply.MakeIntReply(0)
	}
	// 如果无法获取交集，则返回错误
	return intersectReply
}

// setDiffFunc 在集群模式下处理 SDIFF 命令
// 获取第一个集合的所有成员，然后移除在其他集合中出现的成员
func setDiffFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 2 {
		return reply.MakeArgNumErrReply("sdiff")
	}
	// 获取第一个集合（基础集合）
	firstKey := string(args[1])
	firstPeer := cluster.peerPicker.PickNode(firstKey)
	// 为第一个键创建 SMEMBERS 命令
	smembersArgs := make([][]byte, 2)
	smembersArgs[0] = []byte("SMEMBERS")
	smembersArgs[1] = args[1]
	firstSetReply := cluster.relayExec(firstPeer, conn, smembersArgs)
	if !reply.IsMultiBulkReply(firstSetReply) {
		if reply.IsError(firstSetReply) {
			return firstSetReply
		}
		return reply.MakeMultiBulkReply([][]byte{})
	}
	// 将第一个集合的成员添加到结果集合中
	firstSetMembers := firstSetReply.(*reply.MultiBulkReply)
	result := make(map[string]bool)
	for _, member := range firstSetMembers.Args {
		result[string(member)] = true
	}
	// 如果只有一个集合，直接返回其所有成员
	if len(args) == 2 {
		return firstSetReply
	}
	// 从结果集合中移除其他集合的成员
	for i := 2; i < len(args); i++ {
		key := string(args[i])
		peer := cluster.peerPicker.PickNode(key)
		// 为该键创建 SMEMBERS 命令
		smembersArgs := make([][]byte, 2)
		smembersArgs[0] = []byte("SMEMBERS")
		smembersArgs[1] = args[i]
		nodeReply := cluster.relayExec(peer, conn, smembersArgs)
		if mbReply, ok := nodeReply.(*reply.MultiBulkReply); ok {
			// 从结果集合中移除该集合的成员
			for _, member := range mbReply.Args {
				delete(result, string(member))
			}
		} else if reply.IsError(nodeReply) {
			return nodeReply
		}
		// 如果差集已为空，提前返回
		if len(result) == 0 {
			break
		}
	}
	// 将结果转换为响应格式
	members := make([][]byte, 0, len(result))
	for member := range result {
		members = append(members, []byte(member))
	}
	return reply.MakeMultiBulkReply(members)
}

// setDiffStoreFunc 在集群模式下处理 SDIFFSTORE 命令
// 使用 setDiffFunc 获取差集，然后将其存储到目标键中
func setDiffStoreFunc(cluster *ClusterDatabase, conn resp.Connection, args [][]byte) resp.Reply {
	if len(args) < 3 {
		return reply.MakeArgNumErrReply("sdiffstore")
	}
	// 获取目标键及其所在节点
	destKey := string(args[1])
	destPeer := cluster.peerPicker.PickNode(destKey)
	// 获取源集合的差集
	sourceArgs := make([][]byte, len(args)-1)
	sourceArgs[0] = []byte("SDIFF")
	copy(sourceArgs[1:], args[2:])
	// 使用 setDiffFunc 获取差集
	diffReply := setDiffFunc(cluster, conn, sourceArgs)
	if mbReply, ok := diffReply.(*reply.MultiBulkReply); ok {
		// 首先删除目标键（如果存在）
		delArgs := make([][]byte, 2)
		delArgs[0] = []byte("DEL")
		delArgs[1] = args[1]
		cluster.relayExec(destPeer, conn, delArgs)
		if len(mbReply.Args) > 0 {
			// 在目标节点上创建新集合
			storeArgs := make([][]byte, len(mbReply.Args)+2)
			storeArgs[0] = []byte("SADD")
			storeArgs[1] = args[1]
			copy(storeArgs[2:], mbReply.Args)
			rep := cluster.relayExec(destPeer, conn, storeArgs)
			// 对于 SDIFFSTORE，需要返回结果的基数（元素个数）
			if intReply, ok := rep.(*reply.IntReply); ok {
				return reply.MakeIntReply(intReply.Code)
			}
			return rep
		}
		// 如果差集为空，返回 0
		return reply.MakeIntReply(0)
	}
	// 如果无法获取差集，则返回错误
	return diffReply
}
