package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/yatori-dev/yatori-go-core/aggregation/yinghua"
	xuexitongApiPkg "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	yinghuaApiPkg "github.com/yatori-dev/yatori-go-core/api/yinghua"
	ctype "github.com/yatori-dev/yatori-go-core/models/ctype"
	consoleConfig "yatori-go-console/config"
	"yatori-go-desktop/service"
)

func workerLog(format string, args ...interface{}) string {
	return fmt.Sprintf("[%s] [系统] %s",
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...))
}

// needsCoreRuntime 判断平台是否需要初始化 ONNX Runtime（OCR）
func needsCoreRuntime(platform string) bool {
	switch platform {
	case "YINGHUA", "CANGHUI", "XUEXITONG", "CQIE", "QSXT":
		return true
	default:
		return false
	}
}

func runWorker(uid string) int {
	p := func(format string, args ...interface{}) {
		fmt.Println(workerLog(format, args...))
	}

	defer func() {
		if r := recover(); r != nil {
			p("worker panic: %v", r)
			p("stack:\n%s", debug.Stack())
		}
	}()

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
			LogModel:     cfg.Setting.BasicSetting.LogModel,
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
	service.SetRuntimeQuestionBankSetting(cfg.Setting.ApiQueSetting)
	p("日志配置: level=%s model=%d file=%d", setting.BasicSetting.LogLevel, setting.BasicSetting.LogModel, setting.BasicSetting.LogOutFileSw)

	user := service.BuildUserFromPO(po)

	emit := func(format string, args ...interface{}) {
		if len(args) > 0 {
			fmt.Println(fmt.Sprintf(format, args...))
		} else {
			fmt.Println(format)
		}
	}

	// 按平台决定是否初始化 ONNX Runtime
	if needsCoreRuntime(po.AccountType) {
		p("初始化 Core Runtime...")
		if err := service.EnsureCoreRuntime(); err != nil {
			p("Core Runtime 初始化失败: %s", err)
			p("这通常是 OCR/ONNX 运行环境加载失败。请确认使用完整发布包、不要直接在压缩包内运行，并安装 Microsoft Visual C++ 2015-2022 x64 运行库后重试。")
			return 1
		}
		p("Core Runtime 初始化完成")
	} else {
		p("跳过 Core Runtime 初始化：当前平台无需 OCR")
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
		randomAnswerOnFail := 0
		var cc service.CoursesCustom
		if json.Unmarshal([]byte(po.CoursesCustom), &cc) == nil {
			if cc.SubmitThresholdPercent > 0 {
				submitThreshold = cc.SubmitThresholdPercent
			}
			randomAnswerOnFail = cc.RandomAnswerOnFail
		}
		p("登录成功，开始学习任务")
		runErr = service.SafeRun(context.Background(), setting, &user, xxtCache, submitThreshold, randomAnswerOnFail, emit)

	case "YINGHUA", "CANGHUI":
		p("英华参数: URL=%q Account=%q PasswordEnc空=%v 密码长度=%d",
			po.URL, po.Account, po.PasswordEnc == "", len(service.DecodePassword(po.PasswordEnc)))
		if po.URL == "" {
			p("英华 URL 为空，无法登录（请在账号管理中填写学校入口地址）")
			return 1
		}
		if po.PasswordEnc == "" {
			p("英华密码未保存，无法登录")
			return 1
		}
		yhCache := &yinghuaApiPkg.YingHuaUserCache{
			PreUrl:   po.URL,
			Account:  po.Account,
			Password: service.DecodePassword(po.PasswordEnc),
		}
		p("英华 cache 构建完成，PreUrl=%q", yhCache.PreUrl)
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
		randFail := 0
		submitThreshold := 60
		var ccIcve service.CoursesCustom
		if json.Unmarshal([]byte(po.CoursesCustom), &ccIcve) == nil {
			randFail = ccIcve.RandomAnswerOnFail
			if ccIcve.SubmitThresholdPercent > 0 {
				submitThreshold = ccIcve.SubmitThresholdPercent
			}
		}
		service.RunIcveWithCourseOpts(setting, user, randFail, submitThreshold, emit)
	case "HQKJ":
		service.RunHqkj(setting, user, emit)
	case "KETANGX":
		service.RunKetangx(setting, user, emit)

	default:
		p("平台 %s 暂不支持，跳过", po.AccountType)
	}

	if runErr != nil {
		p("任务失败: %s", runErr)
		return 1
	}
	p("所有任务完成")
	return 0
}
