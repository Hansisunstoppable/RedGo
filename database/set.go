package database

import (
	"Godis/datastruct/set"
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/lib/util"
	"Godis/resp/reply"
	"strconv"
)

func init() {
	RegisterCommand("SADD", execSAdd, -3)
	RegisterCommand("SCARD", execSCard, 2)
	RegisterCommand("SISMEMBER", execSIsMember, 3)
	RegisterCommand("SMEMBERS", execSMembers, 2)
	RegisterCommand("SREM", execSRem, -3)
	RegisterCommand("SPOP", execSPop, -2)
	RegisterCommand("SRANDMEMBER", execSRandMember, -2)
	RegisterCommand("SUNION", execSUnion, -2)
	RegisterCommand("SUNIONSTORE", execSUnionStore, -3)
	RegisterCommand("SINTER", execSInter, -2)
	RegisterCommand("SINTERSTORE", execSInterStore, -3)
	RegisterCommand("SDIFF", execSDiff, -2)
	RegisterCommand("SDIFFSTORE", execSDiffStore, -3)
}

// execSAdd 实现 SADD key member1 member2...
func execSAdd(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	members := args[1:]

	// 根据 key 获取或创建集合
	setObj, isNew, errReply := getOrInitSet(db, key)
	if errReply != nil {
		return errReply
	}

	// 加入集合
	count := 0
	for _, member := range members {
		count += setObj.Add(string(member))
	}

	// 将更新后的 set 放回底层数据库
	if isNew || count > 0 {
		db.PutEntity(key, &database.DataEntity{
			Data: setObj,
		})

		// Add to AOF
		db.addAof(util.ToCmdLineWithName("SADD", args...))
	}

	return reply.MakeIntReply(int64(count))
}

// execSCard 返回集合内元素数量
func execSCard(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 到底层数据库获取 set 实例
	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil {
		return reply.MakeIntReply(0)
	}
	return reply.MakeIntReply(int64(setObj.Len())) // 返回集合大小
}

// execSIsMember 判断 member 是否在集合中
func execSIsMember(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	member := string(args[1])

	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil {
		return reply.MakeIntReply(0)
	}
	if setObj.Contains(member) {
		return reply.MakeIntReply(1)
	}
	return reply.MakeIntReply(0)
}

func execSMembers(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil {
		return reply.MakeMultiBulkReply([][]byte{})
	}

	// 以字节切片形式返回
	members := setObj.Members()
	result := make([][]byte, len(members))
	for i, member := range members {
		result[i] = []byte(member)
	}

	return reply.MakeMultiBulkReply(result)
}

func execSRem(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	members := args[1:] // 可能删除多个元素，获取列表

	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil {
		return reply.MakeIntReply(0)
	}

	count := 0 // 统计删除了多少条，用于返回
	for _, member := range members {
		count += setObj.Remove(string(member))
	}

	if count > 0 {
		// 判断 set 集合是否为空。若为空，则把 set 删掉
		if setObj.Len() == 0 {
			db.Remove(key)
		} else { // 将更新后的 set 放回底层数据库
			db.PutEntity(key, &database.DataEntity{
				Data: setObj,
			})
		}
		// 写入 aof 文件
		db.addAof(util.ToCmdLineWithName("SREM", args...))
	}
	return reply.MakeIntReply(int64(count))
}

// execSPop 随机弹出一个元素，并返回
func execSPop(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	// 获取要删除的元素数量
	count := 1
	if len(args) >= 2 {
		var err error
		count, err = strconv.Atoi(string(args[1]))
		if err != nil || count < 0 {
			return reply.MakeStandardErrorReply("ERR value is out of range, must be positive")
		}
	}

	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil || setObj.Len() == 0 {
		return reply.MakeNullBulkReply()
	}

	if count == 0 {
		return reply.MakeMultiBulkReply([][]byte{})
	}

	// 防止 count 大于集合元素总数
	if count > setObj.Len() {
		count = setObj.Len()
	}

	// 随机获取 count 个待删除元素
	members := setObj.RandomDistinctMembers(count) // TODO

	// 删除这些元素
	for _, member := range members {
		setObj.Remove(member)
	}

	// 更新底层 kv 内的 set 或者 删除
	if setObj.Len() == 0 {
		db.Remove(key)
	} else {
		db.PutEntity(key, &database.DataEntity{
			Data: setObj,
		})
	}

	// 写入 aof 文件
	cmdArgs := make([][]byte, 2)
	cmdArgs[0] = []byte(key)
	cmdArgs[1] = []byte(strconv.Itoa(count))
	db.addAof(util.ToCmdLineWithName("SPOP", cmdArgs...))

	// 构造 reply
	if count == 1 {
		return reply.MakeBulkReply([]byte(members[0]))
	}

	result := make([][]byte, len(members))
	for i, member := range members {
		result[i] = []byte(member)
	}
	return reply.MakeMultiBulkReply(result)
}

func execSRandMember(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil || setObj.Len() == 0 {
		return reply.MakeNullBulkReply()
	}

	count := 1
	withReplacement := false
	if len(args) >= 2 {
		var err error
		count, err = strconv.Atoi(string(args[1]))
		if err != nil {
			return reply.MakeStandardErrorReply("ERR value is not an integer")
		}
		// count 为负数时，
		// Negative count means return with replacement (can have duplicates)
		if count < 0 {
			withReplacement = true
			count = -count
		}
	}

	var members []string
	if withReplacement {
		members = setObj.RandomMembers(count)
	} else {
		members = setObj.RandomDistinctMembers(count)
	}

	// 构造 reply
	if len(args) == 1 || (count == 1 && len(members) > 0) {
		return reply.MakeBulkReply([]byte(members[0]))
	}

	result := make([][]byte, len(members))
	for i, member := range members {
		result[i] = []byte(member)
	}
	return reply.MakeMultiBulkReply(result)
}

// execSUnion 实现 SUNION key [key...]
// 返回多个集合的并集
func execSUnion(db *DB, args [][]byte) resp.Reply {
	// 创建空的结果集合
	result := set.NewHashSet()
	// 处理每个集合
	for _, arg := range args {
		key := string(arg)
		setObj, errReply := getAsSet(db, key)
		if errReply != nil {
			return errReply
		}
		if setObj == nil {
			continue
		}
		// 将所有成员添加到结果中
		setObj.ForEach(func(member string) bool {
			result.Add(member)
			return true
		})
	}
	// 将集合转换为回复
	members := result.Members()
	resultBytes := make([][]byte, len(members))
	for i, member := range members {
		resultBytes[i] = []byte(member)
	}
	return reply.MakeMultiBulkReply(resultBytes)
}

// execSUnionStore 实现 SUNIONSTORE destination key [key...]
// 将多个集合的并集存储到一个新集合中
func execSUnionStore(db *DB, args [][]byte) resp.Reply {
	destKey := string(args[0])
	keys := args[1:]
	// 执行并集操作
	unionReply := execSUnion(db, keys)
	if _, ok := unionReply.(reply.ErrorReply); ok {
		return unionReply
	}
	// 使用并集结果创建新集合
	unionResult := unionReply.(*reply.MultiBulkReply)
	newSet := set.NewHashSet()
	for _, member := range unionResult.Args {
		newSet.Add(string(member))
	}
	// 将集合存储到数据库中
	db.PutEntity(destKey, &database.DataEntity{
		Data: newSet,
	})
	// 添加至 AOF
	db.addAof(util.ToCmdLineWithName("SUNIONSTORE", args...))
	return reply.MakeIntReply(int64(newSet.Len()))
}

// execSInter 实现 SINTER key [key...]
// 返回多个集合的交集
func execSInter(db *DB, args [][]byte) resp.Reply {
	if len(args) == 0 {
		return reply.MakeEmptyMultiBulkReply()
	}
	// 获取第一个集合作为基础
	key := string(args[0])
	firstSet, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if firstSet == nil {
		return reply.MakeEmptyMultiBulkReply()
	}
	// 使用第一个集合的成员创建结果集合
	result := set.NewHashSet()
	firstSet.ForEach(func(member string) bool {
		result.Add(member)
		return true
	})
	// 与其他每个集合取交集
	for i := 1; i < len(args); i++ {
		key := string(args[i])
		currentSet, errReply := getAsSet(db, key)
		if errReply != nil {
			return errReply
		}
		// 空集合或键不存在意味着交集为空
		if currentSet == nil {
			return reply.MakeEmptyMultiBulkReply()
		}
		// 仅保留当前集合中存在的成员
		toRemove := make([]string, 0)
		result.ForEach(func(member string) bool {
			if !currentSet.Contains(member) {
				toRemove = append(toRemove, member)
			}
			return true
		})
		// 移除非交集成员
		for _, member := range toRemove {
			result.Remove(member)
		}
		// 如果结果已为空，提前终止
		if result.Len() == 0 {
			return reply.MakeEmptyMultiBulkReply()
		}
	}
	// 将结果转换为回复
	members := result.Members()
	resultBytes := make([][]byte, len(members))
	for i, member := range members {
		resultBytes[i] = []byte(member)
	}
	return reply.MakeMultiBulkReply(resultBytes)
}

// execSInterStore 实现 SINTERSTORE destination key [key...]
// 将多个集合的交集存储到一个新集合中
func execSInterStore(db *DB, args [][]byte) resp.Reply {
	destKey := string(args[0])
	keys := args[1:]
	// 执行交集操作
	interReply := execSInter(db, keys)
	if _, ok := interReply.(reply.ErrorReply); ok {
		return interReply
	}
	// 使用交集结果创建新集合
	interResult, ok := interReply.(*reply.MultiBulkReply)
	if !ok {
		return reply.MakeEmptyMultiBulkReply()
	}
	newSet := set.NewHashSet()
	for _, member := range interResult.Args {
		newSet.Add(string(member))
	}
	// 将集合存储到数据库中
	db.PutEntity(destKey, &database.DataEntity{
		Data: newSet,
	})
	// 添加至 AOF
	db.addAof(util.ToCmdLineWithName("SINTERSTORE", args...))
	return reply.MakeIntReply(int64(newSet.Len()))
}

// execSDiff 实现 SDIFF key [key...]
// 返回集合之间的差集
func execSDiff(db *DB, args [][]byte) resp.Reply {
	// 获取第一个集合作为基础
	key := string(args[0])
	firstSet, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if firstSet == nil {
		return reply.MakeEmptyMultiBulkReply()
	}
	// 使用第一个集合的成员创建结果集合
	result := set.NewHashSet()
	firstSet.ForEach(func(member string) bool {
		result.Add(member)
		return true
	})
	// 移除后续集合中出现的成员
	for i := 1; i < len(args); i++ {
		key := string(args[i])
		currentSet, errReply := getAsSet(db, key)
		if errReply != nil {
			return errReply
		}
		if currentSet == nil {
			continue
		}
		// 移除当前集合中存在的成员
		currentSet.ForEach(func(member string) bool {
			result.Remove(member)
			return true
		})
		// 如果结果已为空，提前终止
		if result.Len() == 0 {
			return reply.MakeEmptyMultiBulkReply()
		}
	}
	// 将结果转换为回复
	members := result.Members()
	resultBytes := make([][]byte, len(members))
	for i, member := range members {
		resultBytes[i] = []byte(member)
	}
	return reply.MakeMultiBulkReply(resultBytes)
}

// execSDiffStore 实现 SDIFFSTORE destination key [key...]
// 将集合之间的差集存储到一个新集合中
func execSDiffStore(db *DB, args [][]byte) resp.Reply {
	destKey := string(args[0])
	keys := args[1:]
	// 执行差集操作
	diffReply := execSDiff(db, keys)
	if _, ok := diffReply.(reply.ErrorReply); ok {
		return diffReply
	}
	// 使用差集结果创建新集合
	diffResult, ok := diffReply.(*reply.MultiBulkReply)
	if !ok {
		return reply.MakeIntReply(0)
	}
	newSet := set.NewHashSet()
	for _, member := range diffResult.Args {
		newSet.Add(string(member))
	}
	// 将集合存储到数据库中
	db.PutEntity(destKey, &database.DataEntity{
		Data: newSet,
	})
	// 添加至 AOF
	db.addAof(util.ToCmdLineWithName("SDIFFSTORE", args...))
	return reply.MakeIntReply(int64(newSet.Len()))
}

// SetType 表示集合的类型（intset 或 hashset）
func execSetType(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	// 获取集合
	setObj, errReply := getAsSet(db, key)
	if errReply != nil {
		return errReply
	}
	if setObj == nil {
		return reply.MakeNullBulkReply()
	}
	// 判断集合类型
	if setObj.IsIntSet() {
		return reply.MakeStatusReply("intset")
	}
	return reply.MakeStatusReply("hashset")
}
