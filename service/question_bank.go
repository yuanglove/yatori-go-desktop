package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var runtimeQuestionBank struct {
	sync.RWMutex
	setting ApiQueSetting
}

func SetRuntimeQuestionBankSetting(setting ApiQueSetting) {
	runtimeQuestionBank.Lock()
	defer runtimeQuestionBank.Unlock()
	runtimeQuestionBank.setting = normalizeQuestionBankSetting(setting)
}

func GetRuntimeQuestionBankSetting(fallbackURL string) ApiQueSetting {
	runtimeQuestionBank.RLock()
	setting := runtimeQuestionBank.setting
	runtimeQuestionBank.RUnlock()
	if setting.Url == "" {
		setting.Url = fallbackURL
	}
	return normalizeQuestionBankSetting(setting)
}

type QuestionBankQuestion struct {
	Type       string
	Content    string
	OptionsMap map[string]string
	Options    []string
	CourseName string
}

type QuestionBankResult struct {
	Answers []string
	Raw     string
}

type QuestionBankPreset struct {
	Protocol        string `json:"protocol"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Method          string `json:"method"`
	ContentType     string `json:"contentType"`
	TokenParam      string `json:"tokenParam"`
	AuthType        string `json:"authType"`
	QuestionField   string `json:"questionField"`
	TypeField       string `json:"typeField"`
	OptionsField    string `json:"optionsField"`
	CourseNameField string `json:"courseNameField"`
	OptionsFormat   string `json:"optionsFormat"`
	TypeMap         string `json:"typeMap"`
	AnswerPath      string `json:"answerPath"`
	AnswerSplit     string `json:"answerSplit"`
}

func QuestionBankPresets() []QuestionBankPreset {
	return []QuestionBankPreset{
		{Protocol: "yatori", Name: "Yatori 原协议（Yatori）", URL: "http://localhost:8083", Method: "POST", ContentType: "json", AuthType: "none", QuestionField: "content", TypeField: "type", OptionsField: "options", CourseNameField: "courseName", OptionsFormat: "array", AnswerPath: "question.answers", AnswerSplit: "#"},
		{Protocol: "wanjuan", Name: "万卷题库（AXE）", URL: "https://tk.wanjuantiku.com/api/query", Method: "POST", ContentType: "form", TokenParam: "token", AuthType: "form", QuestionField: "tm", TypeField: "type", OptionsField: "options", CourseNameField: "coursename", OptionsFormat: "text", TypeMap: "single=single,multiple=multiple,judge=judge,fill=completion,short=short", AnswerPath: "data.questions.0.answer", AnswerSplit: "#"},
		{Protocol: "zerror", Name: "ZE 题库（ZError）", URL: "https://api.zaizhexue.top/api/query", Method: "GET", ContentType: "json", TokenParam: "token", AuthType: "query", QuestionField: "title", TypeField: "type", OptionsField: "options", CourseNameField: "courseName", OptionsFormat: "text", AnswerPath: "data.data", AnswerSplit: "#"},
		{Protocol: "xhwlgzs", Name: "小杭题库（XHWL）", URL: "https://api.tiku.xhwlgzs.cn/v1/questions/search", Method: "POST", ContentType: "json", TokenParam: "Authorization", AuthType: "bearer", QuestionField: "value", TypeField: "type", OptionsField: "options", OptionsFormat: "object", TypeMap: "single=0,multiple=1,fill=2,judge=3,short=4", AnswerPath: "data.answer", AnswerSplit: "#"},
		{Protocol: "ocs", Name: "OCS 兼容接口（OCS Compatible）", URL: "", Method: "POST", ContentType: "json", TokenParam: "token", AuthType: "query", QuestionField: "title", TypeField: "type", OptionsField: "options", CourseNameField: "courseName", OptionsFormat: "array", AnswerPath: "data.answer", AnswerSplit: "#"},
		{Protocol: "custom", Name: "自定义接口（Custom）", URL: "", Method: "POST", ContentType: "json", TokenParam: "token", AuthType: "none", QuestionField: "title", TypeField: "type", OptionsField: "options", CourseNameField: "courseName", OptionsFormat: "array", AnswerPath: "answers", AnswerSplit: "#"},
	}
}

func TestQuestionBank(setting ApiQueSetting) (string, error) {
	setting = normalizeQuestionBankSetting(setting)
	q := QuestionBankQuestion{
		Type:       "单选题",
		Content:    "测试题：1+1 等于多少？",
		OptionsMap: map[string]string{"A": "1", "B": "2", "C": "3", "D": "4"},
		CourseName: "连通性测试",
	}
	result, err := QueryQuestionBank(setting, q)
	if err != nil {
		return "", err
	}
	if len(result.Answers) == 0 {
		return "", errors.New("接口可访问，但没有解析到答案，请检查协议、Key、答案路径或题库是否收录测试题")
	}
	return fmt.Sprintf("题库连接成功，协议=%s，解析答案=%s", setting.Protocol, strings.Join(result.Answers, ",")), nil
}

func QueryQuestionBank(setting ApiQueSetting, q QuestionBankQuestion) (QuestionBankResult, error) {
	setting = normalizeQuestionBankSetting(setting)
	switch strings.ToLower(setting.Protocol) {
	case "", "yatori":
		return queryCustomQuestionBank(setting, q, true)
	case "wanjuan", "wanxiang", "axe":
		return queryCustomQuestionBank(setting, q, false)
	case "zerror", "ze":
		return queryCustomQuestionBank(setting, q, false)
	case "xhwlgzs", "xhwlgz", "xiaohang":
		return queryCustomQuestionBank(setting, q, false)
	case "ocs", "custom":
		return queryCustomQuestionBank(setting, q, false)
	default:
		return QuestionBankResult{}, fmt.Errorf("不支持的题库协议: %s", setting.Protocol)
	}
}

func normalizeQuestionBankSetting(setting ApiQueSetting) ApiQueSetting {
	setting.Protocol = strings.TrimSpace(setting.Protocol)
	setting.Url = strings.TrimSpace(setting.Url)
	setting.Token = strings.TrimSpace(setting.Token)
	setting.TokenParam = strings.TrimSpace(setting.TokenParam)
	setting.AuthType = strings.ToLower(strings.TrimSpace(setting.AuthType))
	setting.Method = strings.ToUpper(strings.TrimSpace(setting.Method))
	setting.ContentType = strings.ToLower(strings.TrimSpace(setting.ContentType))
	setting.QuestionField = strings.TrimSpace(setting.QuestionField)
	setting.TypeField = strings.TrimSpace(setting.TypeField)
	setting.OptionsField = strings.TrimSpace(setting.OptionsField)
	setting.CourseNameField = strings.TrimSpace(setting.CourseNameField)
	setting.OptionsFormat = strings.ToLower(strings.TrimSpace(setting.OptionsFormat))
	setting.TypeMap = strings.TrimSpace(setting.TypeMap)
	setting.AnswerPath = strings.TrimSpace(setting.AnswerPath)
	setting.AnswerSplit = strings.TrimSpace(setting.AnswerSplit)
	// 从 URL 自动识别 xhwlgzs 协议，重置用户填写的 yatori 默认值
	if strings.Contains(setting.Url, "xhwlgzs.cn") && strings.EqualFold(setting.Protocol, "yatori") {
		setting.Protocol = "xhwlgzs"
		setting.Url = strings.Replace(setting.Url, "/questions/generate", "/questions/search", 1)
		setting.AuthType = ""
		setting.TokenParam = ""
		setting.QuestionField = ""
		setting.AnswerPath = ""
	}
	if setting.Protocol == "" {
		setting.Protocol = "yatori"
	}
	applyQuestionBankPresetDefaults(&setting)
	if setting.Method == "" {
		setting.Method = "POST"
	}
	if setting.ContentType == "" {
		setting.ContentType = "json"
	}
	if setting.AuthType == "" {
		setting.AuthType = "none"
	}
	if setting.TokenParam == "" {
		setting.TokenParam = "token"
	}
	if setting.QuestionField == "" {
		setting.QuestionField = "title"
	}
	if setting.TypeField == "" {
		setting.TypeField = "type"
	}
	if setting.OptionsField == "" {
		setting.OptionsField = "options"
	}
	if setting.CourseNameField == "" && setting.Protocol != "xhwlgzs" {
		setting.CourseNameField = "courseName"
	}
	if setting.OptionsFormat == "" {
		setting.OptionsFormat = "array"
	}
	if setting.AnswerPath == "" {
		if strings.EqualFold(setting.Protocol, "yatori") {
			setting.AnswerPath = "question.answers"
		} else {
			setting.AnswerPath = "answers"
		}
	}
	if setting.AnswerSplit == "" {
		setting.AnswerSplit = "#"
	}
	return setting
}

func applyQuestionBankPresetDefaults(setting *ApiQueSetting) {
	for _, preset := range QuestionBankPresets() {
		if !strings.EqualFold(setting.Protocol, preset.Protocol) {
			continue
		}
		if setting.Url == "" {
			setting.Url = preset.URL
		}
		if setting.Method == "" {
			setting.Method = preset.Method
		}
		if setting.ContentType == "" {
			setting.ContentType = preset.ContentType
		}
		if setting.TokenParam == "" {
			setting.TokenParam = preset.TokenParam
		}
		if setting.AuthType == "" {
			setting.AuthType = preset.AuthType
		}
		if setting.QuestionField == "" {
			setting.QuestionField = preset.QuestionField
		}
		if setting.TypeField == "" {
			setting.TypeField = preset.TypeField
		}
		if setting.OptionsField == "" {
			setting.OptionsField = preset.OptionsField
		}
		if setting.CourseNameField == "" {
			setting.CourseNameField = preset.CourseNameField
		}
		if setting.OptionsFormat == "" {
			setting.OptionsFormat = preset.OptionsFormat
		}
		if setting.TypeMap == "" {
			setting.TypeMap = preset.TypeMap
		}
		if setting.AnswerPath == "" {
			setting.AnswerPath = preset.AnswerPath
		}
		if setting.AnswerSplit == "" {
			setting.AnswerSplit = preset.AnswerSplit
		}
		return
	}
}

func queryCustomQuestionBank(setting ApiQueSetting, q QuestionBankQuestion, yatori bool) (QuestionBankResult, error) {
	if setting.Url == "" {
		return QuestionBankResult{}, errors.New("题库接口 URL 不能为空")
	}
	payload := buildQuestionBankPayload(setting, q)
	if yatori {
		payload = map[string]any{"question": payload}
	}
	raw, err := sendFlexibleQuestionBankRequest(setting, payload)
	if err != nil {
		return QuestionBankResult{}, err
	}
	answers := extractAnswersWithFallback(raw, setting)
	if len(answers) == 0 {
		return QuestionBankResult{Raw: raw}, questionBankParseError(raw)
	}
	return QuestionBankResult{Answers: answers, Raw: raw}, nil
}

func buildQuestionBankPayload(setting ApiQueSetting, q QuestionBankQuestion) map[string]any {
	payload := map[string]any{}
	setPayloadField(payload, setting.QuestionField, q.Content)
	setPayloadField(payload, setting.TypeField, mappedQuestionType(setting, q.Type))
	setPayloadField(payload, setting.OptionsField, formattedOptions(setting, q))
	if setting.CourseNameField != "" && q.CourseName != "" {
		setPayloadField(payload, setting.CourseNameField, q.CourseName)
	}
	if strings.EqualFold(setting.Protocol, "wanjuan") || strings.EqualFold(setting.Protocol, "wanxiang") || strings.EqualFold(setting.Protocol, "axe") {
		payload["answernum"] = "1"
		payload["questionnum"] = "1"
		payload["ai"] = "0"
	}
	return payload
}

func setPayloadField(payload map[string]any, field string, value any) {
	field = strings.TrimSpace(field)
	if field == "" {
		return
	}
	parts := strings.Split(field, ".")
	cur := payload
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
}

func sendFlexibleQuestionBankRequest(setting ApiQueSetting, payload map[string]any) (string, error) {
	headers := map[string]string{}
	endpoint := setting.Url
	applyAuth(setting, payload, &endpoint, headers)
	var body io.Reader
	contentType := ""
	if setting.Method == http.MethodGet {
		values := url.Values{}
		flattenPayload(values, "", payload)
		endpoint = appendQuery(endpoint, values)
	} else if setting.ContentType == "form" {
		values := url.Values{}
		flattenPayload(values, "", payload)
		body = strings.NewReader(values.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	return doQuestionBankRequest(setting.Method, endpoint, contentType, body, headers)
}

func applyAuth(setting ApiQueSetting, payload map[string]any, endpoint *string, headers map[string]string) {
	if setting.Token == "" {
		return
	}
	tokenName := setting.TokenParam
	if tokenName == "" {
		tokenName = "token"
	}
	switch setting.AuthType {
	case "query":
		values := url.Values{}
		values.Set(tokenName, setting.Token)
		*endpoint = appendQuery(*endpoint, values)
	case "form", "body", "json":
		payload[tokenName] = setting.Token
	case "bearer":
		headers[tokenName] = "Bearer " + setting.Token
	case "header":
		headers[tokenName] = setting.Token
	}
}

func doQuestionBankRequest(method, endpoint, contentType string, body io.Reader, headers map[string]string) (string, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "yatori-go-desktop")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("题库请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	raw := string(data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw, fmt.Errorf("题库接口返回 HTTP %d: %s", resp.StatusCode, abbreviate(raw, 180))
	}
	return raw, nil
}

func extractAnswersWithFallback(raw string, setting ApiQueSetting) []string {
	paths := []string{setting.AnswerPath, "question.answers", "answers", "answer", "data.answers", "data.answer", "data.data", "data.result.answer", "data.list.0.answer", "data.questions.0.answer", "result.answer", "result.answers"}
	for _, path := range paths {
		answers := extractAnswersFromJSON(raw, path, setting.AnswerSplit)
		if len(answers) > 0 {
			return answers
		}
	}
	return nil
}

func extractAnswersFromJSON(raw, path, split string) []string {
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil
	}
	value, ok := jsonPath(root, path)
	if !ok {
		return nil
	}
	return normalizeAnswers(value, split)
}

func jsonPath(root any, path string) (any, bool) {
	cur := root
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		switch typed := cur.(type) {
		case map[string]any:
			v, ok := typed[part]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, false
			}
			cur = typed[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func normalizeAnswers(value any, split string) []string {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeAnswers(item, split)...)
		}
		return compactAnswers(out)
	case []string:
		return compactAnswers(typed)
	case map[string]any:
		for _, key := range []string{"answer", "answers", "value", "content"} {
			if v, ok := typed[key]; ok {
				if answers := normalizeAnswers(v, split); len(answers) > 0 {
					return answers
				}
			}
		}
		return nil
	case string:
		if split == "" {
			split = "#"
		}
		parts := regexp.MustCompile(`[|;；、\n\r]+`).Split(typed, -1)
		if strings.Contains(typed, split) {
			parts = strings.Split(typed, split)
		}
		return compactAnswers(parts)
	case float64, bool:
		return []string{fmt.Sprint(typed)}
	default:
		return nil
	}
}

func compactAnswers(input []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func formattedOptions(setting ApiQueSetting, q QuestionBankQuestion) any {
	optionsMap := normalizedOptionMap(q)
	switch setting.OptionsFormat {
	case "object", "map":
		return optionsMap
	case "text":
		return strings.Join(optionValuesFromMap(optionsMap), "\n")
	case "values":
		values := make([]string, 0, len(optionsMap))
		keys := sortedKeys(optionsMap)
		for _, key := range keys {
			values = append(values, optionsMap[key])
		}
		return values
	default:
		return optionValuesFromMap(optionsMap)
	}
}

func normalizedOptionMap(q QuestionBankQuestion) map[string]string {
	if len(q.OptionsMap) > 0 {
		return q.OptionsMap
	}
	out := map[string]string{}
	for i, option := range q.Options {
		key := string(rune('A' + i))
		text := strings.TrimSpace(option)
		if len(text) >= 2 && text[1] == '.' && text[0] >= 'A' && text[0] <= 'Z' {
			key = string(text[0])
			text = strings.TrimSpace(text[2:])
		}
		out[key] = text
	}
	return out
}

func optionValues(q QuestionBankQuestion) []string {
	return optionValuesFromMap(normalizedOptionMap(q))
}

func optionValuesFromMap(optionsMap map[string]string) []string {
	keys := sortedKeys(optionsMap)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(optionsMap[key])
		if value == "" {
			continue
		}
		out = append(out, key+"."+value)
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mappedQuestionType(setting ApiQueSetting, t string) any {
	kind := canonicalQuestionKind(t)
	for _, item := range strings.Split(setting.TypeMap, ",") {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) != kind {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
		return value
	}
	return t
}

func canonicalQuestionKind(t string) string {
	low := strings.ToLower(strings.TrimSpace(t))
	switch {
	case strings.Contains(low, "多") || strings.Contains(low, "multiple"):
		return "multiple"
	case strings.Contains(low, "判") || strings.Contains(low, "true") || strings.Contains(low, "judge"):
		return "judge"
	case strings.Contains(low, "填") || strings.Contains(low, "fill") || strings.Contains(low, "completion"):
		return "fill"
	case strings.Contains(low, "简") || strings.Contains(low, "论") || strings.Contains(low, "名词") || strings.Contains(low, "short"):
		return "short"
	default:
		return "single"
	}
}

func wanjuanQuestionType(t string) string {
	kind := canonicalQuestionKind(t)
	switch kind {
	case "multiple":
		return "multiple"
	case "judge":
		return "judge"
	case "fill":
		return "completion"
	case "short":
		return "short"
	default:
		return "single"
	}
}

func flattenPayload(values url.Values, prefix string, payload map[string]any) {
	for k, v := range payload {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch typed := v.(type) {
		case map[string]any:
			flattenPayload(values, key, typed)
		case map[string]string:
			for mk, mv := range typed {
				values.Set(key+"["+mk+"]", mv)
			}
		case []string:
			values.Set(key, strings.Join(typed, "\n"))
		default:
			values.Set(key, fmt.Sprint(typed))
		}
	}
}

func questionBankParseError(raw string) error {
	var obj map[string]any
	if json.Unmarshal([]byte(raw), &obj) == nil {
		for _, key := range []string{"msg", "message", "error", "code", "status"} {
			if v, ok := obj[key]; ok && fmt.Sprint(v) != "" {
				return fmt.Errorf("题库响应中没有可用答案: %s=%v", key, v)
			}
		}
	}
	return fmt.Errorf("题库响应中没有可用答案: %s", abbreviate(raw, 180))
}

func appendQuery(endpoint string, values url.Values) string {
	if len(values) == 0 {
		return endpoint
	}
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	return endpoint + sep + values.Encode()
}

func abbreviate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "..."
}
