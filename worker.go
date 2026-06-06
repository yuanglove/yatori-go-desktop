package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	consoleConfig "yatori-go-console/config"
	xuexitongApiPkg "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	ctype "github.com/yatori-dev/yatori-go-core/models/ctype"
	"yatori-go-desktop/service"
)

func workerLog(format string, args ...interface{}) string {
	return fmt.Sprintf("[%s] [系统] %s",
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...))
}

func runWorker(uid string) int {
	p := func(format string, args ...interface{}) {
		fmt.Println(workerLog(format, args...))
	}

	if err := service.InitDB(); err != nil {
		p("初始化数据库失败: %s", err)
		return 1
	}

	po, err := service.GetAccountPO(uid)
	if err != nil {
		p("账号不存在: %s", err)
		return 1
	}

	cfgPath, err := service.DefaultConfigPath()
	if err != nil {
		p("获取配置路径失败: %s", err)
		return 1
	}

	cfg, err := service.LoadConfig(cfgPath)
	if err != nil {
		p("加载配置失败: %s", err)
		return 1
	}

	setting := consoleConfig.Setting{
		BasicSetting: consoleConfig.BasicSetting{
			ColorLog:     0,
			LogOutFileSw: cfg.Setting.BasicSetting.LogOutFileSw,
			LogLevel:     cfg.Setting.BasicSetting.LogLevel,
		},
		EmailInform: consoleConfig.EmailInform{
			Sw:       cfg.Setting.EmailInform.Sw,
			SMTPHost: cfg.Setting.EmailInform.SMTPHost,
			SMTPPort: cfg.Setting.EmailInform.SMTPPort,
			UserName: cfg.Setting.EmailInform.UserName,
			Password: cfg.Setting.EmailInform.Password,
		},
		AiSetting: consoleConfig.AiSetting{
			AiType: ctype.AiType(cfg.Setting.AiSetting.AiType),
			AiUrl:  cfg.Setting.AiSetting.AiUrl,
			Model:  cfg.Setting.AiSetting.Model,
			APIKEY: cfg.Setting.AiSetting.APIKey,
		},
		ApiQueSetting: consoleConfig.ApiQueSetting{
			Url: cfg.Setting.ApiQueSetting.Url,
		},
	}
	if setting.BasicSetting.LogLevel == "" {
		setting.BasicSetting.LogLevel = "INFO"
	}

	user := service.BuildUserFromPO(po)

	act := service.BuildActivity(po)
	if act == nil {
		p("平台 %s 暂不支持 worker 模式", po.AccountType)
		return 1
	}

	p("正在登录 %s...", po.Account)
	if err := act.Login(); err != nil {
		p("登录失败: %s", err)
		return 1
	}

	cache := act.GetUserCache()
	if cache == nil {
		p("登录后 cache 为空")
		return 1
	}
	xxtCache, ok := cache.(*xuexitongApiPkg.XueXiTUserCache)
	if !ok {
		p("cache 类型断言失败，非 XUEXITONG")
		return 1
	}

	p("登录成功，开始学习任务")

	submitThreshold := 100
	var cc service.CoursesCustom
	if json.Unmarshal([]byte(po.CoursesCustom), &cc) == nil && cc.SubmitThresholdPercent > 0 {
		submitThreshold = cc.SubmitThresholdPercent
	}

	emit := func(format string, args ...interface{}) {
		if len(args) > 0 {
			fmt.Println(fmt.Sprintf(format, args...))
		} else {
			fmt.Println(format)
		}
	}

	if err := service.SafeRun(context.Background(), setting, &user, xxtCache, submitThreshold, emit); err != nil {
		p("任务失败: %s", err)
		return 1
	}

	p("任务完成")
	return 0
}
