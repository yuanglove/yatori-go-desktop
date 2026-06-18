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
	profileDir := filepath.Join(dataDir, "icve-cookie-browser")
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

	cookies, err := readChromeCookies(port)
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", fmt.Errorf("未读取到 Cookie，请确认已在打开的浏览器窗口中完成登录并刷新页面")
	}

	sort.SliceStable(cookies, func(i, j int) bool {
		if cookies[i].Domain == cookies[j].Domain {
			return cookies[i].Name < cookies[j].Name
		}
		return cookies[i].Domain < cookies[j].Domain
	})
	parts := make([]string, 0, len(cookies))
	seen := map[string]bool{}
	for _, ck := range cookies {
		name := strings.TrimSpace(ck.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		parts = append(parts, name+"="+ck.Value)
	}
	cookieText := strings.Join(parts, "; ")
	if !strings.Contains(cookieText, "token=") {
		return "", fmt.Errorf("未检测到 token Cookie，请确认智慧职教已登录成功")
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

func readChromeCookies(port int) ([]cdpCookie, error) {
	target, err := waitForChromeTarget(port, 15*time.Second)
	if err != nil {
		return nil, err
	}
	ws, _, err := websocket.DefaultDialer.Dial(target.WebSocketDebuggerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("连接浏览器调试会话失败: %w", err)
	}
	defer ws.Close()

	if cookies, err := callGetCookies(ws, "Network.getAllCookies"); err == nil {
		return cookies, nil
	}
	return callGetCookies(ws, "Storage.getCookies")
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
