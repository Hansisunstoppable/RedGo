package tcp

import (
	"Godis/interface/tcp"
	"Godis/lib/logger"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Config stores tcp server properties(端口号)
type Config struct {
	Address string
}

// ListenAndServeWithSignal 绑定端口，启动服务，将关闭信号与 closeChan 绑定
func ListenAndServeWithSignal(cfg *Config, handler tcp.Handler) error {
	closeChan := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		switch sig {
		// 收到系统的退出信号后，就向 closeChan 写入信息，结束服务
		case syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			closeChan <- struct{}{}
		}
	}()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("bind: %s, start listening...", cfg.Address))
	ListenAndServe(listener, handler, closeChan)
	return nil
}

// ListenAndServe 循环监听客户端连接，处理连接，并响应关闭信号
func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan <-chan struct{}) {
	// 监听关闭信号
	go func() {
		<-closeChan // 信号到达后，才能继续执行下面的关闭逻辑
		logger.Info("shutting down...")
		_ = listener.Close()
		_ = handler.Close()
	}()

	// 非正常关闭的情况
	defer func() {
		_ = listener.Close()
		_ = handler.Close()
	}()

	// 监听端口号
	ctx := context.Background()
	var waitDone sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		logger.Info("accept a new connection from ", conn.RemoteAddr().String())
		waitDone.Add(1)
		go func() {
			defer waitDone.Done()
			handler.Handle(ctx, conn)
		}()
	}
	waitDone.Wait()
}
