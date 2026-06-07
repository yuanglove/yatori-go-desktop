package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	consoleConfig "yatori-go-console/config"
	"github.com/yatori-dev/yatori-go-core/aggregation/yinghua"
	xuexitongApiPkg "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	yinghuaApiPkg "github.com/yatori-dev/yatori-go-core/api/yinghua"
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

	emit := func(format string, args ...interface{}) {
		if len(args) > 0 {
			fmt.Println(fmt.Sprintf(format, args...))
		} else {
			fmt.Println(format)
		}
	}

	p("正在登录 %s (%s)...", po.Account, po.AccountType)

	var runErr error
	switch po.AccountType {
	case "XUEXITONG":
		act := service.BuildActivity(po)
		if act == nil {
			p("构建学习通 Activity 失败")
			return 1
		}
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
		submitThreshold := 100
		var cc service.CoursesCustom
		if json.Unmarshal([]byte(po.CoursesCustom), &cc) == nil && cc.SubmitThresholdPercent > 0 {
			submitThreshold = cc.SubmitThresholdPercent
		}
		p("登录成功，开始学习任务")
		runErr = service.SafeRun(context.Background(), setting, &user, xxtCache, submitThreshold, emit)

	case "YINGHUA":
		yhCache := &yinghuaApiPkg.YingHuaUserCache{
			PreUrl:   po.URL,
			Account:  po.Account,
			Password: service.DecodePassword(po.PasswordEnc),
		}
		if err := yinghua.YingHuaLoginAction(yhCache); err != nil {
			p("英华登录失败: %s", err)
			return 1
		}
		p("英华登录成功，开始学习任务")
		runErr = service.SafeYingHuaRun(context.Background(), setting, &user, yhCache, emit)

	case "ENAEA":
		service.RunEnaea(setting, user, emit)
	case "QSXT":
		service.RunQsxt(setting, user, emit)
	case "CQIE":
		service.RunCqie(setting, user, emit)
	case "WELEARN":
		service.RunWeLearn(setting, user, emit)
	case "ICVE":
		service.RunIcve(setting, user, emit)
	case "HQKJ":
		service.RunHqkj(setting, user, emit)
	case "KETANGX":
		service.RunKetangx(setting, user, emit)

	default:
		p("平台 %s 暂不支持 worker 模式", po.AccountType)
		return 1
	}

	if runErr != nil {
		p("任务失败: %s", runErr)
		return 1
	}
	p("任务完成")
	return 0
}
