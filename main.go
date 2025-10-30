package main

import (
	"Godis/config"
	"Godis/lib/logger"
	"Godis/resp/handler"
	"Godis/tcp"
	"fmt"
	"os"
)

// 默认配置文件
const configFile string = "redis.conf"

var defaultProperties = &config.ServerProperties{
	Bind: "0.0.0.0",
	Port: 6379,
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

func main() {
	// 读取配置文件
	if fileExists(configFile) {
		config.SetupConfig(configFile)
	} else {
		config.Properties = defaultProperties
	}

	// 启动服务端
	err := tcp.ListenAndServeWithSignal(&tcp.Config{
		Address: fmt.Sprintf("%s:%d",
			config.Properties.Bind,
			config.Properties.Port),
	}, handler.MakeRespHandler())

	if err != nil {
		logger.Error(err)
	}
}
