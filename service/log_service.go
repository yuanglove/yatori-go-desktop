package service

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const logBufSize = 1000

type LogHub struct {
	mu    sync.Mutex
	lines []string
}

func NewLogHub() *LogHub { return &LogHub{} }

// ansiRe 匹配所有 ANSI 转义序列
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func (h *LogHub) Push(msg string) {
	msg = toUTF8(stripANSI(msg))
	if strings.TrimSpace(msg) == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = append(h.lines, msg)
	if len(h.lines) > logBufSize {
		h.lines = h.lines[len(h.lines)-logBufSize:]
	}
}

func (h *LogHub) Recent(n int) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n <= 0 || n >= len(h.lines) {
		cp := make([]string, len(h.lines))
		copy(cp, h.lines)
		return cp
	}
	src := h.lines[len(h.lines)-n:]
	cp := make([]string, len(src))
	copy(cp, src)
	return cp
}

// HijackStdout 重定向 os.Stdout，按行过滤后推入 hub 并调用 emit。
// 只保留含原核心特征词的行，过滤 Wails 框架噪音。
func (h *LogHub) HijackStdout(emit func(string)) {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stdout = w

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := toUTF8(scanner.Text())
			if isCoreLog(line) {
				h.Push(line)
				if emit != nil {
					emit(line)
				}
			}
		}
	}()
	// 注意：不提供 restore，桌面程序生命周期内持续拦截
	// 如需恢复，保存 os.Stdout 原值后调用 w.Close()
	_ = r // keep reference
}

// HijackWriter 返回一个写入 hub 的 io.Writer（备用方案）
func (h *LogHub) HijackWriter(emit func(string)) io.Writer {
	pr, pw := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			h.Push(line)
			if emit != nil {
				emit(line)
			}
		}
	}()
	return pw
}

// toUTF8 尝试把 GBK 编码的字符串转为 UTF-8；如果已是合法 UTF-8 则原样返回
func toUTF8(s string) string { return NormalizeLogText(s) }
func ToUTF8(s string) string { return NormalizeLogText(s) }

// NormalizeLogText 修复常见编码问题：
//  1. 非法 UTF-8（GBK 字节流被当 UTF-8）→ GBK decode
//  2. mojibake（UTF-8 字节被 Latin-1/Windows-1252 误读后存成 UTF-8）→ 逐 rune codepoint 还原
func NormalizeLogText(s string) string {
	s = stripANSI(s)

	// 已是合法 UTF-8：检查是否包含 mojibake 特征字符
	if utf8.ValidString(s) {
		// 尝试 Latin-1 反解：把每个 rune 的 codepoint 当字节，重新 UTF-8 decode
		if fixed := fixMojibake(s); fixed != s {
			return fixed
		}
		// CJK 字符被 GBK 误解码为其他 CJK 字符（UTF-8→GBK→UTF-8）
		if fixed := fixGBKasMojibake(s); fixed != s {
			return applyResidualMojibakeMap(fixed)
		}
		return s
	}

	// 非法 UTF-8：尝试 GBK decode
	out, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), s)
	if err == nil && utf8.ValidString(out) {
		return out
	}

	return strings.ToValidUTF8(s, "?")
}

// fixMojibake 把每个 rune 的 codepoint 当 Latin-1 字节，尝试还原中文。
// 覆盖两种情况：
//  1. GBK bytes 被 Latin-1 误读后存成 UTF-8（codepoints 均 ≤0xFF，还原字节后可 GBK decode）
//  2. GBK bytes 被 Latin-1 误读后存成 UTF-8，还原字节后直接是合法 UTF-8
func fixMojibake(s string) string {
	runes := []rune(s)
	raw := make([]byte, 0, len(runes))
	for _, r := range runes {
		if r > 0xFF {
			return s // 含非 Latin-1 字符，不是 mojibake
		}
		raw = append(raw, byte(r))
	}
	// 情况1：还原字节后是合法 UTF-8 且含 CJK
	if utf8.Valid(raw) {
		if result := string(raw); hasCJK(result) {
			return result
		}
	}
	// 情况2：还原字节后是 GBK，尝试 GBK decode
	out, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), string(raw))
	if err == nil && utf8.ValidString(out) && hasCJK(out) {
		return out
	}
	return s
}

// fixGBKasMojibake 修复 UTF-8 字节被当 GBK 解码后产生的乱码。
// 逐段处理 CJK 字符，尝试 GBK 编码后作为 UTF-8 解码。
func fixGBKasMojibake(s string) string {
	runes := []rune(s)
	var result strings.Builder
	changed := false
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r < 0x4E00 || r > 0x9FFF {
			result.WriteRune(r)
			i++
			continue
		}
		j := i
		for j < len(runes) && runes[j] >= 0x4E00 && runes[j] <= 0x9FFF {
			j++
		}
		chunk := string(runes[i:j])
		gbkBytes, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), chunk)
		if err == nil && utf8.ValidString(gbkBytes) && hasCJK(gbkBytes) && gbkBytes != chunk {
			result.WriteString(gbkBytes)
			changed = true
		} else {
			result.WriteString(chunk)
		}
		i = j
	}
	if !changed {
		return s
	}
	return result.String()
}

// applyResidualMojibakeMap 修复 fixGBKasMojibake 后残留的已知乱码词。
var residualMojibakeMap = map[string]string{
	"缁统": "系统",
}

func applyResidualMojibakeMap(s string) string {
	for bad, good := range residualMojibakeMap {
		s = strings.ReplaceAll(s, bad, good)
	}
	return s
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// isCoreLog 判断是否为原核心日志，过滤 Wails/Go runtime 噪音
func isCoreLog(line string) bool {
	for _, kw := range []string{
		"课程", "任务点", "视频", "学时", "提交", "学习", "登录",
		"拉取", "章节", "考试", "完成", "失败", "错误",
		"INFO", "DEBUG", "WARN", "ERROR", "[系统]",
	} {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

// TailLog 读取最新日志文件末尾 n 行
func TailLog(n int) ([]string, error) {
	logDir, err := LogDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(logDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var latest string
	for _, e := range entries {
		if !e.IsDir() {
			latest = filepath.Join(logDir, e.Name())
		}
	}
	if latest == "" {
		return []string{}, nil
	}
	return tailFile(latest, n)
}

func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, stripANSI(scanner.Text()))
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, scanner.Err()
}

func fixUTF8AsGBKMojibake(s string) string { return fixMojibake(s) }
func normalizeLogText(s string) string      { return NormalizeLogText(s) }
