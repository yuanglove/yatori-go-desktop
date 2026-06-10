package service

// other_runners.go
// worker 子进程里 log.Fatal/os.Exit 只杀子进程，不影响主窗口。
// 直接复用原 logic 包的 UserLoginOperation + RunBrushOperation。

import (
	"fmt"
	"time"

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
	emit("%s", platformLog("ICVE", user.Account, "开始登录"))
	users := icveLogic.FilterAccount(fakeConfigData(user))
	caches := icveLogic.UserLoginOperation(users)
	emit("%s", platformLog("ICVE", user.Account, "登录成功，开始刷课"))
	icveLogic.RunBrushOperation(setting, users, caches)
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
