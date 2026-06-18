package service

// other_runners.go
// worker 子进程里 log.Fatal/os.Exit 只杀子进程，不影响主窗口。
// 直接复用原 logic 包的 UserLoginOperation + RunBrushOperation。

import (
	"fmt"
	"strings"
	"time"

	icveAction "github.com/yatori-dev/yatori-go-core/aggregation/icve"
	icveApi "github.com/yatori-dev/yatori-go-core/api/icve"
	consoleConfig "yatori-go-console/config"

	cqieLogic "yatori-go-console/logic/cqie"
	enaeaLogic "yatori-go-console/logic/enaea"
	hqkjLogic "yatori-go-console/logic/haiqikeji"
	icveLogic "yatori-go-console/logic/icve"
	ketangxLogic "yatori-go-console/logic/ketangx"
	qsxtLogic "yatori-go-console/logic/qingshuxuetang"
)

func fakeConfigData(user consoleConfig.User) *consoleConfig.JSONDataForConfig {
	return &consoleConfig.JSONDataForConfig{Users: []consoleConfig.User{user}}
}

func platformLog(platform, account, msg string) string {
	return fmt.Sprintf("[%s] [%s][%s] %s",
		time.Now().Format("2006-01-02 15:04:05"), platform, account, msg)
}

func RunEnaea(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("ENAEA", user.Account, "开始登录"))
	users := enaeaLogic.FilterAccount(fakeConfigData(user))
	caches := enaeaLogic.UserLoginOperation(users)
	emit("%s", platformLog("ENAEA", user.Account, "登录成功，开始刷课"))
	enaeaLogic.RunBrushOperation(setting, users, caches)
	emit("%s", platformLog("ENAEA", user.Account, "任务完成"))
}

func RunQsxt(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("QSXT", user.Account, "开始登录"))
	users := qsxtLogic.FilterAccount(fakeConfigData(user))
	caches := qsxtLogic.UserLoginOperation(users)
	emit("%s", platformLog("QSXT", user.Account, "登录成功，开始刷课"))
	qsxtLogic.RunBrushOperation(setting, users, caches)
	emit("%s", platformLog("QSXT", user.Account, "任务完成"))
}

func RunCqie(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("CQIE", user.Account, "开始登录"))
	users := cqieLogic.FilterAccount(fakeConfigData(user))
	caches := cqieLogic.UserLoginOperation(users)
	emit("%s", platformLog("CQIE", user.Account, "登录成功，开始刷课"))
	cqieLogic.RunBrushOperation(setting, users, caches)
	emit("%s", platformLog("CQIE", user.Account, "任务完成"))
}

func RunWeLearn(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	RunWeLearnSafe(setting, user, emit)
}

func RunIcve(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("ICVE", user.Account, "开始 Cookie 登录"))
	if strings.TrimSpace(user.Password) == "" || len(strings.TrimSpace(user.Password)) <= 30 {
		emit("%s", platformLog("ICVE", user.Account, "Cookie 为空或长度过短：智慧职教目前只支持 Cookie 登录，请把浏览器复制的完整 Cookie 填到密码/Cookie 字段"))
		return
	}
	users := icveLogic.FilterAccount(fakeConfigData(user))
	if len(users) == 0 {
		emit("%s", platformLog("ICVE", user.Account, "账号过滤失败：未找到 ICVE 账号配置"))
		return
	}
	cache := &icveApi.IcveUserCache{Account: user.Account, Password: strings.TrimSpace(user.Password)}
	if err := icveAction.IcveCookieLogin(cache); err != nil {
		emit("%s", platformLog("ICVE", user.Account, "Cookie 登录失败: "+err.Error()))
		return
	}
	emit("%s", platformLog("ICVE", user.Account, "Cookie 登录成功，开始刷课"))
	icveLogic.RunBrushOperation(setting, users, []*icveApi.IcveUserCache{cache})
	emit("%s", platformLog("ICVE", user.Account, "任务完成"))
}

func RunHqkj(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("HQKJ", user.Account, "开始登录"))
	users := hqkjLogic.FilterAccount(fakeConfigData(user))
	caches := hqkjLogic.UserLoginOperation(users)
	emit("%s", platformLog("HQKJ", user.Account, "登录成功，开始刷课"))
	hqkjLogic.RunBrushOperation(setting, users, caches)
	emit("%s", platformLog("HQKJ", user.Account, "任务完成"))
}

func RunKetangx(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	emit("%s", platformLog("KETANGX", user.Account, "开始登录"))
	users := ketangxLogic.FilterAccount(fakeConfigData(user))
	caches := ketangxLogic.UserLoginOperation(users)
	emit("%s", platformLog("KETANGX", user.Account, "登录成功，开始刷课"))
	ketangxLogic.RunBrushOperation(setting, users, caches)
	emit("%s", platformLog("KETANGX", user.Account, "任务完成"))
}
