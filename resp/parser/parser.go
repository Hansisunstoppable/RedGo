package parser

import (
	"Godis/interface/resp"
	"Godis/lib/logger"
	"Godis/resp/reply"
	"bufio"
	"errors"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
)

// Payload 解析后的结果
type Payload struct {
	Data resp.Reply // 客户端发给服务端的数据 和 服务端发给客户端的数据使用的是一个结构，因此也能用 Reply 接口存
	Err  error
}

// readState 解析状态
type readState struct {
	readingMultiLine  bool     // 是否正在读取多行数据，处理数组回复 * 和多行字符串回复 $
	expectedArgsCount int      // 期望读取的参数数量，处理数组回复 *
	msgType           byte     // 消息类型
	args              [][]byte // 已读取参数，每个参数对应一个 []byte
	bulkLen           int64    // 接下来要读取的 Bulk(多行) 回复的长度
}

func (r *readState) isDone() bool {
	// 期望读取参数量大于 0，且已读取的 等于 期望读取的
	return r.expectedArgsCount > 0 && len(r.args) == r.expectedArgsCount
}

// ParseStream 异步解析数据，非阻塞读写，立即返回一个通道
func ParseStream(reader io.Reader) <-chan *Payload {
	ch := make(chan *Payload)
	go parseIt(reader, ch)
	return ch
}

// parseStream 读取并解析二进制原始数据, 并封装为 resp.Reply 写入到 ch 通道
func parseIt(reader io.Reader, ch chan<- *Payload) {
	defer func() {
		// panic 后，通过 recover 恢复
		if err := recover(); err != nil {
			// 打印调用栈信息
			logger.Error(string(debug.Stack()))
		}
	}()

	bufReader := bufio.NewReader(reader) // 读取缓冲区
	var state readState
	var err error
	var msg []byte

	// 读取数据
	for {
		var ioErr bool                                // 是否发生 IO 错误
		msg, ioErr, err = readLine(bufReader, &state) // 根据 state 里的参数逐行读取解析
		if err != nil {
			// IO 错误, 关闭通道，直接退出
			if ioErr {
				ch <- &Payload{Err: err}
				close(ch)
				return
			}
			// 非 IO 错误，继续执行
			ch <- &Payload{Err: err}
			state = readState{} // 重置状态，readState{}
			continue
		}
		// 单行读取
		if !state.readingMultiLine {
			if msg[0] == '*' {
				// 解析第一行，包含行数信息，如 "*3/r/n", 表示接下来 3 行都是数组回复
				err = parseMultiBulkHeader(msg, &state)
				if err != nil {
					ch <- &Payload{Err: errors.New("Protocol error" + string(msg))}
					state = readState{}
					continue
				}
				// 需要的参数量为 0（需要读取接下来 0 行），直接返回
				if state.expectedArgsCount == 0 {
					ch <- &Payload{Data: &reply.EmptyMultiBulkReply{}}
					state = readState{}
					continue
				}
			} else if msg[0] == '$' {
				err := parseBulkHeader(msg, &state)
				if err != nil {
					ch <- &Payload{Err: errors.New("Protocol error" + string(msg))}
					state = readState{}
					continue
				}
				if state.bulkLen == -1 {
					ch <- &Payload{Data: reply.MakeNullBulkReply()}
					state = readState{}
					continue
				}
			} else {
				// 单行回复, 状态(+)、错误(-)、整数(:)
				result, err := parseSingleLineReply(msg)
				ch <- &Payload{Data: result, Err: err} // 解析结果送入通道
				state = readState{}
				continue
			}
		} else {
			// 多行读取，读取 bulk 与 arrays 的数据部分
			err = readBody(msg, &state)
			if err != nil {
				ch <- &Payload{Err: errors.New("Protocol error" + string(msg))}
				state = readState{}
				continue
			}
			// 如果满足 state.isDone(), 表示已经完整解析了信息体，封装返回
			if state.isDone() {
				var result resp.Reply
				if state.msgType == '*' {
					result = reply.MakeMultiBulkReply(state.args)
				} else if state.msgType == '$' {
					result = reply.MakeBulkReply(state.args[0])
				}
				ch <- &Payload{Data: result, Err: err}
				state = readState{}
			}
		}
	}
}

// readLine 读取一行数据原始二进制码，直到遇到 \r\n， 返回读取的数据、是否出现错误、具体错误
func readLine(bufReader *bufio.Reader, state *readState) ([]byte, bool, error) {
	var line []byte
	var err error
	// 读取一行即可（非 bulk 多行）
	if state.bulkLen == 0 {
		line, err = bufReader.ReadBytes('\n')
		if err != nil {
			return nil, true, err
		}
		// 数据不符合 RESP 的 \r\n 结尾格式
		if len(line) == 0 || line[len(line)-2] != '\r' {
			return nil, false, errors.New("invalid line terminator")
		}
	} else {
		// 读取 bulk 回复, 要根据 state.bulkLen 来读取， 2 是 \r\n 的长度
		line = make([]byte, state.bulkLen+2)
		_, err := io.ReadFull(bufReader, line)
		if err != nil {
			return nil, true, err
		}
		// 数据不符合 RESP 的 \r\n 结尾格式
		if len(line) == 0 || line[len(line)-2] != '\r' || line[len(line)-1] != '\n' {
			return nil, false, errors.New("invalid bulk line terminator")
		}
		// 已将 bulk 读取掉，重置 bulkLen
		state.bulkLen = 0
	}
	return line, false, nil
}

// parseMultiBulkHeader 解析第一行的信息，数组长度，赋值给 readState 里的多行相关字段
func parseMultiBulkHeader(msg []byte, state *readState) error {
	var err error
	var expectedLine uint64
	// 解析第一行的内容，如 "*3\r\n", 去掉前后的 '*' 与 '\r\n', 解析出 3
	expectedLine, err = strconv.ParseUint(string(msg[1:len(msg)-2]), 10, 32)

	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if expectedLine == 0 {
		// 空数组
		state.expectedArgsCount = 0
		return nil
	} else if expectedLine > 0 {
		// 设置多行相关参数
		state.msgType = msg[0]                       // 类型
		state.readingMultiLine = true                // 是否正在读取多行
		state.expectedArgsCount = int(expectedLine)  // 待读取的参数数量
		state.args = make([][]byte, 0, expectedLine) // 已经读入的数据
		return nil
	} else {
		// 负数，错误
		return errors.New("protocol error: " + string(msg))
	}
}

// parseBulkHeader 解析 bulk 的参数长度，并赋值给 readState.bulkLen
func parseBulkHeader(msg []byte, state *readState) error {
	var err error
	state.bulkLen, err = strconv.ParseInt(string(msg[1:len(msg)-2]), 10, 64)
	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if state.bulkLen == -1 { // nil bulk
		return nil
	} else if state.bulkLen > 0 {
		state.msgType = msg[0]
		state.readingMultiLine = true
		state.expectedArgsCount = 1
		state.args = make([][]byte, 0, 1)
		return nil
	} else {
		return errors.New("protocol error: " + string(msg))
	}
}

// parseSingleLineReply 把单行回复解析成 Reply: 状态(+)、错误(-)、整数(:)
func parseSingleLineReply(msg []byte) (resp.Reply, error) {
	str := strings.TrimSuffix(string(msg), "\r\n") // 去掉后缀 \r\n
	var result resp.Reply
	switch msg[0] {
	case '+': // status reply
		result = reply.MakeStatusReply(str[1:]) // 从 1 开始，去掉前缀
	case '-': // err reply
		result = reply.MakeStandardErrorReply(str[1:])
	case ':': // int reply
		val, err := strconv.ParseInt(str[1:], 10, 64)
		if err != nil {
			return nil, errors.New("protocol error: " + string(msg))
		}
		result = reply.MakeIntReply(val)
	}

	return result, nil
}

// 通过头部的信息，来读取接下来多行信息体
func readBody(msg []byte, state *readState) error {
	line := msg[:len(msg)-2]
	var err error
	if line[0] == '$' {
		// 获取 bulk 长度，供下一行读取使用
		state.bulkLen, err = strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return errors.New("protocol error: " + string(msg))
		}
		if state.bulkLen <= 0 { // 空 bulk
			state.args = append(state.args, []byte{})
			state.bulkLen = 0
		}
	} else {
		state.args = append(state.args, line)
	}
	return nil
}
