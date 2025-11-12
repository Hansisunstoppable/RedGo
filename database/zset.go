package database

import (
	"Godis/interface/database"
	"Godis/interface/resp"
	"Godis/lib/util"
	"Godis/resp/reply"
	"strconv"
)

// parseFloat 将字符串解析为 float64
func parseFloat(val string) (float64, resp.Reply) {
	score, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, reply.MakeStandardErrorReply("value is not a valid float")
	}
	return score, nil
}

// execZAdd 实现 ZADD 命令// execZAdd implements the ZADD command
// ZADD key [NX|XX] [CH] [INCR] score member [score member ...]
func execZAdd(db *DB, args [][]byte) resp.Reply {
	if len(args) < 3 || len(args)%2 == 0 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zadd' command")
	}

	key := string(args[0])

	// 获取 zset
	zsetObj, exists := getAsZSet(db, key)
	if exists && zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	added := 0
	// 这里默认不添加 [NX|XX] [CH] [INCR] 等额外参数，按照 score member 依次排列
	for i := 1; i < len(args); i += 2 {
		scoreStr := string(args[i])
		member := string(args[i+1])

		// 解析分数
		score, err := parseFloat(scoreStr)
		if err != nil {
			return err
		}

		// 插入到 zset
		if zsetObj.Add(member, score) {
			added++
		}
	}

	// 更新后的zset 存回底层数据库
	db.PutEntity(key, &database.DataEntity{Data: zsetObj})

	// 写入 aof
	db.addAof(util.ToCmdLineWithName("ZADD", args...))

	return reply.MakeIntReply(int64(added))
}

// execZScore 实现 ZSCORE 命令, ZSCORE key member
func execZScore(db *DB, args [][]byte) resp.Reply {
	if len(args) != 2 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zscore' command")
	}

	key := string(args[0])
	member := string(args[1])

	// 获取 ZSet
	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	// 获取 score
	score, exists := zsetObj.Score(member)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	return reply.MakeBulkReply([]byte(strconv.FormatFloat(score, 'f', -1, 64)))
}

// execZCard 实现 ZCARD 命令，获取元素数量
func execZCard(db *DB, args [][]byte) resp.Reply {
	if len(args) != 1 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zcard' command")
	}

	key := string(args[0])

	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeIntReply(0)
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	return reply.MakeIntReply(int64(zsetObj.Len()))
}

// execZRange 实现 ZRANGE 命令, 获取有序集合中指定范围内的成员
func execZRange(db *DB, args [][]byte) resp.Reply {
	if len(args) < 3 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zrange' command")
	}

	withScores := false
	if len(args) > 3 && string(args[3]) == "WITHSCORES" {
		withScores = true
	}

	key := string(args[0])

	// 解析处起点与终点
	start, err := strconv.Atoi(string(args[1]))
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}

	stop, err := strconv.Atoi(string(args[2]))
	if err != nil {
		return reply.MakeStandardErrorReply("value is not an integer or out of range")
	}

	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeEmptyMultiBulkReply()
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	// 获取范围内的元素
	members := zsetObj.RangeByRank(start, stop)

	// 返回结果
	if !withScores {
		result := make([][]byte, len(members))
		for i, member := range members {
			result[i] = []byte(member)
		}
		return reply.MakeMultiBulkReply(result)
	} else {
		result := make([][]byte, len(members)*2)
		for i, member := range members {
			result[i*2] = []byte(member)
			score, _ := zsetObj.Score(member)
			result[i*2+1] = []byte(strconv.FormatFloat(score, 'f', -1, 64))
		}
		return reply.MakeMultiBulkReply(result)
	}
}

// execZRem 实现 ZREM 命令，移除 zset 内的元素
func execZRem(db *DB, args [][]byte) resp.Reply {
	if len(args) < 2 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zrem' command")
	}

	key := string(args[0])

	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeIntReply(0)
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	// Remove members
	removed := 0
	for i := 1; i < len(args); i++ {
		member := string(args[i])
		if zsetObj.Remove(member) {
			removed++
		}
	}

	// 将更新后的 zset 存回底层数据库
	if removed > 0 {
		db.PutEntity(key, &database.DataEntity{Data: zsetObj})

		// 写入 aof 文件
		db.addAof(util.ToCmdLineWithName("ZREM", args...))
	}

	return reply.MakeIntReply(int64(removed))
}

// execZCount 实现 ZCOUNT 命令，获取 zset 中，score 在指定范围的元素数量
func execZCount(db *DB, args [][]byte) resp.Reply {
	if len(args) != 3 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zcount' command")
	}

	key := string(args[0])

	// 解析 min 和 max
	min, err := parseFloat(string(args[1]))
	if err != nil {
		return err
	}

	max, err := parseFloat(string(args[2]))
	if err != nil {
		return err
	}

	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeIntReply(0)
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	// 统计 score 在范围内的元素数量
	count := zsetObj.Count(min, max)

	return reply.MakeIntReply(int64(count))
}

// execZRank 实现 ZRANK 命令, ZRANK myzset member1, 获取 myzset 中 member1 的排名
func execZRank(db *DB, args [][]byte) resp.Reply {
	if len(args) != 2 {
		return reply.MakeStandardErrorReply("wrong number of arguments for 'zrank' command")
	}

	key := string(args[0])
	member := string(args[1])

	zsetObj, exists := getAsZSet(db, key)
	if !exists {
		return reply.MakeNullBulkReply()
	}
	if zsetObj == nil {
		return reply.MakeWrongTypeErrReply()
	}

	// 获取 member 的排名
	score, exists := zsetObj.Score(member)
	if !exists {
		return reply.MakeNullBulkReply()
	}

	rank := -1
	if zsetObj.Encoding() == 1 { // 跳表模式
		// 获取跳表，再按照获取 rank
		skiplist := zsetObj.GetSkiplist()
		rank = skiplist.GetRank(member, score)
	} else {
		// 按照顺序，获取所有 member
		members := zsetObj.RangeByRank(0, -1)
		// 找到对应 member 的 rank
		for i, m := range members {
			if m == member {
				rank = i
				break
			}
		}
	}

	if rank == -1 {
		return reply.MakeNullBulkReply()
	}

	return reply.MakeIntReply(int64(rank))
}

func init() {
	RegisterCommand("ZADD", execZAdd, -4) // ZADD zset1 score1 member1 至少四个参数
	RegisterCommand("ZSCORE", execZScore, 3)
	RegisterCommand("ZCARD", execZCard, 2)
	RegisterCommand("ZRANGE", execZRange, -4)
	RegisterCommand("ZREM", execZRem, -3)
	RegisterCommand("ZCOUNT", execZCount, 4)
	RegisterCommand("ZRANK", execZRank, 3)
}
