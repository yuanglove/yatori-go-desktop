package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"yatori-go-desktop/service"
)

// --- 非泛型 DTO，Wails bindgen 能正确处理 ---

type BoolResult struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type StringResult struct {
	Ok    bool   `json:"ok"`
	Data  string `json:"data"`
	Error string `json:"error,omitempty"`
}

type ConfigResult struct {
	Ok    bool              `json:"ok"`
	Data  service.AppConfig `json:"data"`
	Error string            `json:"error,omitempty"`
}

type AccountListResult struct {
	Ok    bool                `json:"ok"`
	Data  []service.AccountVO `json:"data"`
	Error string              `json:"error,omitempty"`
}

type TaskStatusListResult struct {
	Ok    bool                 `json:"ok"`
	Data  []service.TaskStatus `json:"data"`
	Error string               `json:"error,omitempty"`
}

type XXTExamCodeRequestListResult struct {
	Ok    bool                         `json:"ok"`
	Data  []service.XXTExamCodeRequest `json:"data"`
	Error string                       `json:"error,omitempty"`
}

type DashboardResult struct {
	Ok    bool              `json:"ok"`
	Data  service.Dashboard `json:"data"`
	Error string            `json:"error,omitempty"`
}

type StringListResult struct {
	Ok    bool     `json:"ok"`
	Data  []string `json:"data"`
	Error string   `json:"error,omitempty"`
}

type PlatformListResult struct {
	Ok    bool                   `json:"ok"`
	Data  []service.PlatformInfo `json:"data"`
	Error string                 `json:"error,omitempty"`
}

type UpdateInfo struct {
	HasUpdate      bool   `json:"hasUpdate"`
	LatestVersion  string `json:"latestVersion"`
	CurrentVersion string `json:"currentVersion"`
	URL            string `json:"url"`
}

type UpdateResult struct {
	Ok    bool       `json:"ok"`
	Data  UpdateInfo `json:"data"`
	Error string     `json:"error,omitempty"`
}

// --- App ---

type App struct {
	ctx        context.Context
	taskMgr    *service.TaskManager
	logHub     *service.LogHub
	configPath string
	icveCookie *service.ICVECookieCapture
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logHub = service.NewLogHub()
	a.icveCookie = service.NewICVECookieCapture()
	// 拦截 os.Stdout：把原核心 lg.Print 输出桥接到 LogHub + Wails Events
	a.logHub.HijackStdout(func(line string) {
		clean := service.ToUTF8(service.StripANSI(line))
		runtime.EventsEmit(a.ctx, "log:all", map[string]string{"uid": "_stdout", "msg": clean})
	})
	a.taskMgr = service.NewTaskManager(func(uid, msg string) {
		clean := service.ToUTF8(service.StripANSI(msg))
		a.logHub.Push(clean)
		runtime.EventsEmit(a.ctx, "log:"+uid, clean)
		runtime.EventsEmit(a.ctx, "log:all", map[string]string{"uid": uid, "msg": clean})
	})
	if path, err := service.DefaultConfigPath(); err == nil {
		a.configPath = path
		a.taskMgr.SetConfigPath(path)
		// 首次启动：确保 config.yaml 存在（LoadConfig 返回默认配置时写入磁盘）
		if cfg, e := service.LoadConfig(path); e == nil {
			_ = service.SaveConfig(path, cfg)
			a.taskMgr.SetMaxWorkers(cfg.Setting.BasicSetting.MaxWorkers)
		}
	}
	// 初始化数据库（失败时只记录，不阻止启动）
	_ = service.InitDB()
}

func (a *App) shutdown(_ context.Context) {
	a.taskMgr.StopAll()
	if a.icveCookie != nil {
		a.icveCookie.Stop()
	}
}

// --- 配置 ---

func (a *App) GetConfig() ConfigResult {
	cfg, err := service.LoadConfig(a.configPath)
	if err != nil {
		return ConfigResult{Error: err.Error()}
	}
	return ConfigResult{Ok: true, Data: cfg}
}

func (a *App) SaveConfig(cfg service.AppConfig) BoolResult {
	if a.configPath == "" {
		return BoolResult{Error: "配置路径未初始化，请检查 AppData 权限"}
	}
	if errs := service.ValidateConfig(cfg); len(errs) > 0 {
		return BoolResult{Error: errs[0]}
	}
	if err := service.SaveConfig(a.configPath, cfg); err != nil {
		return BoolResult{Error: err.Error()}
	}
	a.taskMgr.SetMaxWorkers(cfg.Setting.BasicSetting.MaxWorkers)
	return BoolResult{Ok: true}
}

func (a *App) GetDataDir() StringResult {
	dir, err := service.DataDir()
	if err != nil {
		return StringResult{Error: err.Error()}
	}
	return StringResult{Ok: true, Data: dir}
}

func (a *App) OpenDataDir() BoolResult {
	dir, err := service.DataDir()
	if err != nil {
		return BoolResult{Error: err.Error()}
	}
	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

func (a *App) ImportConfig() ConfigResult {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择 config.yaml",
		Filters: []runtime.FileFilter{{DisplayName: "YAML 文件", Pattern: "*.yaml;*.yml"}},
	})
	if err != nil || path == "" {
		return ConfigResult{Error: "用户取消"}
	}
	cfg, err := service.LoadConfig(path)
	if err != nil {
		return ConfigResult{Error: err.Error()}
	}
	return ConfigResult{Ok: true, Data: cfg}
}

func (a *App) ExportConfig(cfg service.AppConfig) BoolResult {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 config.yaml",
		DefaultFilename: "config.yaml",
		Filters:         []runtime.FileFilter{{DisplayName: "YAML 文件", Pattern: "*.yaml"}},
	})
	if err != nil || path == "" {
		return BoolResult{Error: "用户取消"}
	}
	if err := service.SaveConfig(path, cfg); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

// --- 账号 ---

func (a *App) ListAccounts() AccountListResult {
	list, err := service.ListAccounts(a.taskMgr)
	if err != nil {
		return AccountListResult{Error: err.Error()}
	}
	return AccountListResult{Ok: true, Data: list}
}

func (a *App) AddAccount(req service.AccountReq) BoolResult {
	if err := service.AddAccount(req); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

func (a *App) UpdateAccount(req service.AccountReq) BoolResult {
	if err := service.UpdateAccount(req); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

func (a *App) DeleteAccount(uid string) BoolResult {
	if err := service.DeleteAccount(uid); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

// --- 任务 ---

func (a *App) StartTask(uid string) BoolResult {
	if err := a.taskMgr.Start(uid); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

func (a *App) StopTask(uid string) BoolResult {
	a.taskMgr.Stop(uid)
	return BoolResult{Ok: true}
}

func (a *App) GetTaskStatuses() TaskStatusListResult {
	return TaskStatusListResult{Ok: true, Data: a.taskMgr.Statuses()}
}

func (a *App) ListXXTExamCodeRequests() XXTExamCodeRequestListResult {
	list, err := service.ListPendingXXTExamCodeRequests()
	if err != nil {
		return XXTExamCodeRequestListResult{Error: err.Error()}
	}
	return XXTExamCodeRequestListResult{Ok: true, Data: list}
}

func (a *App) AnswerXXTExamCodeRequest(id, code string) BoolResult {
	if err := service.AnswerXXTExamCodeRequest(id, code); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

func (a *App) CancelXXTExamCodeRequest(id string) BoolResult {
	if err := service.CancelXXTExamCodeRequest(id); err != nil {
		return BoolResult{Error: err.Error()}
	}
	return BoolResult{Ok: true}
}

// --- 仪表盘 ---

func (a *App) GetDashboard() DashboardResult {
	d, err := service.GetDashboard(a.taskMgr, a.configPath)
	if err != nil {
		return DashboardResult{Error: err.Error()}
	}
	return DashboardResult{Ok: true, Data: d}
}

// --- 日志 ---

func (a *App) TailLogFile(n int) StringListResult {
	// 优先返回内存缓冲（LogEmitter 写入的实时日志）
	mem := a.logHub.Recent(n)
	// 补充文件日志（yatori-go-core 写入的）
	file, _ := service.TailLog(n)
	all := append(file, mem...)
	if len(all) > n {
		all = all[len(all)-n:]
	}
	if all == nil {
		all = []string{}
	}
	return StringListResult{Ok: true, Data: all}
}

// GetRecentLogs 供日志页初次加载时拉取历史（内存缓冲 + 文件）
func (a *App) GetRecentLogs(n int) StringListResult {
	return a.TailLogFile(n)
}

// TestAIConfig 用当前配置发一个最小请求，验证 AI 是否可用
func (a *App) TestAIConfig() StringResult {
	cfg, err := service.LoadConfig(a.configPath)
	if err != nil {
		return StringResult{Error: "读取配置失败: " + err.Error()}
	}
	ai := cfg.Setting.AiSetting
	result, testErr := service.TestAI(ai.AiUrl, ai.Model, ai.AiType, ai.APIKey)
	if testErr != nil {
		return StringResult{Error: testErr.Error()}
	}
	return StringResult{Ok: true, Data: result}
}

func (a *App) TestQuestionBankConfig() StringResult {
	cfg, err := service.LoadConfig(a.configPath)
	if err != nil {
		return StringResult{Error: "读取配置失败: " + err.Error()}
	}
	result, testErr := service.TestQuestionBank(cfg.Setting.ApiQueSetting)
	if testErr != nil {
		return StringResult{Error: testErr.Error()}
	}
	return StringResult{Ok: true, Data: result}
}

// --- 平台 ---

func (a *App) GetPlatformSupport() PlatformListResult {
	return PlatformListResult{Ok: true, Data: service.PlatformSupportList()}
}

func (a *App) OpenURL(url string) BoolResult {
	if url == "" {
		return BoolResult{Error: "url 不能为空"}
	}
	if len(url) < 8 || (url[:7] != "http://" && url[:8] != "https://") {
		return BoolResult{Error: "仅允许 http:// 或 https:// 链接"}
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return BoolResult{Ok: true}
}

func (a *App) StartICVECookieCapture(url string) StringResult {
	if a.icveCookie == nil {
		a.icveCookie = service.NewICVECookieCapture()
	}
	opened, err := a.icveCookie.Start(url)
	if err != nil {
		return StringResult{Error: err.Error()}
	}
	return StringResult{Ok: true, Data: opened}
}

func (a *App) ReadICVECookie() StringResult {
	if a.icveCookie == nil {
		return StringResult{Error: "自动获取 Cookie 流程尚未启动"}
	}
	cookie, err := a.icveCookie.ReadCookie()
	if err != nil {
		return StringResult{Error: err.Error()}
	}
	return StringResult{Ok: true, Data: cookie}
}

type CourseListResult struct {
	Ok    bool               `json:"ok"`
	Data  []service.CourseVO `json:"data"`
	Error string             `json:"error,omitempty"`
}

func (a *App) GetCourses(uid string) CourseListResult {
	type result struct {
		data []service.CourseVO
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := service.GetCourses(uid)
		ch <- result{data: data, err: err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return CourseListResult{Error: r.err.Error()}
		}
		return CourseListResult{Ok: true, Data: r.data}
	case <-time.After(75 * time.Second):
		return CourseListResult{Error: "课程进度拉取超时，请稍后重试"}
	}
}

func (a *App) CheckForUpdates(currentVersion string) UpdateResult {
	latest, url, err := fetchLatestGitHubVersion()
	if err != nil {
		return UpdateResult{Error: err.Error()}
	}
	current := normalizeVersion(currentVersion)
	latestNorm := normalizeVersion(latest)
	return UpdateResult{Ok: true, Data: UpdateInfo{
		HasUpdate:      compareVersions(latestNorm, current) > 0,
		LatestVersion:  latestNorm,
		CurrentVersion: current,
		URL:            url,
	}}
}

func fetchLatestGitHubVersion() (string, string, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	status, err := getJSON(client, "https://api.github.com/repos/yuanglove/yatori-go-desktop/releases/latest", &release)
	if err == nil && release.TagName != "" {
		if release.HTMLURL == "" {
			release.HTMLURL = "https://github.com/yuanglove/yatori-go-desktop/releases"
		}
		return release.TagName, release.HTMLURL, nil
	}
	if err != nil && status != http.StatusNotFound {
		return "", "", err
	}

	var tags []struct {
		Name string `json:"name"`
	}
	_, err = getJSON(client, "https://api.github.com/repos/yuanglove/yatori-go-desktop/tags", &tags)
	if err != nil {
		return "", "", err
	}
	if len(tags) == 0 || tags[0].Name == "" {
		return "", "", fmt.Errorf("GitHub 暂无可用版本标签")
	}
	return tags[0].Name, "https://github.com/yuanglove/yatori-go-desktop/releases", nil
}

func getJSON(client *http.Client, url string, out any) (int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "yatori-go-desktop")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("无法连接 GitHub：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("解析 GitHub 响应失败：%w", err)
	}
	return resp.StatusCode, nil
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
}

func compareVersions(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av, bv := 0, 0
		if i < len(ap) {
			_, _ = fmt.Sscanf(ap[i], "%d", &av)
		}
		if i < len(bp) {
			_, _ = fmt.Sscanf(bp[i], "%d", &bv)
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}
