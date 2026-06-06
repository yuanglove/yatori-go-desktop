package main

import (
	"fmt"
	"os"

	"yatori-go-desktop/service"
)

// runCLI 保留原控制台模式入口，供 -cli flag 使用
func runCLI() {
	cfgPath, err := service.DefaultConfigPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法确定配置文件路径:", err)
		os.Exit(1)
	}
	cfg, err := service.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取配置失败:", err)
		os.Exit(1)
	}
	if errs := service.ValidateConfig(cfg); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "配置错误:", e)
		}
		os.Exit(1)
	}
	fmt.Println("CLI 模式启动，共", len(cfg.Users), "个账号")
	// 占位：后续第三阶段接入 TaskManager.StartAll()
	fmt.Println("（CLI 完整刷课逻辑将在第三阶段接入）")
}
