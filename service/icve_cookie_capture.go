package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultICVELoginURL = "https://www.icve.com.cn/"

type ICVECookieCapture struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	port        int
	userDataDir string
}

type chromeTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type browserSessionData struct {
	Cookies []cdpCookie
	Storage map[string]string
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func NewICVECookieCapture() *ICVECookieCapture {
	return &ICVECookieCapture{}
}

func (c *ICVECookieCapture) Start(loginURL string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if loginURL == "" {
		loginURL = defaultICVELoginURL
	}
	if !strings.HasPrefix(loginURL, "http://") && !strings.HasPrefix(loginURL, "https://") {
		return "", fmt.Errorf("登录地址必须以 http:// 或 https:// 开头")
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		c.cmd = nil
	}

	browser, err := findChromiumBrowser()
	if err != nil {
		return "", err
	}
	port, err := freeTCPPort()
	if err != nil {
		return "", err
	}
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	profileDir := filepath.Join(dataDir, "icve-cookie-browser", time.Now().Format("20060102150405"))
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时浏览器目录失败: %w", err)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"--user-data-dir=" + profileDir,
		loginURL,
	}
	cmd := exec.Command(browser, args...)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动浏览器失败: %w", err)
	}

	c.cmd = cmd
	c.port = port
	c.userDataDir = profileDir
	return loginURL, nil
}

func (c *ICVECookieCapture) ReadCookie() (string, error) {
	c.mu.Lock()
	port := c.port
	c.mu.Unlock()

	if port == 0 {
		return "", fmt.Errorf("尚未启动自动获取 Cookie 流程")
	}

	session, err := readChromeSession(port)
	if err != nil {
		return "", err
	}
	cookies := normalizeICVECookies(session.Cookies, session.Storage)
	if len(cookies) == 0 {
		return "", fmt.Errorf("未读取到 Cookie，请确认已在打开的浏览器窗口中完成登录并刷新页面")
	}

	sort.SliceStable(cookies, func(i, j int) bool {
		if cookieNamePriority(cookies[i].Name) != cookieNamePriority(cookies[j].Name) {
			return cookieNamePriority(cookies[i].Name) < cookieNamePriority(cookies[j].Name)
		}
		if cookies[i].Domain == cookies[j].Domain {
			return cookies[i].Name < cookies[j].Name
		}
		return cookies[i].Domain < cookies[j].Domain
	})
	parts := make([]string, 0, len(cookies))
	seen := map[string]bool{}
	for _, ck := range cookies {
		name := strings.TrimSpace(ck.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name + "|" + ck.Domain + "|" + ck.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, name+"="+ck.Value)
	}
	cookieText := strings.Join(parts, "; ")
	if !hasCookieName(cookies, "token") {
		return "", fmt.Errorf("未检测到 token Cookie，请确认智慧职教已登录成功；如果刚登录完成，请先刷新一次页面再读取")
	}
	return cookieText, nil
}

func (c *ICVECookieCapture) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	c.cmd = nil
	c.port = 0
}

func findChromiumBrowser() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("msedge.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chrome.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("未找到 Edge 或 Chrome，无法自动获取 Cookie")
}

func freeTCPPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func readChromeSession(port int) (browserSessionData, error) {
	target, err := waitForChromeTarget(port, 15*time.Second)
	if err != nil {
		return browserSessionData{}, err
	}
	ws, _, err := websocket.DefaultDialer.Dial(target.WebSocketDebuggerURL, nil)
	if err != nil {
		return browserSessionData{}, fmt.Errorf("连接浏览器调试会话失败: %w", err)
	}
	defer ws.Close()

	data := browserSessionData{Storage: map[string]string{}}
	if cookies, err := callGetCookies(ws, "Network.getAllCookies"); err == nil {
		data.Cookies = cookies
	} else {
		cookies, fallbackErr := callGetCookies(ws, "Storage.getCookies")
		if fallbackErr != nil {
			return browserSessionData{}, err
		}
		data.Cookies = cookies
	}
	if storage, err := callReadStorage(ws); err == nil {
		data.Storage = storage
	}
	return data, nil
}

func waitForChromeTarget(port int, timeout time.Duration) (chromeTarget, error) {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/json", port)
	var lastErr error
	for time.Now().Before(deadline) {
		targets, err := fetchChromeTargets(url)
		if err == nil {
			for _, t := range targets {
				if t.Type == "page" && t.WebSocketDebuggerURL != "" {
					return t, nil
				}
			}
		}
		lastErr = err
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		return chromeTarget{}, fmt.Errorf("等待浏览器启动超时: %w", lastErr)
	}
	return chromeTarget{}, fmt.Errorf("等待浏览器启动超时")
}

func fetchChromeTargets(url string) ([]chromeTarget, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("浏览器调试接口返回 %d", resp.StatusCode)
	}
	var targets []chromeTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func callGetCookies(ws *websocket.Conn, method string) ([]cdpCookie, error) {
	if err := ws.WriteJSON(map[string]any{"id": 1, "method": method}); err != nil {
		return nil, err
	}
	for {
		var resp cdpResponse
		if err := ws.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != 1 {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, resp.Error.Message)
		}
		var result struct {
			Cookies []cdpCookie `json:"cookies"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, err
		}
		return result.Cookies, nil
	}
}

func callReadStorage(ws *websocket.Conn) (map[string]string, error) {
	expr := `(() => {
		const out = {};
		for (const storeName of ['localStorage', 'sessionStorage']) {
			try {
				const store = window[storeName];
				for (let i = 0; i < store.length; i++) {
					const key = store.key(i);
					out[storeName + '.' + key] = store.getItem(key);
				}
			} catch (_) {}
		}
		return out;
	})()`
	if err := ws.WriteJSON(map[string]any{
		"id":     2,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expr,
			"returnByValue": true,
		},
	}); err != nil {
		return nil, err
	}
	for {
		var resp cdpResponse
		if err := ws.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != 2 {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("Runtime.evaluate: %s", resp.Error.Message)
		}
		var result struct {
			Result struct {
				Value map[string]string `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, err
		}
		if result.Result.Value == nil {
			return map[string]string{}, nil
		}
		return result.Result.Value, nil
	}
}

func normalizeICVECookies(cookies []cdpCookie, storage map[string]string) []cdpCookie {
	out := make([]cdpCookie, 0, len(cookies)+2)
	for _, ck := range cookies {
		if !isICVERelatedCookie(ck) {
			continue
		}
		out = append(out, ck)
	}
	for _, name := range []string{"token", "zhzj-Token"} {
		if hasCookieName(out, name) {
			continue
		}
		if value := findStorageToken(storage, name); value != "" {
			out = append(out, cdpCookie{Name: name, Value: value, Domain: ".icve.com.cn", Path: "/"})
		}
	}
	return out
}

func isICVERelatedCookie(ck cdpCookie) bool {
	domain := strings.ToLower(strings.TrimSpace(ck.Domain))
	return domain == "" || strings.Contains(domain, "icve.com.cn")
}

func hasCookieName(cookies []cdpCookie, name string) bool {
	for _, ck := range cookies {
		if strings.EqualFold(strings.TrimSpace(ck.Name), name) && strings.TrimSpace(ck.Value) != "" {
			return true
		}
	}
	return false
}

func findStorageToken(storage map[string]string, want string) string {
	want = strings.ToLower(want)
	for key, value := range storage {
		if strings.TrimSpace(value) == "" {
			continue
		}
		k := strings.ToLower(key)
		if strings.HasSuffix(k, "."+want) || strings.HasSuffix(k, "_"+want) || strings.HasSuffix(k, "-"+want) || k == want {
			return value
		}
	}
	return ""
}

func cookieNamePriority(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "token":
		return 90
	case "zhzj-token":
		return 91
	default:
		return 10
	}
}
