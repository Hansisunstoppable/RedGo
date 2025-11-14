package aof

import (
	"Godis/config"
	"Godis/interface/database"
	"Godis/lib/logger"
	"Godis/lib/util"
	"Godis/resp/connection"
	"Godis/resp/parser"
	"Godis/resp/reply"
	"io"
	"os"
	"strconv"
	"strings"
)

type CmdLine [][]byte

const aofBufferSize = 1 << 16 //  AOF 通道的缓冲区大小
const initDBIndex = 0

// payload 待写入的 aof 命令 与 对应的数据库编号
type payload struct {
	cmd     CmdLine // 命令
	dbIndex int     // 这条命令对应的数据库编号，恢复时，在对应的数据库执行
}

// AofHandlerhandles the AOF functionality for Redis.
type AofHandler struct {
	db          database.Database // 数据库，用于通过 aof 文件恢复时，执行命令
	aofChan     chan *payload     // 接收待写入 AOF 文件的命令的通道，实现异步写入
	aofFile     *os.File          // AOF 文件的文件句柄
	aofFileName string
	currentDB   int // AOF 下，当前数据库的编号
}

func NewAofHandler(db database.Database) (*AofHandler, error) {
	handler := &AofHandler{}
	handler.aofFileName = config.Properties.AppendFilename
	handler.db = db
	aofFile, err := os.OpenFile(handler.aofFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	handler.aofFile = aofFile
	handler.aofChan = make(chan *payload, aofBufferSize)
	handler.LoadAof() // 将之前的 AOF 文件加载到数据库中（执行所有 AOF 命令来恢复）
	// 启动一个协程，读取管道内容，写入 AOF 文件（异步写入）
	go func() {
		handler.handleAof()
	}()
	return handler, nil
}

// AddAof 添加待写入命令到 aofChan, 以便异步写入 AOF 文件
func (h *AofHandler) AddAof(dbIndex int, cmd CmdLine) {
	// TODO 为什么未开启 aof 模式时，也要创建 aof 管道？
	if h.aofChan == nil || !config.Properties.AppendOnly {
		h.aofChan = make(chan *payload, 100)
	}
	h.aofChan <- &payload{
		cmd:     cmd,
		dbIndex: dbIndex,
	}
}

// LoadAof 读取 AOF 文件中的命令，将其执行到数据库中
func (h *AofHandler) LoadAof() error {
	aofFile, err := os.Open(h.aofFileName)
	if err != nil {
		logger.Error("AOF file open error: " + err.Error())
		return err
	}
	defer func() {
		aofFile.Close()
	}()

	ch := parser.ParseStream(aofFile)    // 解析 AOF 文件得到 payload， 送入 ch
	fakeConn := &connection.Connection{} // 构造一个空 client ，后续若执行到 SELECT 命令，通过 execSelect 传入 selectedDB 字段
	for p := range ch {
		if p.Err != nil {
			// If the error is EOF or unexpected EOF, break the loop
			if p.Err == io.EOF || p.Err == io.ErrUnexpectedEOF {
				// End of file
				break
			}
			// Other errors
			logger.Error("AOF file parse error: " + p.Err.Error())
			continue
		}
		if p.Data == nil {
			logger.Error("AOF file empty payload")
			continue
		}
		r, ok := p.Data.(*reply.MultiBulkReply) // r 里面为解析出来的待执行命令
		if !ok {
			logger.Error("AOF file require multi bulk reply")
			continue
		}

		// 若遇到了 select 命令，则要更新 h.currentDB
		cmdName := strings.ToLower(string(r.Args[0]))
		if cmdName == "select" {
			if len(r.Args) != 2 {
				logger.Error("Invalid DB index in SELECT command: " + err.Error())
			}
			dbIndex, err := strconv.Atoi(string(r.Args[1]))
			if err != nil {
				logger.Error("Invalid DB index in SELECT command: " + err.Error())
				continue
			}
			h.currentDB = dbIndex
		}

		// exec 方法需要传入一个 client，用于传递执行命令后的响应。但这里无需响应，只需保存 selectedDB（当前选择的数据库编号）
		rep := h.db.Exec(fakeConn, r.Args)
		if reply.IsError(rep) {
			logger.Error("Execute AOF command error")
		}
	}
	// 初始默认连接 0 号数据库; 若执行完 aof 之后不在 0 号数据库，则需要向 aof 写入 select 0
	if h.currentDB != 0 {
		data := reply.MakeMultiBulkReply(util.ToCmdLine("SELECT", strconv.Itoa(initDBIndex))).ToBytes()
		_, err := h.aofFile.Write(data)
		if err != nil {
			logger.Error("write aof file error: " + err.Error())
		}
	}

	return nil
}

// handleAof 从 aofChan 通道中读取命令，写入 AOF 文件
func (h *AofHandler) handleAof() {
	// 先判断是否产生数据库切换
	// 若需要切换，要将额外的 Select 命令写入到 aof 文件，以供未来恢复的时候执行
	// h.currentDB = 0 无需初始化为 0，因为一开始有可能不是 0
	for p := range h.aofChan {
		if p.dbIndex != h.currentDB { // 数据库产生切换，要向 aof 文件写入 SELECT dbIndex 命令
			h.currentDB = p.dbIndex                                                                       // 更新当前数据库序号
			data := reply.MakeMultiBulkReply(util.ToCmdLine("SELECT", strconv.Itoa(p.dbIndex))).ToBytes() // 写入 SELECT 命令
			_, err := h.aofFile.Write(data)
			if err != nil {
				logger.Error("write aof file error: " + err.Error())
				continue
			}
		}

		// 将命令本身 写入 aof 文件
		data := reply.MakeMultiBulkReply(p.cmd).ToBytes()
		_, err := h.aofFile.Write(data)
		if err != nil {
			logger.Error("write aof file error: " + err.Error())
			continue
		}
		// 确保数据立即刷新到磁盘
		// h.aofFile.Sync()
	}
}
