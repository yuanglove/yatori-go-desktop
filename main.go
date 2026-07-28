package main

import (
	"embed"
	"flag"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cliMode := flag.Bool("cli", false, "以 CLI 模式运行")
	workerMode := flag.Bool("worker", false, "内部 worker 模式")
	workerUID := flag.String("uid", "", "worker 模式：任务 UID")
	flag.Parse()

	if *workerMode && *workerUID != "" {
		os.Exit(runWorker(*workerUID))
		return
	}

	if *cliMode {
		runCLI()
		return
	}

	releaseInstance, alreadyRunning, err := acquireDesktopInstance()
	if err != nil {
		panic(err)
	}
	if alreadyRunning {
		return
	}
	defer releaseInstance()

	app := NewApp()
	err = wails.Run(&options.App{
		Title:            "Yatori 学习管理工具",
		Width:            1200,
		Height:           780,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})
	if err != nil {
		panic(err)
	}
}
