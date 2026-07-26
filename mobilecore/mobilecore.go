package mobilecore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/yatori-dev/yatori-go-core/aggregation/yinghua"
	xuexitongApiPkg "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	yinghuaApiPkg "github.com/yatori-dev/yatori-go-core/api/yinghua"
	ctype "github.com/yatori-dev/yatori-go-core/models/ctype"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	consoleConfig "yatori-go-console/config"
	"yatori-go-desktop/service"
)

const (
	CoreVersion      = "0.4.0"
	APISchemaVersion = 1
)

var state = struct {
	sync.Mutex
	dataDir    string
	configPath string
	logs       []string
	tasks      map[string]taskStatus
	cancels    map[string]context.CancelFunc
	maxWorkers int
}{
	tasks:      map[string]taskStatus{},
	cancels:    map[string]context.CancelFunc{},
	maxWorkers: 3,
}

type initInfo struct {
	DataDir          string `json:"dataDir"`
	DesktopCore      string `json:"desktopCoreVersion"`
	APISchemaVersion int    `json:"apiSchemaVersion"`
}

type capabilityInfo struct {
	NativeGoCore     bool     `json:"nativeGoCore"`
	DesktopCore      string   `json:"desktopCoreVersion"`
	APISchemaVersion int      `json:"apiSchemaVersion"`
	Platforms        []string `json:"platforms"`
	Mode             string   `json:"mode"`
}

type taskStatus struct {
	UID       string `json:"uid"`
	Account   string `json:"account"`
	Platform  string `json:"platform"`
	State     string `json:"state"`
	StartTime string `json:"startTime,omitempty"`
	LastLog   string `json:"lastLog,omitempty"`
	Error     string `json:"error,omitempty"`
}

func Init(dataDir string) string {
	if dataDir == "" {
		return fail(CodeInvalidArgument, "dataDir cannot be empty")
	}
	for _, dir := range []string{
		dataDir,
		filepath.Join(dataDir, "assets"),
		filepath.Join(dataDir, "assets", "log"),
		filepath.Join(dataDir, "assets", "ocr"),
		filepath.Join(dataDir, "cache"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fail(CodeInternalError, err.Error())
		}
	}

	service.SetDataDir(dataDir)
	configPath, err := service.DefaultConfigPath()
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	cfg, err := service.LoadConfig(configPath)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	if err := service.SaveConfig(configPath, cfg); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	if err := service.InitDB(); err != nil {
		return fail(CodeInternalError, err.Error())
	}

	state.Lock()
	state.dataDir = dataDir
	state.configPath = configPath
	state.maxWorkers = cfg.Setting.BasicSetting.MaxWorkers
	state.logs = appendLimited(state.logs, logLine("system", "Go Mobile Core initialized"))
	state.Unlock()

	return ok(initInfo{DataDir: dataDir, DesktopCore: CoreVersion, APISchemaVersion: APISchemaVersion})
}

func GetCapabilities() string {
	return ok(capabilityInfo{
		NativeGoCore:     true,
		DesktopCore:      CoreVersion,
		APISchemaVersion: APISchemaVersion,
		Platforms:        []string{"ICVE", "XUEXITONG", "YINGHUA", "WELEARN", "HQKJ"},
		Mode:             "desktop-core-aar",
	})
}

func GetCoreVersionJSON() string {
	return ok(map[string]interface{}{
		"desktopCoreVersion": CoreVersion,
		"apiSchemaVersion":   APISchemaVersion,
	})
}

func GetConfigJSON() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	cfg, err := service.LoadConfig(currentConfigPath())
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(cfg)
}

func SaveConfigJSON(configJSON string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	var cfg service.AppConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}
	if cfg.Setting.BasicSetting.LogLevel == "" {
		cfg.Setting.BasicSetting.LogLevel = "INFO"
	}
	if cfg.Setting.BasicSetting.MaxWorkers <= 0 {
		cfg.Setting.BasicSetting.MaxWorkers = 3
	}
	if errs := service.ValidateConfig(cfg); len(errs) > 0 {
		return fail(CodeInvalidArgument, errs[0])
	}
	if err := service.SaveConfig(currentConfigPath(), cfg); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	state.Lock()
	state.maxWorkers = cfg.Setting.BasicSetting.MaxWorkers
	state.Unlock()
	pushLog("system", "config saved")
	return ok("ok")
}

func ImportConfigText(text string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	var cfg service.AppConfig
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		return fail(CodeInvalidArgument, "閰嶇疆瑙ｆ瀽澶辫触: "+err.Error())
	}
	if cfg.Setting.BasicSetting.LogLevel == "" {
		cfg.Setting.BasicSetting.LogLevel = "INFO"
	}
	if cfg.Setting.BasicSetting.MaxWorkers <= 0 {
		cfg.Setting.BasicSetting.MaxWorkers = 3
	}
	if errs := service.ValidateConfig(cfg); len(errs) > 0 {
		return fail(CodeInvalidArgument, errs[0])
	}
	if err := service.SaveConfig(currentConfigPath(), cfg); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	state.Lock()
	state.maxWorkers = cfg.Setting.BasicSetting.MaxWorkers
	state.Unlock()
	pushLog("system", "config imported")
	return ok(cfg)
}

func ListAccountsJSON() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	list, err := service.ListAccounts(service.NewTaskManager(nil))
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	state.Lock()
	defer state.Unlock()
	for i := range list {
		if task, ok := state.tasks[list[i].UID]; ok && task.State == "running" {
			list[i].IsRunning = true
		}
	}
	return ok(list)
}

func AddAccountJSON(accountJSON string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	var req service.AccountReq
	if err := json.Unmarshal([]byte(accountJSON), &req); err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}
	if err := service.AddAccount(req); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok("ok")
}

func UpdateAccountJSON(accountJSON string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	var req service.AccountReq
	if err := json.Unmarshal([]byte(accountJSON), &req); err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}
	if err := service.UpdateAccount(req); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok("ok")
}

func DeleteAccount(uid string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	if err := service.DeleteAccount(uid); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok("ok")
}

func ExportAccountsDBBase64() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	path, err := service.DBPath()
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(map[string]string{
		"fileName": "yatori-accounts.db",
		"mimeType": "application/octet-stream",
		"base64":   base64.StdEncoding.EncodeToString(data),
	})
}

func ImportAccountsDBBase64(encoded string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fail(CodeInvalidArgument, "璐﹀彿鏁版嵁搴?Base64 瑙ｆ瀽澶辫触: "+err.Error())
	}
	state.Lock()
	cacheDir := filepath.Join(state.dataDir, "cache")
	state.Unlock()
	_ = os.MkdirAll(cacheDir, 0755)
	tmp, err := os.CreateTemp(cacheDir, "yatori-accounts-*.db")
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fail(CodeInternalError, err.Error())
	}
	if err := tmp.Close(); err != nil {
		return fail(CodeInternalError, err.Error())
	}
	source, err := gorm.Open(sqlite.Open(tmpPath), &gorm.Config{})
	if err != nil {
		return fail(CodeInvalidArgument, "鎵撳紑璐﹀彿鏁版嵁搴撳け璐? "+err.Error())
	}
	var rows []service.AccountPO
	if err := source.Find(&rows).Error; err != nil {
		return fail(CodeInvalidArgument, "璇诲彇璐﹀彿鏁版嵁搴撳け璐? "+err.Error())
	}
	summary, err := service.ImportAccountPOs(rows)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	pushLog("system", fmt.Sprintf("accounts imported: new=%d update=%d skip=%d", summary.Imported, summary.Updated, summary.Skipped))
	return ok(summary)
}

func GetCoursesJSON(uid string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	courses, err := service.GetCourses(uid)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(courses)
}

func StartTask(uid string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	if uid == "" {
		return fail(CodeInvalidArgument, "uid cannot be empty")
	}
	po, err := service.GetAccountPO(uid)
	if err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}

	state.Lock()
	if task, ok := state.tasks[uid]; ok && task.State == "running" {
		state.Unlock()
		return fail(CodeInvalidArgument, "task is already running")
	}
	running := 0
	for _, task := range state.tasks {
		if task.State == "running" {
			running++
		}
	}
	if running >= state.maxWorkers {
		limit := state.maxWorkers
		state.Unlock()
		return fail(CodeInvalidArgument, fmt.Sprintf("running task limit reached: %d", limit))
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.cancels[uid] = cancel
	state.tasks[uid] = taskStatus{
		UID:       uid,
		Account:   po.Account,
		Platform:  po.AccountType,
		State:     "running",
		StartTime: time.Now().Format("2006-01-02 15:04:05"),
		LastLog:   logLine("system", "task started"),
	}
	state.logs = appendLimited(state.logs, logLine("system", "task started uid="+uid))
	state.Unlock()

	go runMobileTask(ctx, po)
	return ok("ok")
}

func StopTask(uid string) string {
	if uid == "" {
		return fail(CodeInvalidArgument, "uid cannot be empty")
	}
	state.Lock()
	if cancel, ok := state.cancels[uid]; ok {
		cancel()
		delete(state.cancels, uid)
	}
	task := state.tasks[uid]
	task.UID = uid
	task.State = "stopped"
	task.LastLog = logLine("system", "task stopped")
	state.tasks[uid] = task
	state.logs = appendLimited(state.logs, logLine("system", "task stopped uid="+uid))
	state.Unlock()
	return ok("ok")
}

func GetTaskStatusesJSON() string {
	state.Lock()
	defer state.Unlock()
	tasks := make([]taskStatus, 0, len(state.tasks))
	for _, task := range state.tasks {
		tasks = append(tasks, task)
	}
	return ok(tasks)
}

func GetRecentLogsJSON(limit int64) string {
	state.Lock()
	defer state.Unlock()
	if limit <= 0 || int(limit) >= len(state.logs) {
		cp := append([]string(nil), state.logs...)
		return ok(cp)
	}
	cp := append([]string(nil), state.logs[len(state.logs)-int(limit):]...)
	return ok(cp)
}

func ListXXTExamCodeRequestsJSON() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	list, err := service.ListPendingXXTExamCodeRequests()
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(list)
}

func AnswerXXTExamCodeRequest(id, code string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	if err := service.AnswerXXTExamCodeRequest(id, code); err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}
	return ok("ok")
}

func CancelXXTExamCodeRequest(id string) string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	if err := service.CancelXXTExamCodeRequest(id); err != nil {
		return fail(CodeInvalidArgument, err.Error())
	}
	return ok("ok")
}

func TestAIJSON() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	cfg, err := service.LoadConfig(currentConfigPath())
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	result, err := service.TestAI(cfg.Setting.AiSetting.AiUrl, cfg.Setting.AiSetting.Model, cfg.Setting.AiSetting.AiType, cfg.Setting.AiSetting.APIKey)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(result)
}

func TestQuestionBankJSON() string {
	if err := ensureInitialized(); err != nil {
		return fail(CodeNotInitialized, err.Error())
	}
	cfg, err := service.LoadConfig(currentConfigPath())
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	result, err := service.TestQuestionBank(cfg.Setting.ApiQueSetting)
	if err != nil {
		return fail(CodeInternalError, err.Error())
	}
	return ok(result)
}

func ensureInitialized() error {
	state.Lock()
	defer state.Unlock()
	if state.dataDir == "" {
		return errString("call Init(dataDir) first")
	}
	return nil
}

func currentConfigPath() string {
	state.Lock()
	path := state.configPath
	state.Unlock()
	if path != "" {
		return path
	}
	path, _ = service.DefaultConfigPath()
	return path
}

func runMobileTask(ctx context.Context, po service.AccountPO) {
	uid := po.UID
	emit := func(format string, args ...interface{}) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg := format
		if len(args) > 0 {
			msg = fmt.Sprintf(format, args...)
		}
		pushTaskLog(uid, msg)
	}
	defer func() {
		if r := recover(); r != nil {
			pushTaskLog(uid, fmt.Sprintf("worker panic: %v\n%s", r, debug.Stack()))
			finishTask(uid, "failed", fmt.Sprintf("%v", r))
		}
	}()

	cfg, err := service.LoadConfig(currentConfigPath())
	if err != nil {
		pushTaskLog(uid, "load config failed: "+err.Error())
		finishTask(uid, "failed", err.Error())
		return
	}
	setting := buildConsoleSetting(cfg)
	service.SetRuntimeQuestionBankSetting(cfg.Setting.ApiQueSetting)
	user := service.BuildUserFromPO(po)
	pushTaskLog(uid, fmt.Sprintf("login %s (%s)...", po.Account, po.AccountType))

	var runErr error
	switch po.AccountType {
	case "XUEXITONG":
		act := service.BuildActivity(po)
		if act == nil {
			runErr = fmt.Errorf("build XUEXITONG activity failed")
			break
		}
		if err := act.Login(); err != nil {
			runErr = err
			break
		}
		xxtCache, ok := act.GetUserCache().(*xuexitongApiPkg.XueXiTUserCache)
		if !ok || xxtCache == nil {
			runErr = fmt.Errorf("invalid XUEXITONG cache")
			break
		}
		examOptions := courseExamOptions(po)
		runErr = service.SafeRunWithExamOptions(ctx, setting, &user, xxtCache, examOptions, emit)
	case "YINGHUA", "CANGHUI":
		yhCache := &yinghuaApiPkg.YingHuaUserCache{
			PreUrl:   po.URL,
			Account:  po.Account,
			Password: service.DecodePassword(po.PasswordEnc),
		}
		if err := yinghua.YingHuaLoginAction(yhCache); err != nil {
			runErr = err
			break
		}
		runErr = service.SafeYingHuaRun(ctx, setting, &user, yhCache, emit)
	case "ENAEA":
		service.RunEnaea(setting, user, emit)
	case "QSXT":
		service.RunQsxt(setting, user, emit)
	case "CQIE":
		service.RunCqie(setting, user, emit)
	case "WELEARN":
		service.RunWeLearn(setting, user, emit)
	case "ICVE":
		randFail, submitThreshold := icveOptions(po)
		service.RunIcveWithCourseOpts(setting, user, randFail, submitThreshold, emit)
	case "HQKJ":
		service.RunHqkj(setting, user, emit)
	case "KETANGX":
		service.RunKetangx(setting, user, emit)
	default:
		runErr = fmt.Errorf("platform %s is not supported", po.AccountType)
	}
	if runErr != nil {
		pushTaskLog(uid, "task failed: "+runErr.Error())
		finishTask(uid, "failed", runErr.Error())
		return
	}
	select {
	case <-ctx.Done():
		finishTask(uid, "stopped", "")
	default:
		pushTaskLog(uid, "all tasks completed")
		finishTask(uid, "stopped", "")
	}
}

func buildConsoleSetting(cfg service.AppConfig) consoleConfig.Setting {
	return consoleConfig.Setting{
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
}

func courseExamOptions(po service.AccountPO) service.XXTExamOptions {
	submitThreshold := 100
	randomAnswerOnFail := 0
	var cc service.CoursesCustom
	if json.Unmarshal([]byte(po.CoursesCustom), &cc) == nil {
		if cc.SubmitThresholdPercent > 0 {
			submitThreshold = cc.SubmitThresholdPercent
		}
		randomAnswerOnFail = cc.RandomAnswerOnFail
	}
	return service.XXTExamOptions{
		SubmitThreshold:    submitThreshold,
		RandomAnswerOnFail: randomAnswerOnFail,
		AutoExamCode:       0,
		ExamCodes:          nil,
		AccountUID:         po.UID,
		AccountName:        po.Account,
	}
}

func icveOptions(po service.AccountPO) (int, int) {
	randFail := 0
	submitThreshold := 60
	var cc service.CoursesCustom
	if json.Unmarshal([]byte(po.CoursesCustom), &cc) == nil {
		randFail = cc.RandomAnswerOnFail
		if cc.SubmitThresholdPercent > 0 {
			submitThreshold = cc.SubmitThresholdPercent
		}
	}
	return randFail, submitThreshold
}

func pushLog(level, msg string) {
	state.Lock()
	defer state.Unlock()
	state.logs = appendLimited(state.logs, logLine(level, msg))
}

func pushTaskLog(uid, msg string) {
	state.Lock()
	defer state.Unlock()
	line := msg
	state.logs = appendLimited(state.logs, line)
	task := state.tasks[uid]
	task.LastLog = line
	state.tasks[uid] = task
}

func finishTask(uid, finalState, errMsg string) {
	state.Lock()
	defer state.Unlock()
	task := state.tasks[uid]
	task.State = finalState
	task.Error = errMsg
	state.tasks[uid] = task
	delete(state.cancels, uid)
}

func logLine(level, msg string) string {
	return "[" + time.Now().Format("2006-01-02 15:04:05") + "] [" + level + "] " + msg
}

func appendLimited(lines []string, line string) []string {
	lines = append(lines, line)
	if len(lines) > 1000 {
		return lines[len(lines)-1000:]
	}
	return lines
}

type errString string

func (e errString) Error() string { return string(e) }
