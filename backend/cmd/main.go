package main

import (
	"log"

	"platform/config"
	"platform/router"
	"platform/util"
)

func main() {
	// 加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := util.InitDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 启动服务器
	log.Printf("服务器启动中，监听端口: %s", config.AppConfig.Port)
	router.StartServer()
}
