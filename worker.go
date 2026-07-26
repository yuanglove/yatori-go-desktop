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
	return fmt.Sprintf("[%s] [\u7cfb\u7edf] %s",
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...))
}

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
		p("\u521d\u59cb\u5316\u6570\u636e\u5e93\u5931\u8d25: %s", err)
		return 1
	}

	po, err := service.GetAccountPO(uid)
	if err != nil {
		p("\u8d26\u53f7\u4e0d\u5b58\u5728: %s", err)
		return 1
	}

	cfgPath, err := service.DefaultConfigPath()
	if err != nil {
		p("\u83b7\u53d6\u914d\u7f6e\u8def\u5f84\u5931\u8d25: %s", err)
		return 1
	}

	cfg, err := service.LoadConfig(cfgPath)
	if err != nil {
		p("\u52a0\u8f7d\u914d\u7f6e\u5931\u8d25: %s", err)
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
	p("\u65e5\u5fd7\u914d\u7f6e: level=%s model=%d file=%d", setting.BasicSetting.LogLevel, setting.BasicSetting.LogModel, setting.BasicSetting.LogOutFileSw)

	user := service.BuildUserFromPO(po)

	emit := func(format string, args ...interface{}) {
		if len(args) > 0 {
			fmt.Println(fmt.Sprintf(format, args...))
		} else {
			fmt.Println(format)
		}
	}

	if needsCoreRuntime(po.AccountType) {
		p("\u521d\u59cb\u5316 Core Runtime...")
		if err := service.EnsureCoreRuntime(); err != nil {
			p("Core Runtime \u521d\u59cb\u5316\u5931\u8d25: %s", err)
			p("OCR/ONNX \u8fd0\u884c\u73af\u5883\u52a0\u8f7d\u5931\u8d25\uff0c\u8bf7\u786e\u8ba4\u4f7f\u7528\u5b8c\u6574\u53d1\u5e03\u5305\u5e76\u5b89\u88c5 VC++ \u8fd0\u884c\u5e93\u540e\u91cd\u8bd5")
			return 1
		}
		p("Core Runtime \u521d\u59cb\u5316\u5b8c\u6210")
	} else {
		p("\u8df3\u8fc7 Core Runtime \u521d\u59cb\u5316\uff1a\u5f53\u524d\u5e73\u53f0\u65e0\u9700 OCR")
	}

	p("\u6b63\u5728\u767b\u5f55 %s (%s)...", po.Account, po.AccountType)

	var runErr error
	switch po.AccountType {
	case "XUEXITONG":
		act := service.BuildActivity(po)
		if act == nil {
			p("\u6784\u5efa\u5b66\u4e60\u901a Activity \u5931\u8d25")
			return 1
		}
		if err := act.Login(); err != nil {
			p("\u767b\u5f55\u5931\u8d25: %s", err)
			return 1
		}
		cache := act.GetUserCache()
		if cache == nil {
			p("\u767b\u5f55\u540e cache \u4e3a\u7a7a")
			return 1
		}
		xxtCache, ok := cache.(*xuexitongApiPkg.XueXiTUserCache)
		if !ok {
			p("cache \u7c7b\u578b\u65ad\u8a00\u5931\u8d25\uff0c\u975e XUEXITONG")
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
		p("\u767b\u5f55\u6210\u529f\uff0c\u5f00\u59cb\u5b66\u4e60\u4efb\u52a1")
		runErr = service.SafeRunWithExamOptions(context.Background(), setting, &user, xxtCache, service.XXTExamOptions{
			SubmitThreshold:    submitThreshold,
			RandomAnswerOnFail: randomAnswerOnFail,
			AutoExamCode:       0,
			ExamCodes:          nil,
			AccountUID:         po.UID,
			AccountName:        po.Account,
		}, emit)

	case "YINGHUA", "CANGHUI":
		p("\u82f1\u534e\u53c2\u6570: URL=%q Account=%q PasswordEnc\u7a7a=%v \u5bc6\u7801\u957f\u5ea6=%d",
			po.URL, po.Account, po.PasswordEnc == "", len(service.DecodePassword(po.PasswordEnc)))
		if po.URL == "" {
			p("\u82f1\u534e URL \u4e3a\u7a7a\uff0c\u65e0\u6cd5\u767b\u5f55\uff08\u8bf7\u5728\u8d26\u53f7\u7ba1\u7406\u4e2d\u586b\u5199\u5b66\u6821\u5165\u53e3\u5730\u5740\uff09")
			return 1
		}
		if po.PasswordEnc == "" {
			p("\u82f1\u534e\u5bc6\u7801\u672a\u4fdd\u5b58\uff0c\u65e0\u6cd5\u767b\u5f55")
			return 1
		}
		yhCache := &yinghuaApiPkg.YingHuaUserCache{
			PreUrl:   po.URL,
			Account:  po.Account,
			Password: service.DecodePassword(po.PasswordEnc),
		}
		p("\u82f1\u534e cache \u6784\u5efa\u5b8c\u6210\uff0cPreUrl=%q", yhCache.PreUrl)
		if err := yinghua.YingHuaLoginAction(yhCache); err != nil {
			p("\u82f1\u534e\u767b\u5f55\u5931\u8d25: %s", err)
			return 1
		}
		p("\u82f1\u534e\u767b\u5f55\u6210\u529f\uff0c\u5f00\u59cb\u5b66\u4e60\u4efb\u52a1")
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
		p("\u5e73\u53f0 %s \u6682\u4e0d\u652f\u6301\uff0c\u8df3\u8fc7", po.AccountType)
	}

	if runErr != nil {
		p("\u4efb\u52a1\u5931\u8d25: %s", runErr)
		return 1
	}
	p("\u6240\u6709\u4efb\u52a1\u5b8c\u6210")
	return 0
}
