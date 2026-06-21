package service

import (
	"crypto/aes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	icveAction "github.com/yatori-dev/yatori-go-core/aggregation/icve"
	icveApi "github.com/yatori-dev/yatori-go-core/api/icve"
	coreUtils "github.com/yatori-dev/yatori-go-core/utils"
	consoleConfig "yatori-go-console/config"
)

const icveAESKey = "djekiytolkijduey"

// errNotInAnswerTime is returned when the platform rejects a save/submit because the exam window is closed.
var errNotInAnswerTime = fmt.Errorf("not in answer time")

// ---------- AES-ECB + PKCS7 ----------

func icveEncryptPayload(v interface{}) (string, error) {
	plain, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(icveAESKey))
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	pad := bs - len(plain)%bs
	padded := make([]byte, len(plain)+pad)
	copy(padded, plain)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	ct := make([]byte, len(padded))
	for i := 0; i < len(padded); i += bs {
		block.Encrypt(ct[i:i+bs], padded[i:i+bs])
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

func icveSubmitThresholdPercent(account string) int {
	const defaultThreshold = 60
	cfgPath, err := DefaultConfigPath()
	if err != nil {
		return defaultThreshold
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return defaultThreshold
	}
	for _, u := range cfg.Users {
		if strings.TrimSpace(u.Account) == strings.TrimSpace(account) && u.CoursesCustom.SubmitThresholdPercent > 0 {
			return u.CoursesCustom.SubmitThresholdPercent
		}
	}
	return defaultThreshold
}

// ---------- HTTP helpers ----------

func icvePOST(cache *icveApi.IcveUserCache, rawURL string, body string, contentType string, timeout time.Duration) (string, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if cache.IpProxySW && cache.ProxyIP != "" {
		tr.Proxy = func(r *http.Request) (*url.URL, error) { return url.Parse("http://" + cache.ProxyIP) }
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", coreUtils.DefaultUserAgent)
	req.Header.Set("Authorization", "Bearer "+cache.ZYKAccessToken)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://zyk.icve.com.cn/")
	req.Header.Set("Origin", "https://zyk.icve.com.cn")
	for _, c := range cache.Cookies {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 {
		return "", fmt.Errorf("token invalid or expired: 401")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return string(b), nil
}

// ---------- Structs ----------

type icveExamItem struct {
	Id           string  `json:"id"`
	ExamId       string  `json:"examId"`
	TaskId       string  `json:"taskId"`
	ExamName     string  `json:"examName"`
	Name         string  `json:"name"`
	Title        string  `json:"title"`
	CourseInfoId string  `json:"courseInfoId"`
	CourseId     string  `json:"courseId"`
	CategoryId   string  `json:"categoryId"`
	RecordId     string  `json:"recordId"`
	ResitId      string  `json:"resitId"`
	Status       string  `json:"status"`
	TypeId       string  `json:"typeId"`
	ExamStatus   int     `json:"examStatus"`
	Score        float64 `json:"score"`
	GroupId      int     `json:"groupId"`
	Source       string  `json:"-"`
}

type icveStudentCourseItem struct {
	Id             string `json:"id"`
	CourseId       string `json:"courseId"`
	CourseInfoId   string `json:"courseInfoId"`
	CourseName     string `json:"courseName"`
	Name           string `json:"name"`
	CourseInfoName string `json:"courseInfoName"`
	ProjectGroupId string `json:"projectGroupId"`
	Status         string `json:"status"`
}

func (c icveStudentCourseItem) displayName() string {
	if c.CourseName != "" {
		return c.CourseName
	}
	return c.Name
}

func (e icveExamItem) name() string {
	if e.ExamName != "" {
		return e.ExamName
	}
	if e.Name != "" {
		return e.Name
	}
	return e.Title
}

func (e icveExamItem) isDone() bool {
	return e.Status == "1" || e.Status == "2"
}

func icveExamKind(item icveExamItem, fallbackCategoryId string) string {
	name := item.name()
	cat := strings.TrimSpace(item.CategoryId)
	if cat == "" {
		cat = strings.TrimSpace(fallbackCategoryId)
	}
	typeId := strings.TrimSpace(item.TypeId)
	if icveNameContains(name, "作业", "课后作业") {
		return "work"
	}
	if icveNameContains(name, "测验", "测试") {
		if typeId == "1" {
			return "work"
		}
		return "quiz"
	}
	if icveNameContains(name, "考试", "期中考试", "期末考试") && (cat == "2" || cat == "3" || cat == "4") {
		return "exam"
	}
	// 标题含"考试"且 categoryId=2/3/4 时强制归 exam（优先于 typeId）
	if strings.Contains(name, "考试") && (cat == "2" || cat == "3" || cat == "4") {
		return "exam"
	}
	// typeId 判断
	if typeId == "1" || strings.Contains(typeId, "题库作业") {
		return "work"
	}
	if typeId == "2" || typeId == "3" || strings.Contains(typeId, "登分作业") || strings.Contains(typeId, "附件作业") {
		return "work"
	}
	// 标题关键字
	if strings.Contains(name, "考试") {
		return "exam"
	}
	if strings.Contains(name, "作业") {
		return "work"
	}
	// 测验/测试：categoryId=1 时归 work（网页"题库作业"标签），否则归 quiz
	if strings.Contains(name, "测验") || strings.Contains(name, "测试") {
		if typeId == "1" {
			return "work"
		}
		return "quiz"
	}
	switch cat {
	case "3", "4":
		return "exam"
	case "2":
		return "exam"
	case "1":
		return "work"
	default:
		return "work"
	}
}

func icveListItemKind(item icveExamItem, fallbackCategoryId, endpoint string) string {
	kind := icveExamKind(item, fallbackCategoryId)
	cat := strings.TrimSpace(item.CategoryId)
	if cat == "" {
		cat = strings.TrimSpace(fallbackCategoryId)
	}
	if endpoint == "/teacher/exam/answeredExamList" && cat == "3" {
		return "quiz"
	}
	return kind
}

func icveSwitchEnabled(v *int) bool {
	return v == nil || *v != 0
}

func icveNameContains(name string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}

func icveExamKindEnabled(kind string, cc consoleConfig.CoursesCustom) bool {
	switch kind {
	case "exam":
		return icveSwitchEnabled(cc.CxExamSw)
	case "quiz":
		return icveSwitchEnabled(cc.CxChapterTestSw)
	default:
		return icveSwitchEnabled(cc.CxWorkSw)
	}
}

func icveExamKindText(kind string) string {
	switch kind {
	case "exam":
		return "考试"
	case "quiz":
		return "测验"
	default:
		return "作业"
	}
}

type icveOptionItem struct {
	Label   string
	Content string
	Id      string
}

type icveExamProblemRecord struct {
	QuestionNo      int                    `json:"questionNo"`
	OptionSort      string                 `json:"optionSort"`
	Answer          string                 `json:"answer"`
	PaperId         string                 `json:"paperId"`
	Stem            string                 `json:"-"`
	QuestionType    string                 `json:"-"`
	OptionsMap      map[string]string      `json:"-"`
	Options         []icveOptionItem       `json:"-"`
	RawFields       map[string]interface{} `json:"-"`
	RecordAnswer    string                 `json:"-"`
	QuestionId      string                 `json:"-"`
	TypeNameRaw     string                 `json:"-"`
	ProblemRecordId string                 `json:"-"`
}

type icveDraftProblemItem struct {
	QuestionNo        int    `json:"questionNo"`
	OptionSort        string `json:"optionSort,omitempty"`
	Answer            string `json:"answer"`
	PaperId           string `json:"paperId"`
	KnowledgePointsId string `json:"knowledgePointsId,omitempty"`
	FileInfo          string `json:"fileInfo,omitempty"`
}

// icveHomeworkExamPayload mirrors the real browser payload for
// /teacher/homeworkExam/draft and /teacher/homeworkExam/add.
type icveHomeworkExamPayload struct {
	CategoryId                string                 `json:"categoryId"`
	CourseId                  string                 `json:"courseId"`
	CourseInfoId              string                 `json:"courseInfoId"`
	ExamId                    string                 `json:"examId"`
	ExamName                  string                 `json:"examName"`
	ExamTime                  int                    `json:"examTime"`
	GroupId                   string                 `json:"groupId"`
	IsLast                    bool                   `json:"isLast"`
	Status                    string                 `json:"status"`
	TaskExamProblemRecordList []icveDraftProblemItem `json:"taskExamProblemRecordList"`
	UpdateBy                  string                 `json:"updateBy"`
	UpdateTime                string                 `json:"updateTime"`
	UserId                    string                 `json:"userId"`
	Id                        string                 `json:"id"`
	ResitId                   string                 `json:"resitId"`
	Device                    int                    `json:"device"`
}

// buildICVEDraftProblemList 婵犵數鍋涢顓熸叏閹绢喗鐓€闁挎繂顦埀顒€鍊圭粋鎺斺偓锝庡亜閸擃喖顪冮妶鍡樷拻闁告鍥ㄥ€舵繛鍡樻尰閸嬬姵绻涢幋鐐靛ⅹ濠碘€茬矙閺岋絾鎯旈鑲╀哗濡?draft/add 婵犵數鍋為崹鍫曞箰婵犳艾绠板Δ锝呭暙閺?payload 闂傚倷绀侀幉锛勬暜濡ゅ懌鈧啯寰勯幇顑?// answer 闂備浇顕х€涒晝绮欓幒妞尖偓鍐幢濞戣鲸鏅╅悗鍏夊亾闁告洦鍋嗛ˇ銊モ攽閻愬弶顥滅紒缁樺姇椤?p.RecordAnswer闂傚倷鐒︾€笛呯矙閹达附鍋嬮煫鍥ㄧ☉閺嬩線鏌曢崼婵愭Ц缂佺媴缍侀弻褍顫濋鈧埀顒冨吹缁瑦绻濋崟顓狅紲濠电偞鍨堕悷锕偹夐弽顐ょ＜閺夊牄鍔岄崫娲煛娴ｅ摜肖濞寸媴绠撻幊鏍煛閸愨晛鏋撴繝鐢靛剳缁茶棄煤閿旂偓宕查柛鏇ㄥ灠濮规煡鏌嶈閸撴岸銆冮妷鈺傚€烽柟缁樺笚濮ｆ劙鎮峰鍛暭闁圭鍟块悾鐑藉閵堝棗浜滄繝闈涘€搁ˇ鎵礊韫囨洜纾藉ù锝呮贡閳洘鎱ㄦ繝鍌滅Ш闁?// questionNo 闂傚倸鍊搁崐绋课涘鍐ｆ灃婵炴垯鍨洪崑鍕煃瑜滈崜鐔煎蓟?,1,2...闂傚倷鐒︾€笛呯矙閹次层劑鍩€椤掑倻纾奸弶鍫涘妼瀵perId 婵犵數鍋炲娆撳触鐎ｎ喗鏅梻?RawFields["id"]
func buildICVEDraftProblemList(problems []icveExamProblemRecord) []icveDraftProblemItem {
	items := make([]icveDraftProblemItem, 0, len(problems))
	for i, p := range problems {
		item := icveDraftProblemItem{
			QuestionNo: i,
			OptionSort: p.OptionSort,
			Answer:     p.RecordAnswer,
			PaperId:    p.PaperId,
		}
		if p.RawFields != nil {
			if v, ok := p.RawFields["id"].(string); ok && v != "" {
				item.PaperId = v
			} else if p.QuestionId != "" {
				item.PaperId = p.QuestionId
			}
			// optionSort 婵犵數鍋炲娆撳触鐎ｎ喗鏅梻浣告啞钃辩紒瀣浮楠炲繘宕ㄩ娑樼／闂侀潧顭梽鍕枔?dataJson闂傚倷鐒︾€笛呯矙閹达附鍤愭い鏍仜閻鏌熼鍡忓亾闁?optionSort
			if v, ok := p.RawFields["dataJson"].(string); ok && v != "" {
				item.OptionSort = v
			} else if v, ok := p.RawFields["optionSort"].(string); ok && v != "" {
				item.OptionSort = v
			}
			for _, kk := range []string{"knowledgePointsId", "knowledgePointId", "knowledgePointsID"} {
				if v, ok := p.RawFields[kk].(string); ok && v != "" {
					item.KnowledgePointsId = v
					break
				}
			}
		}
		items = append(items, item)
	}
	return items
}

// MarshalJSON preserves raw platform fields and overlays recordAnswer.
func (r icveExamProblemRecord) MarshalJSON() ([]byte, error) {
	if len(r.RawFields) == 0 {
		type plain icveExamProblemRecord
		return json.Marshal(plain(r))
	}
	out := make(map[string]interface{}, len(r.RawFields)+2)
	for k, v := range r.RawFields {
		out[k] = v
	}
	if r.RecordAnswer != "" {
		out["recordAnswer"] = r.RecordAnswer
	}
	return json.Marshal(out)
}

type icveExamStartResp struct {
	Id                        string                  `json:"id"`
	ExamId                    string                  `json:"examId"`
	TaskId                    string                  `json:"taskId"`
	CourseInfoId              string                  `json:"courseInfoId"`
	CourseId                  string                  `json:"courseId"`
	CategoryId                string                  `json:"categoryId"`
	GroupId                   int                     `json:"groupId"`
	TaskExamProblemRecordList []icveExamProblemRecord `json:"taskExamProblemRecordList"`
	ProblemList               []icveExamProblemRecord `json:"problemList"`
	PaperQuestionList         []icveExamProblemRecord `json:"paperQuestionList"`
	Questions                 []icveExamProblemRecord `json:"questions"`
	Rows                      []icveExamProblemRecord `json:"rows"`
	List                      []icveExamProblemRecord `json:"list"`
}

func (r *icveExamStartResp) problems() []icveExamProblemRecord {
	for _, list := range [][]icveExamProblemRecord{
		r.TaskExamProblemRecordList,
		r.ProblemList,
		r.PaperQuestionList,
		r.Questions,
		r.Rows,
		r.List,
	} {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

type icveExamSubmitPayload struct {
	CategoryId                string                  `json:"categoryId"`
	CourseId                  string                  `json:"courseId"`
	CourseInfoId              string                  `json:"courseInfoId"`
	ExamId                    string                  `json:"examId"`
	ExamTime                  int                     `json:"examTime"`
	GroupId                   int                     `json:"groupId"`
	IsLast                    bool                    `json:"isLast"`
	Status                    string                  `json:"status"`
	TaskExamProblemRecordList []icveExamProblemRecord `json:"taskExamProblemRecordList"`
	UpdateBy                  string                  `json:"updateBy"`
	UpdateTime                string                  `json:"updateTime"`
	UserId                    string                  `json:"userId"`
	Id                        string                  `json:"id"`
	ResitId                   string                  `json:"resitId"`
	Device                    int                     `json:"device"`
}

// ---------- API helpers ----------

const icveBase = "https://zyk.icve.com.cn/prod-api"

func icveFetchListByEndpoint(cache *icveApi.IcveUserCache, courseId, courseInfoId, categoryId, flag, endpoint string) ([]icveExamItem, error) {
	items, _, err := icveFetchListByEndpointRaw(cache, courseId, courseInfoId, categoryId, flag, endpoint)
	return items, err
}

func icveFetchListByEndpointRaw(cache *icveApi.IcveUserCache, courseId, courseInfoId, categoryId, flag, endpoint string) ([]icveExamItem, string, error) {
	q := url.Values{}
	q.Set("courseId", courseId)
	q.Set("courseInfoId", courseInfoId)
	q.Set("categoryId", categoryId)
	if strings.Contains(endpoint, "/teacher/homeworkExam/") || strings.Contains(endpoint, "/teacher/exam/") {
		q.Set("pageNum", "1")
		q.Set("pageSize", "100")
		q.Set("flag", flag)
	}
	reqURL := icveBase + endpoint + "?" + q.Encode()
	raw, err := icveRetry(func() (string, error) {
		return icveGET(cache, reqURL, 10*time.Second)
	})
	if err != nil {
		return nil, "", err
	}
	items, err := icveParseListExam(raw)
	if err != nil {
		return nil, raw, err
	}
	if len(items) == 0 {
		// 诊断：检查 raw 里是否有 rows 但解析为 0
		var probe struct {
			Rows json.RawMessage `json:"rows"`
		}
		if json.Unmarshal([]byte(raw), &probe) == nil && len(probe.Rows) > 0 && string(probe.Rows) != "null" && string(probe.Rows) != "[]" {
			icveDebugLog("[ICVE][解析失败] raw has rows but parsed zero items endpoint=%s raw=%s", endpoint, abbreviate(raw, 300))
		}
	}
	source := "homework"
	if strings.Contains(endpoint, "/teacher/exam/") {
		source = "exam"
	}
	for i := range items {
		items[i].Source = source
	}
	return items, raw, nil
}

func icveParseListExam(raw string) ([]icveExamItem, error) {
	if items, ok := icveParseListExamDirect(raw); ok {
		return items, nil
	}
	return icveParseGenericList(raw, func(b []byte) ([]icveExamItem, error) {
		var v []icveExamItem
		return v, json.Unmarshal(b, &v)
	})
}

func icveParseListExamDirect(raw string) ([]icveExamItem, bool) {
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
		Rows json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, false
	}
	if env.Code != 0 && env.Code != 200 {
		return nil, false
	}
	for _, src := range []json.RawMessage{env.Rows, env.Data} {
		if len(src) == 0 || string(src) == "null" {
			continue
		}
		var items []icveExamItem
		if err := json.Unmarshal(src, &items); err == nil && len(items) > 0 {
			return items, true
		}
		if items, err := icveParseNestedGenericList(src, func(b []byte) ([]icveExamItem, error) {
			var v []icveExamItem
			return v, json.Unmarshal(b, &v)
		}); err == nil && len(items) > 0 {
			return items, true
		}
	}
	return nil, false
}

func icveFetchStudentCourses(cache *icveApi.IcveUserCache) ([]icveStudentCourseItem, error) {
	endpoints := []string{
		"/teacher/courseList/myCourseList",
		"/teacher/courseInfoStudent/myCourseList",
	}
	flags := []string{"1", "0", "2", "3"}
	seen := map[string]bool{}
	var all []icveStudentCourseItem
	for _, ep := range endpoints {
		for _, flag := range flags {
			q := url.Values{}
			q.Set("pageSize", "100")
			q.Set("pageNum", "1")
			q.Set("flag", flag)
			reqURL := icveBase + ep + "?" + q.Encode()
			raw, err := icveRetry(func() (string, error) {
				return icveGET(cache, reqURL, 10*time.Second)
			})
			if err != nil {
				icveDebugLog("[ICVE][课程候选] 来源=%s flag=%s 失败: %s", ep, flag, err)
				continue
			}
			items, err := icveParseGenericList(raw, func(b []byte) ([]icveStudentCourseItem, error) {
				var v []icveStudentCourseItem
				return v, json.Unmarshal(b, &v)
			})
			if err != nil || len(items) == 0 {
				icveDebugLog("[ICVE][课程候选] 来源=%s flag=%s 数量=0", ep, flag)
				continue
			}
			icveDebugLog("[ICVE][课程候选] 来源=%s flag=%s 数量=%d", ep, flag, len(items))
			for _, item := range items {
				key := item.CourseId + "_" + item.CourseInfoId
				if seen[key] {
					continue
				}
				seen[key] = true
				icveDebugLog("[ICVE][候选课程] name=%s courseId=%s courseInfoId=%s courseInfoName=%s",
					item.displayName(), item.CourseId, item.CourseInfoId, item.CourseInfoName)
				all = append(all, item)
			}
		}
	}
	return all, nil
}

// icveProbeHomeworkCourse 从候选课程中探测哪个能拉到 categoryId=1 flag=1 的作业
// 返回匹配的课程项和作业数，找不到时返回 ok=false
func icveProbeHomeworkCourse(cache *icveApi.IcveUserCache, courseName, courseId string, candidates []icveStudentCourseItem) (icveStudentCourseItem, int, bool) {
	want := strings.TrimSpace(courseName)
	for _, c := range candidates {
		match := (c.CourseId != "" && c.CourseId == courseId) ||
			(want != "" && strings.TrimSpace(c.displayName()) == want) ||
			(want != "" && strings.Contains(c.CourseInfoName, want))
		if !match {
			continue
		}
		items, err := icveFetchListByEndpoint(cache, c.CourseId, c.CourseInfoId, "1", "1",
			"/teacher/homeworkExam/answeredExamList")
		n := 0
		if err == nil {
			n = len(items)
		}
		icveDebugLog("[ICVE][probe] courseId=%s courseInfoId=%s 作业数=%d err=%v", c.CourseId, c.CourseInfoId, n, err)
		if n > 0 {
			return c, n, true
		}
	}
	return icveStudentCourseItem{}, 0, false
}

func icveResolveStudentCourse(cache *icveApi.IcveUserCache, course icveAction.IcveCourse) (icveStudentCourseItem, bool, error) {
	items, err := icveFetchStudentCourses(cache)
	if err != nil {
		return icveStudentCourseItem{}, false, err
	}
	for _, item := range items {
		if item.CourseId != "" && item.CourseId == course.CourseId {
			return item, true, nil
		}
	}
	want := strings.TrimSpace(course.CourseName)
	for _, item := range items {
		if want != "" && strings.TrimSpace(item.displayName()) == want {
			return item, true, nil
		}
	}
	return icveStudentCourseItem{}, false, nil
}

func icveParseGenericList[T any](raw string, parse func([]byte) ([]T, error)) ([]T, error) {
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
		Rows json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err == nil {
		if env.Code != 0 && env.Code != 200 {
			if env.Msg != "" {
				return nil, fmt.Errorf("%s", env.Msg)
			}
			return nil, fmt.Errorf("闂傚倷娴囬～澶嬬娴犲纾块弶鍫亖娴滆绻涢幋鐐垫噭濞存嚎鍊濋弻锟犲磼濞戞﹩鍤嬬紓浣插亾闁?code=%d", env.Code)
		}
		for _, src := range []json.RawMessage{env.Data, env.Rows} {
			if len(src) == 0 || string(src) == "null" {
				continue
			}
			if items, err := parse(src); err == nil && len(items) > 0 {
				return items, nil
			}
			if items, err := icveParseNestedGenericList(src, parse); err == nil && len(items) > 0 {
				return items, nil
			}
		}
		return []T{}, nil
	}
	return parse([]byte(raw))
}

func icveParseNestedGenericList[T any](src json.RawMessage, parse func([]byte) ([]T, error)) ([]T, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"rows", "list", "records", "items", "data"} {
		if child, ok := obj[key]; ok && len(child) > 0 && string(child) != "null" {
			if items, err := parse(child); err == nil && len(items) > 0 {
				return items, nil
			}
			if items, err := icveParseNestedGenericList(child, parse); err == nil && len(items) > 0 {
				return items, nil
			}
		}
	}
	return nil, fmt.Errorf("no nested list")
}

var icveStartExamFn = func(cache *icveApi.IcveUserCache, exam icveExamItem) (*icveExamStartResp, string, error) {
	return icveStartExam(cache, exam)
}

func icveStartExam(cache *icveApi.IcveUserCache, exam icveExamItem) (*icveExamStartResp, string, error) {
	examId := exam.ExamId
	if examId == "" {
		examId = exam.Id
	}
	taskId := exam.Id
	if exam.TaskId != "" {
		taskId = exam.TaskId
	}
	if exam.Source == "exam" && exam.RecordId != "" {
		taskId = exam.RecordId
	}
	if taskId == "" {
		taskId = exam.ExamId
	}
	courseInfoId := exam.CourseInfoId
	groupId := strconv.Itoa(exam.GroupId)
	if groupId == "" {
		groupId = "0"
	}

	urls := []string{
		icveBase + "/teacher/taskExamProblemRecord/examRecordPaperList?examId=" + url.QueryEscape(examId) + "&taskId=" + url.QueryEscape(taskId) + "&groupId=" + url.QueryEscape(groupId),
	}
	if exam.Source == "exam" {
		urls = append([]string{
			icveBase + "/teacher/taskExamProblemRecord/examDraftRecordPaperList?examId=" + url.QueryEscape(examId) + "&taskId=" + url.QueryEscape(taskId) + "&groupId=" + url.QueryEscape(groupId),
		}, urls...)
	}
	urls = append(urls,
		icveBase+"/teacher/homeworkExam/"+url.PathEscape(examId),
		icveBase+"/teacher/homeworkExam/paper?homeworkId="+url.QueryEscape(examId)+"&courseInfoId="+url.QueryEscape(courseInfoId),
		icveBase+"/teacher/homeworkExam/start?homeworkId="+url.QueryEscape(examId)+"&courseInfoId="+url.QueryEscape(courseInfoId),
	)

	var lastRaw string
	var lastRec *icveExamStartResp
	for _, u := range urls {
		raw, err := icveGET(cache, u, 10*time.Second)
		if err != nil {
			continue
		}
		lastRaw = raw
		rec, err := icveParseStartResp(raw)
		if err != nil {
			continue
		}
		lastRec = rec
		if len(rec.problems()) > 0 {
			rec.TaskExamProblemRecordList = rec.problems()
			return rec, u, nil
		}
	}
	if lastRec != nil {
		// 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ挾鍋涢悵姗€姊洪柅鐐茶嫰婢у鈧娲忛崕鏌ュ箚閺傚簱鏀介柛顐亗缁辨ê鈹戦悙鏉戠仸闁瑰皷鏅滅粋宥呪攽鐎ｎ亜鐎┑顔筋焾濞夋稓绮堟径鎰厽闁哄啠鍋撳ù婊冪埣楠炴帒螖閳ь剟宕欓悩缁樼厵闂侇叏绠戦弸鐔兼煟?rec闂傚倷鐒︾€笛呯矙閹达附鍤愭い鏍ㄦ皑閺嗐倝鏌涢埄鍏╂垶鍒婇幘顔界厱鐟滃酣銆冭箛娑樻辈妞ゆ牜鍋為悡鐔兼煃閸濆嫸宸ラ柡瀣懇閹綊骞囬鍏肩亖缂備緡鍠楀畝鎼佸箖椤旈敮鍋撻棃娑欐喐妞わ腹鏅犲鍝勑ч崶褏浼囩紓渚囧櫍缁犳牠骞嗛崘顔藉亜闁稿繒鍘ф禒娲⒑闂堟侗鐓梻鍕閹€斥枎閹惧鍘甸梺鍓茬厛閸嬪懎鐣烽崟顖涚厽闁挎梻鏅晶鐢碘偓娈垮櫘閸撶喖銆佸鈧幃銏ゆ閳哄啰顦梻鍌氼煬閸嬪嫬煤閿曞倶鈧啴骞囬绛嬫綗?		lastRec.TaskExamProblemRecordList = lastRec.problems()
		return lastRec, lastRaw, nil
	}
	return nil, lastRaw, fmt.Errorf("start exam failed: no endpoint returned problems")
}

func icveParseStartResp(raw string) (*icveExamStartResp, error) {
	var env struct {
		Code int                `json:"code"`
		Msg  string             `json:"msg"`
		Data *icveExamStartResp `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		if rec, derr := icveParseStartRespDynamic(raw); derr == nil {
			return rec, nil
		}
		return nil, err
	}
	if env.Code != 0 && env.Code != 200 {
		msg := env.Msg
		if msg == "" {
			msg = fmt.Sprintf("code=%d", env.Code)
		}
		return nil, fmt.Errorf("闂傚倷娴囬～澶嬬娴犲纾块弶鍫亖娴滆绻涢幋娆忕仾闁绘挶鍎茬换娑㈠幢濡桨鍒婇梺? %s", msg)
	}
	if env.Data == nil {
		if rec, derr := icveParseStartRespDynamic(raw); derr == nil {
			return rec, nil
		}
		return nil, fmt.Errorf("start response data is empty")
	}
	typedProblems := env.Data.problems()
	if rec, derr := icveParseStartRespDynamic(raw); derr == nil && len(rec.problems()) > 0 {
		if len(typedProblems) == 0 || !icveProblemsHaveContent(typedProblems) || icveProblemsHaveContent(rec.problems()) {
			return rec, nil
		}
	}
	return env.Data, nil
}

func icveProblemsHaveContent(problems []icveExamProblemRecord) bool {
	for _, p := range problems {
		if icveLooksLikeQuestion(p) {
			return true
		}
	}
	return false
}

func icveParseStartRespDynamic(raw string) (*icveExamStartResp, error) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}
	rec := &icveExamStartResp{}
	if data, ok := root["data"].(map[string]interface{}); ok {
		fillICVEStartMeta(rec, data)
		if qs := findICVEQuestionList(data); len(qs) > 0 {
			rec.TaskExamProblemRecordList = qs
			return rec, nil
		}
	}
	fillICVEStartMeta(rec, root)
	if qs := findICVEQuestionList(root); len(qs) > 0 {
		rec.TaskExamProblemRecordList = qs
		return rec, nil
	}
	return rec, nil
}

func fillICVEStartMeta(rec *icveExamStartResp, m map[string]interface{}) {
	if rec.Id == "" {
		rec.Id = icveAnyString(m["id"])
	}
	if rec.ExamId == "" {
		rec.ExamId = icveAnyString(m["examId"])
	}
	if rec.TaskId == "" {
		rec.TaskId = icveAnyString(firstNonNil(m["taskId"], m["recordId"]))
	}
	if rec.CourseInfoId == "" {
		rec.CourseInfoId = icveAnyString(m["courseInfoId"])
	}
	if rec.CourseId == "" {
		rec.CourseId = icveAnyString(m["courseId"])
	}
	if rec.CategoryId == "" {
		rec.CategoryId = icveAnyString(m["categoryId"])
	}
	if rec.GroupId == 0 {
		rec.GroupId = icveAnyInt(m["groupId"])
	}
}

func findICVEQuestionList(v interface{}) []icveExamProblemRecord {
	switch t := v.(type) {
	case map[string]interface{}:
		candidates := []string{
			"taskExamProblemRecordList", "problemList", "paperQuestionList", "questions",
			"rows", "list", "records", "problemRecordList", "paperProblems", "items",
			"children",
		}
		for _, key := range candidates {
			if arr, ok := t[key].([]interface{}); ok {
				if qs := parseICVEQuestionArray(arr); len(qs) > 0 {
					return qs
				}
				if qs := findICVEQuestionList(arr); len(qs) > 0 {
					return qs
				}
			}
		}
		for _, key := range []string{"data", "result", "obj", "page", "paper", "exam", "homework"} {
			if child, ok := t[key]; ok {
				if qs := findICVEQuestionList(child); len(qs) > 0 {
					return qs
				}
			}
		}
	case []interface{}:
		if qs := parseICVEQuestionArray(t); len(qs) > 0 {
			return qs
		}
		for _, item := range t {
			if qs := findICVEQuestionList(item); len(qs) > 0 {
				return qs
			}
		}
	}
	return nil
}

// icveDebugLog prints diagnostic info to stderr for option-parsing diagnostics.
func icveDebugLog(format string, args ...interface{}) {
	if os.Getenv("YATORI_ICVE_DEBUG") != "1" {
		return
	}
	fmt.Printf("[ICVE-DBG] "+format+"\n", args...)
}

func parseICVEQuestionArray(arr []interface{}) []icveExamProblemRecord {
	out := make([]icveExamProblemRecord, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// diagnostic: print raw keys and snippet for first question
		if i == 0 {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			// 闂備浇顕х换鎺楀磻閻愯娲冀椤愶綆娼熼梺鐟邦嚟婵參宕戦幘鏂ユ婵炲棗绻愰～灞筋渻閵堝棙澶勯柛鐘冲哺楠炴劙骞掑Δ鈧悘鎶芥煠绾板崬澧诲ù鐓庢川缁?keys
			childKeys := map[string][]string{}
			for k, v := range m {
				if child, ok2 := v.(map[string]interface{}); ok2 {
					ck := make([]string, 0, len(child))
					for ck2 := range child {
						ck = append(ck, ck2)
					}
					sort.Strings(ck)
					childKeys[k] = ck
				}
			}
			if b, _ := json.Marshal(m); b != nil {
				snippet := string(b)
				if len(snippet) > 2000 {
					snippet = snippet[:2000] + "..."
				}
				icveDebugLog("[ICVE][闂備浇宕垫慨鏉懨洪妶澶嬫櫇闁靛鏅涢拑鐔兼煕?婵犵妲呴崑鎾跺緤妤ｅ啯鍋嬮柣妯荤湽閳ь剙鍊块幊鐐哄Ψ閿濆嫮鐩庨梺鑽ゅ█缂傛艾鈻嶉敐鍜冭€跨痪鎯х暥ys=%v 闂備浇顕х€涒晝绮欓幒妤€绀夐柡鍥ュ焺閺佸銇勯弮鍌氫壕閻庢鍓氶妵鍕晬閸曨剟妾畒s=%v raw=%s", keys, childKeys, snippet)
			}
		}
		fieldMap := icveQuestionFieldMap(m)
		stem := icveAnyString(firstNonNil(
			m["title"], m["content"], m["stem"],
			m["questionName"], m["name"], m["questionContent"],
			m["problemContent"], m["topic"], m["titleName"],
			fieldMap["title"], fieldMap["content"], fieldMap["stem"], fieldMap["question"],
			fieldMap["questionName"], fieldMap["name"], fieldMap["questionContent"],
			fieldMap["problemContent"], fieldMap["topic"], fieldMap["titleName"],
			m["question"],
		))
		if strings.HasPrefix(stem, "map[") || strings.HasPrefix(stem, "[map[") {
			stem = ""
		}
		qType := icveAnyString(firstNonNil(
			m["questionType"], m["type"], m["queType"], m["qtype"],
			m["questionTypeName"], m["typeName"], m["problemType"], m["itemType"],
			fieldMap["questionType"], fieldMap["type"], fieldMap["queType"], fieldMap["qtype"],
			fieldMap["questionTypeName"], fieldMap["typeName"], fieldMap["problemType"], fieldMap["itemType"],
		))
		optRaw := firstNonNil(
			m["optionSort"], m["optionList"], m["optionDTOList"], m["options"],
			m["answerList"], m["choiceList"], m["itemList"], m["choices"],
			m["optionJson"], m["questionOptions"], m["optionsList"], m["items"],
			m["dataJson"],
			fieldMap["optionSort"], fieldMap["optionList"], fieldMap["optionDTOList"], fieldMap["options"],
			fieldMap["answerList"], fieldMap["choiceList"], fieldMap["itemList"], fieldMap["choices"],
			fieldMap["optionJson"], fieldMap["questionOptions"], fieldMap["optionsList"], fieldMap["items"],
			fieldMap["dataJson"],
		)
		// if top-level option is a JSON string, re-parse it
		if s, isStr := optRaw.(string); isStr && len(s) > 2 && (s[0] == '[' || s[0] == '{') {
			var reArr []interface{}
			if json.Unmarshal([]byte(s), &reArr) == nil {
				optRaw = reArr
			}
		}
		// nested: look one and two levels deeper for option arrays
		if optRaw == nil {
			optFieldNames := []string{
				"optionSort", "optionList", "optionDTOList", "options",
				"answerList", "choiceList", "itemList", "choices",
				"optionJson", "questionOptions", "optionsList", "items",
			}
		outer:
			for _, v := range m {
				if child, ok2 := v.(map[string]interface{}); ok2 {
					for _, fn := range optFieldNames {
						if child[fn] != nil {
							optRaw = child[fn]
							break outer
						}
					}
					for _, v2 := range child {
						if gc, ok3 := v2.(map[string]interface{}); ok3 {
							for _, fn := range optFieldNames {
								if gc[fn] != nil {
									optRaw = gc[fn]
									break outer
								}
							}
						}
					}
				}
			}
		}
		// if nested option field is a JSON string, re-parse
		if s2, isStr2 := optRaw.(string); isStr2 && len(s2) > 2 && (s2[0] == '[' || s2[0] == '{') {
			var reArr2 []interface{}
			if json.Unmarshal([]byte(s2), &reArr2) == nil {
				optRaw = reArr2
			}
		}
		optStr := icveMarshalAny(optRaw)
		optMap := icveParseOptionsMap(optStr)

		// infer question type from structure when not explicit
		if qType == "" {
			answer := icveAnyString(firstNonNil(m["answer"], m["userAnswer"], m["studentAnswer"], fieldMap["answer"], fieldMap["userAnswer"], fieldMap["studentAnswer"]))
			stemLower := strings.ToLower(stem)
			if strings.Contains(stemLower, "[fill") || strings.Contains(stemLower, "fillbox") || strings.Contains(stemLower, "contenteditable") {
				qType = "fill"
			} else if len(optMap) == 0 && (answer == "\u6b63\u786e" || answer == "\u9519\u8bef" || answer == "true" || answer == "false" || answer == "0" || answer == "1") {
				qType = "judge"
			} else if len(optMap) > 0 {
				if strings.Contains(answer, ",") {
					qType = "multiple"
				} else {
					qType = "single"
				}
			}
		}

		optItems, _ := icveParseOptions(optStr)
		// 婵犵數鍎戠徊钘壝洪敂鐐床闁告洦鍨板Ч鏌ユ煃瑜滈崜鐔煎蓟濞戞鐔兼偂鎼淬垻鈧顪冮妶搴′簼闁告梹顨婇崺鈧い鎺嶈兌閳洘銇勯妸銉︻棦妞ゃ垺鐟ラ悾婵嬪礋椤愩値妲舵俊鐐€栭悧妤冪矙閹烘绠梺顒€绉甸悡銉︾箾閹寸倖鎴濓耿閹殿喗鍠愬Λ棰佽兌閸斿秶绱掗崒姘毙у┑鈥崇埣瀹曞崬顫滈崱妯炴繈姊虹拠鍙夋崳闁硅櫕鎹囧鐢割敆閸曨偅杈堥梺鎸庣箓椤︻垶鎮欐繝鍥ㄧ厽闁归偊鍘奸ˉ瀣煟閹邦剨鍔熺紒杈ㄦ尰閹峰懘宕烽鐔锋珣闂傚倷娴囪闁哄懐濮撮悾?		rawCopy := make(map[string]interface{}, len(m))
		rawCopy := make(map[string]interface{}, len(m))
		for k, v := range m {
			rawCopy[k] = v
		}
		typeNameRaw := qType
		normType := normalizeICVEQuestionType(qType, m, optMap)
		qId := icveAnyString(firstNonNil(m["questionId"], fieldMap["questionId"], m["problemId"], fieldMap["problemId"], m["id"]))
		probRecId := icveAnyString(firstNonNil(m["problemRecordId"], m["recordId"], m["studentRecordId"], fieldMap["problemRecordId"], fieldMap["recordId"], fieldMap["studentRecordId"]))
		icveDebugLog("[ICVE][parseQ %d] questionId=%s problemRecordId=%s paperId=%s rawKeys=%v",
			i+1, qId, probRecId,
			icveAnyString(firstNonNil(m["paperId"], m["paperQuestionId"])),
			icveMapKeys(m))
		q := icveExamProblemRecord{
			QuestionNo:      icveAnyInt(m["questionNo"]),
			OptionSort:      optStr,
			Answer:          icveAnyString(firstNonNil(m["answer"], m["userAnswer"], m["studentAnswer"], fieldMap["answer"], fieldMap["userAnswer"], fieldMap["studentAnswer"])),
			PaperId:         icveAnyString(firstNonNil(m["id"], m["paperId"], m["paperQuestionId"], m["questionId"], fieldMap["paperId"], fieldMap["paperQuestionId"], fieldMap["questionId"])),
			Stem:            stem,
			QuestionType:    normType,
			OptionsMap:      optMap,
			Options:         optItems,
			RawFields:       rawCopy,
			RecordAnswer:    icveAnyString(firstNonNil(m["recordAnswer"], m["userAnswer"], m["studentAnswer"], fieldMap["recordAnswer"], fieldMap["userAnswer"], fieldMap["studentAnswer"])),
			QuestionId:      qId,
			TypeNameRaw:     typeNameRaw,
			ProblemRecordId: probRecId,
		}
		if !icveLooksLikeQuestion(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

func icveQuestionFieldMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	var walk func(map[string]interface{}, int)
	walk = func(cur map[string]interface{}, depth int) {
		if depth > 3 {
			return
		}
		for k, v := range cur {
			if _, exists := out[k]; !exists {
				out[k] = v
			}
			switch child := v.(type) {
			case map[string]interface{}:
				walk(child, depth+1)
			}
		}
	}
	walk(m, 0)
	return out
}

func icveLooksLikeQuestion(q icveExamProblemRecord) bool {
	if strings.TrimSpace(q.Stem) != "" {
		return true
	}
	if len(q.Options) > 0 || len(q.OptionsMap) > 0 {
		return true
	}
	if strings.TrimSpace(q.QuestionType) != "" && strings.TrimSpace(q.OptionSort) != "" {
		return true
	}
	if strings.TrimSpace(q.QuestionId) != "" && strings.TrimSpace(q.OptionSort) != "" {
		return true
	}
	return false
}

// normalizeICVEQuestionType 闂備浇顕х换鎰崲閹邦儵娑樜旈崨顔惧弮闂佸啿鎼幊搴ㄦ儗濡ゅ懏鐓涘璺猴功娴犮垽鏌￠崒鍐ㄥ暊閺€鑺ャ亜閺冨倵鎷￠柛搴㈡⒐閵囧嫰鏁愰崟顓犵厯閻庤娲栧﹢閬嶅焵椤掑﹦鍒板褍娴风划鍫ュ礋椤愮喐顫嶉梺鐟扮仢閸燁偊鐛Δ鍛厸?闂傚倷娴囧銊╂倿閿曗偓椤灝顫滈埀顒勫箖?typeId/闂傚倷绀侀崥瀣ｉ幒鎾变粓闁归棿绀侀崙鐘绘煕閹伴潧鏋熼柡鍜佸墮椤法鎹勯搹鍦紘闂佺硶鏅濇慨鎾煡婢舵劕绠绘い鏍ㄧ煯婢规洖鈹戦悙鏉戠仸闁瑰憡鎮傞獮鏍敃閿曗偓閼稿綊鏌熺紒銏犳灍闁绘帡绠栭弻鏇熺節韫囨搩娲梺鎼炲妼閵堟悂寮诲☉銏犵婵犻潧娴傚Λ鐐烘⒑?// 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭? single / multiple / judge / fill / short / essay
func normalizeICVEQuestionType(qType string, m map[string]interface{}, optMap map[string]string) string {
	typeId := icveAnyInt(firstNonNil(m["typeId"], m["itemTypeId"], m["questionTypeId"]))
	switch typeId {
	case 1:
		return "single"
	case 2:
		return "multiple"
	case 3:
		return "fill"
	}
	kind := canonicalQuestionKind(qType)
	// 闂傚倷娴囬～澶嬬娴犲绀夌€广儱顦拑鐔兼煕閵夘喖澧紒鈧崱娑欑厪闁割偅绻冮ˉ鐐烘煟閹烘洘顥夐摶鐐烘煟濡も偓閻楃偤宕楅鍌滅＜闁告瑦锚瀹撳棝鏌＄仦鑺ュ櫤鐎垫澘瀚伴獮鍥煘閹傚閻庡箍鍎遍ˇ顖炴儗濡や降浜滈煫鍥ㄦ尰閸ｈ櫣绱?濠电姵顔栭崰妤冩崲閹邦喖绶ら柛褎顨呴悘?闂傚倸鍊烽悞锔锯偓绗涘洦鍋￠柕濞炬櫓閺?闂傚倷绀侀幉锟犳偡閿旂偓绠掔紓鍌氬€哥粔鎾敄婢跺鍤曟い鏃傚亾瀹曞鏌涘┑鍡楊伀缂佺姵甯″娲川婵犲倻鐟插┑鐘亾闁革富鍘界€?judge
	if kind == "single" && len(optMap) == 2 {
		isJudgeVal := func(s string) bool {
			switch strings.TrimSpace(strings.ToLower(s)) {
			case "\u6b63\u786e", "\u9519\u8bef", "\u5bf9", "\u9519", "true", "false":
				return true
			}
			return false
		}
		allJudge := true
		for _, v := range optMap {
			if !isJudgeVal(v) {
				allJudge = false
				break
			}
		}
		if allJudge {
			return "judge"
		}
	}
	return kind
}

// icveBuildFillAnswer 闂備浇顕х换鎰崲閹邦儵娑樜旈崨顔芥珳闂佺粯鍔楅弫鎼佹儗濮橆厾绠剧€瑰壊鍠曢幉楣冩煛閸♀晛鐏﹂柡宀嬬節瀹曟帒顫濋鐑嗘П婵犵數濮崑鎾绘煙濞堝灝鏋ゅ瑙勫▕閺岋繝宕橀敐鍛婵＄偑鍊戦崹鍦矓閻熸壆鏆﹂柣妯款嚙瀹告繈姊洪銊╂妞ゃ儱锕娲偡閹殿喗鎲奸梺鍛婃⒐濞茬喖寮婚敃鍌氱厸闁告侗鍘鹃悡?JSON 闂傚倷绀侀幖顐ょ矓閸洖鍌ㄧ憸蹇撐?// 婵犵數濮烽。浠嬪焵椤掆偓閸熷潡鍩€椤掆偓缂嶅﹪骞?origAnswer 闂佽楠稿﹢閬嶁€﹂崼婵愬殨閻犺桨缍?JSON 闂傚倷娴囧銊╂倿閿旂晫鐝堕柛鈩冪懃閸ㄦ繄鈧箍鍎遍ˇ浼村疾椤掑嫭鍊堕柣鎰硾娴滃湱绱掗悩杈╃煓闁?SortOrder 缂傚倸鍊搁崐鐑芥倿閿曞倸绠板┑鐘崇閸婂灚銇勯弽顐沪闁哄拋鍓熼幃姗€鎮欓懜娈挎濠电偞鍤崶銊у幐闂佹悶鍎崝宀勵敋濠婂應鍋?Content
func icveProblemRawDiag(p icveExamProblemRecord) string {
	keys := icveMapKeys(p.RawFields)
	snippet := ""
	if len(p.RawFields) > 0 {
		if b, err := json.Marshal(p.RawFields); err == nil {
			snippet = abbreviate(string(b), 1800)
		}
	}
	return fmt.Sprintf("keys=%v raw=%s", keys, snippet)
}

func icveBuildFillAnswer(origAnswer, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	origAnswer = strings.TrimSpace(origAnswer)
	if strings.HasPrefix(origAnswer, "[") {
		var arr []map[string]interface{}
		if json.Unmarshal([]byte(origAnswer), &arr) == nil && len(arr) > 0 {
			arr[0]["Content"] = text
			if b, err := json.Marshal(arr); err == nil {
				return string(b)
			}
		}
	}
	b, _ := json.Marshal([]map[string]interface{}{{"SortOrder": 0, "Content": text}})
	return string(b)
}

func firstNonNil(values ...interface{}) interface{} {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func icveAnyString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func icveAnyInt(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return 0
		}
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return int(f)
		}
	}
	return 0
}

func icveFirstNonZeroInt(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func icveMarshalAny(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// icveParseOptions 闂?optionSort JSON 闂備浇宕甸崰鎰版偡鏉堚晛绶ゅΔ锝呭暞閸婄敻鏌ら幁鎺戝姢闁?[]icveOptionItem闂傚倷鐒︾€笛呯矙閹达附鍤愭い鏍仜缁犳牠鏌熼幑鎰靛殭缂侇偄绉归弻鐔碱敍濞戞﹩妫嗙紓浣哄У閻擄繝寮?map[label]content
// icveParseOptions 闂備浇顕х换鎰崲閹邦儵娑樷攽鐎ｎ亞鍔﹀銈嗗坊閸嬫挻銇勯敂鐐毈濠?JSON 闂備浇宕甸崰鎰版偡鏉堚晛绶ゅΔ锝呭暞閸婄敻鏌ら幁鎺戝姢闁?[]icveOptionItem闂傚倷鐒︾€笛呯矙閹达附鍤愭い鏍仜缁犳牠鏌熼幑鎰靛殭缂侇偄绉归弻鐔碱敍濞戞﹩妫嗙紓浣哄У閻擄繝寮诲☉銏犖ㄧ憸宥嗙濠婂應鍋撳▓鍨灈妞ゎ厾鍏樺?map
// ICVE 婵犵數鍋犻幓顏嗙礊閳ь剚绻涙径瀣鐎?0-based 闂傚倷娴囧銊╂倿閿曗偓椤灝顫滈埀顒勫箖?label闂傚倷鐒︾€笛呯矙閹存繐鑰挎い锝呮▏el=0/SortOrder=0 闂備浇宕甸崑鐐电矙韫囨稑绀夐幖娣妼妗呭┑顔筋焾娴滎剛绮婚弮鈧妵鍕箣閿濆棛銆婂銈呯箰瀹曨剟鈥旈崘顔嘉ч柛銉厛娴犙呯磽娴ｄ粙鍝虹€光偓閹间礁鏋佺€广儱娲ｅ▽顏堟煠濞村娅囬柟鎻掔秺濮婃椽鎳濋悧鍫㈠涧闂佹悶鍔嶇换鍌炲煡婢舵劦鏁傞柛鏇㈡涧濞?0
func icveParseOptions(optionSort string) ([]icveOptionItem, map[string]string) {
	if optionSort == "" {
		return nil, nil
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(optionSort), &raw); err != nil || len(raw) == 0 {
		return nil, nil
	}
	labelFields := []string{"Label", "label", "SortOrder", "sortOrder", "sort", "order", "no", "key", "optionCode"}
	contentFields := []string{"Content", "content", "Text", "text", "Value", "value", "optionContent", "optionText", "title"}
	alphas := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	items := make([]icveOptionItem, 0, len(raw))
	m := make(map[string]string, len(raw)*2)
	for idx, o := range raw {
		numLabel := -1
		textLabel := ""
		for _, lf := range labelFields {
			v, ok := o[lf]
			if !ok || v == nil {
				continue
			}
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s == "" || s == "<nil>" {
				continue
			}
			if n, e2 := strconv.Atoi(s); e2 == nil {
				numLabel = n
			} else {
				textLabel = strings.ToUpper(s)
			}
			break
		}
		var numKey, alphaKey string
		if numLabel >= 0 {
			numKey = strconv.Itoa(numLabel)
			if numLabel < len(alphas) {
				alphaKey = string(alphas[numLabel])
			}
		} else if textLabel != "" {
			alphaKey = textLabel
			if len(textLabel) == 1 && textLabel[0] >= 'A' && textLabel[0] <= 'Z' {
				numKey = strconv.Itoa(int(textLabel[0] - 'A'))
			}
		} else {
			numKey = strconv.Itoa(idx)
			if idx < len(alphas) {
				alphaKey = string(alphas[idx])
			}
		}
		content := ""
		for _, cf := range contentFields {
			v, ok := o[cf]
			if !ok || v == nil {
				continue
			}
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				content = s
				break
			}
		}
		displayLabel := alphaKey
		if displayLabel == "" {
			displayLabel = numKey
		}
		// Id 闂備浇顕х€涒晝绮欓幒鏇熸噷闂備礁鎼Λ娑㈠窗閹捐鐒垫い鎺嶈兌閳洘銇勯妶鍕惈y闂傚倷鐒︾€笛呯矙閹寸偟闄勯柡鍐ㄥ€归?icveMapAnswerToOption 闂傚倷绀侀幉锟犲礉閺嶎厽鍋￠柕澶嗘櫅閻鏌涢埄鍐€掔€规挷绶氶弻娑㈠箛椤撶偟绁烽梻浣峰嵆娴滃爼寮婚敓鐘茬闁冲搫鍊归悗顖涚節閻㈤潧浠滄繛宸弮楠?		items = append(items, icveOptionItem{Label: displayLabel, Content: content, Id: numKey})
		items = append(items, icveOptionItem{Label: displayLabel, Content: content, Id: numKey})
		if numKey != "" {
			m[numKey] = content
		}
		if alphaKey != "" {
			m[alphaKey] = content
		}
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items, m
}

func icveParseOptionsMap(optionSort string) map[string]string {
	_, m := icveParseOptions(optionSort)
	return m
}

func icveRecordIdForSubmit(exam icveExamItem, rec *icveExamStartResp) (string, string) {
	if rec != nil && strings.TrimSpace(rec.Id) != "" {
		return strings.TrimSpace(rec.Id), "rec.Id"
	}
	if strings.TrimSpace(exam.RecordId) != "" {
		return strings.TrimSpace(exam.RecordId), "exam.RecordId"
	}
	if exam.Source == "exam" && strings.TrimSpace(exam.TaskId) != "" {
		return strings.TrimSpace(exam.TaskId), "exam.TaskId"
	}
	return "", "empty"
}

func icveDoExamRequest(cache *icveApi.IcveUserCache, exam icveExamItem, rec *icveExamStartResp, isLast bool) error {
	examId := exam.ExamId
	if examId == "" {
		examId = exam.Id
	}
	categoryId := exam.CategoryId
	if categoryId == "" {
		categoryId = "3"
	}
	recordId, idSource := icveRecordIdForSubmit(exam, rec)

	draftProblems := buildICVEDraftProblemList(rec.TaskExamProblemRecordList)

	payload := icveExamSubmitPayload{
		CategoryId:   categoryId,
		CourseId:     exam.CourseId,
		CourseInfoId: exam.CourseInfoId,
		ExamId:       examId,
		ExamTime:     120,
		GroupId:      exam.GroupId,
		IsLast:       isLast,
		UserId:       cache.UserId,
		Id:           recordId,
		ResitId:      exam.ResitId,
		Device:       1,
	}

	action := "save"
	if isLast {
		action = "submit"
	}
	icveDebugLog("[ICVE][%s] IsLast=%v problemCount=%d", action, isLast, len(rec.TaskExamProblemRecordList))
	for di, dp := range rec.TaskExamProblemRecordList {
		icveDebugLog("[ICVE][濡?d] questionId=%s paperId=%s typeName=%s normType=%s origAnswer=%q recordAnswer=%q optCount=%d",
			di+1, dp.QuestionId, dp.PaperId, dp.TypeNameRaw, dp.QuestionType, dp.Answer, dp.RecordAnswer, len(dp.Options))
	}
	if n := len(rec.TaskExamProblemRecordList); n > 0 {
		show := 3
		if show > n {
			show = n
		}
		if pb, err2 := json.Marshal(rec.TaskExamProblemRecordList[:show]); err2 == nil {
			icveDebugLog("[ICVE][%s payload闁?d濡増顩?%s", action, show, string(pb))
		}
	}

	// 婵犵數鍎戠徊钘壝洪敂鐐床闁稿瞼鍋為崑銈夋煏婵炵偓娅嗛柛?draft 闂傚倷娴囬～澶嬬娴犲纾块弶鍫亖娴滆绻涢幋娆忕仾闁哄拋鍓熼弻娑㈩敃閿濆洨鐣洪梺?id=recordId闂傚倷鐒︾€笛呯矙閹寸偟闄勯柡鍐ㄥ€荤粻鏂款熆閼搁潧濮囬悷?courseId/courseInfoId闂?	// 闂傚倷绀佸﹢杈╁垝椤栫偛绀夐柟鐑樺焾濞尖晠鏌ㄩ弴鐐测偓褰掑磻?add 闂傚倷娴囬～澶嬬娴犲纾块弶鍫亖娴滆绻涢幋娆忕仾闁哄拋鍓熼弻娑㈩敃閿濆洨鐣洪梺?courseId/courseInfoId闂傚倷鐒︾€笛呯矙閹寸偟闄勯柡鍐ㄥ€荤粻鏂款熆閼搁潧濮囬悷?id闂?	// 闂傚倷绀侀幖顐λ囬銏犵？闁肩⒈鍓濇慨铏亜閺囨浜鹃梺杞扮缁夋挳鍩㈡惔锝呯筏?闂傚倸鍊风欢锟犲磻閸曨倠娑樜旈崨顓犵暫?chunk-6caefd3a / app.66ec6775 濠电姷顣藉Σ鍛村垂椤忓牆鐒垫い鎺戝暞閻濐亪鏌?0c6f闂傚倷鐒︾€笛呯矙閹达附鍋╅柣?v"]/d["m"]
	var ep string
	if exam.Source == "exam" {
		if isLast {
			ep = icveBase + "/teacher/exam/record"
		} else {
			ep = icveBase + "/teacher/exam/record/draft"
		}
	} else if isLast {
		ep = icveBase + "/teacher/homeworkExam/add"
	} else {
		ep = icveBase + "/teacher/homeworkExam/draft"
	}
	// 闂傚倷鐒﹀鎸庣閻愬搫绐楅幖娣妽閸嬵亪鏌涢弴銊ョ仩缂佺媭鍣ｉ弻鐔煎箚瑜嶉弳閬嶆煟閿曗偓閻栧ジ寮婚妸銉㈡婵炲棙锚婵＄ 闂傚倷鐒﹂幃鍫曞磿闁秴绠规い鎰堕檮閸嬧晠鏌ｉ幇闈涘闁崇粯妫冮弻褑绠涢幘鍐茶緟闂佽桨绀佺€氫即寮婚妸銉㈡婵妫楅幃鐚籐ast 闂傚倷鐒﹂幃鍫曞磿闁秴绠规い鎰堕檮閸嬧晠鏌ｉ幇闈涘闁?true闂傚倷鐒︾€笛呯矙閹达附鍋嬮柛娑卞灡瀹曞弶鎱ㄥΟ璇差暢闁稿鎸搁埥澶娾枍椤撗傞偗妤犵偛绻戠€靛ジ寮堕幋鐙€鍞介梻浣哥秺閸嬪﹥绂嶅┑瀣埞濠㈣埖鍔栭悡鏇犳喐瀹ュ悿娲Χ婢跺牃鍋撻崨閭︽Щ濡炪値鍋侀崹鑺ヤ繆閹壆鐤€闁哄洨濯Σ?	payload.Id = ""
	payload.IsLast = true

	dp := icveHomeworkExamPayload{
		CategoryId:                payload.CategoryId,
		CourseId:                  payload.CourseId,
		CourseInfoId:              payload.CourseInfoId,
		ExamId:                    payload.ExamId,
		ExamName:                  exam.name(),
		ExamTime:                  payload.ExamTime,
		GroupId:                   strconv.Itoa(payload.GroupId),
		IsLast:                    payload.IsLast,
		Status:                    "",
		UpdateBy:                  "",
		UpdateTime:                "",
		UserId:                    "",
		Id:                        payload.Id,
		ResitId:                   payload.ResitId,
		Device:                    payload.Device,
		TaskExamProblemRecordList: draftProblems,
	}
	icveDebugLog("[ICVE][%s閻犲洭鏀遍惇鐧?endpoint=%s idSource=%s payload.id=%s recordId=%s recId=%s resitId=%s courseId=%s courseInfoId=%s groupId=%s userId=%q isLast=%v",
		action, ep, idSource, dp.Id, exam.RecordId, rec.Id, dp.ResitId, dp.CourseId, dp.CourseInfoId, dp.GroupId, dp.UserId, dp.IsLast)
	if len(draftProblems) > 0 {
		p0 := draftProblems[0]
		orig0 := rec.TaskExamProblemRecordList[0]
		icveDebugLog("[ICVE][%s payload] 濡? answer=%q origAnswer=%q recordAnswer=%q paperId=%s",
			action, p0.Answer, orig0.Answer, orig0.RecordAnswer, p0.PaperId)
	}

	enc, err := icveEncryptPayload(dp)
	if err != nil {
		return fmt.Errorf("加密请求失败: %w", err)
	}

	raw, lastErr := icvePOST(cache, ep, enc, "text/plain", 15*time.Second)
	if lastErr != nil {
		return fmt.Errorf("%s 请求失败: %w", action, lastErr)
	}
	icveDebugLog("[ICVE][%s闁告繂绉寸花鐬?raw=%s", action, abbreviate(raw, 500))
	var resp struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Status  *bool  `json:"status"`
		Success *bool  `json:"success"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("閻熸瑱绲鹃悗浠嬪传瀹ュ懐瀹夊鎯扮簿鐟? %w (raw: %s)", err, abbreviate(raw, 120))
	}
	if resp.Status != nil && !*resp.Status {
		msg := resp.Msg
		if msg == "" {
			msg = "status=false"
		}
		return fmt.Errorf("%s濠㈡儼绮剧憴? %s", action, msg)
	}
	if resp.Success != nil && !*resp.Success {
		msg := resp.Msg
		if msg == "" {
			msg = "success=false"
		}
		return fmt.Errorf("%s濠㈡儼绮剧憴? %s", action, msg)
	}
	if resp.Code != 0 && resp.Code != 200 {
		msg := resp.Msg
		if strings.Contains(msg, "\u4e0d\u5728\u4f5c\u7b54\u65f6\u95f4\u5185") || strings.Contains(msg, "\u4e0d\u5728\u8003\u8bd5\u65f6\u95f4") || strings.Contains(msg, "\u8003\u8bd5\u65f6\u95f4\u5df2\u8fc7") {
			return errNotInAnswerTime
		}
		if msg == "" {
			msg = fmt.Sprintf("code=%d", resp.Code)
		}
		return fmt.Errorf("%s濠㈡儼绮剧憴? %s", action, msg)
	}
	if !isLast && strings.TrimSpace(resp.Msg) != "" {
		rec.TaskId = strings.TrimSpace(resp.Msg)
		icveDebugLog("[ICVE][濠电儑绲藉ú锔炬崲閸岀偞鍋ら柕濞炬櫅娴肩姷鈧箍鍎遍幊鎰偓鐟般€?draft taskId=%s", rec.TaskId)
	}
	return nil
}

func icveVerifySaveEcho(cache *icveApi.IcveUserCache, exam icveExamItem, savedRec *icveExamStartResp, examLabel, account, courseName string, emit emitFn) error {
	verifyExam := exam
	if savedRec != nil && strings.TrimSpace(savedRec.TaskId) != "" {
		verifyExam.Id = strings.TrimSpace(savedRec.TaskId)
		verifyExam.TaskId = strings.TrimSpace(savedRec.TaskId)
	}
	verifyRec, _, vErr := icveStartExamFn(cache, verifyExam)
	if vErr != nil || verifyRec == nil {
		return fmt.Errorf("save verify reload failed: %v", vErr)
	}
	vProbs := verifyRec.problems()
	if len(vProbs) == 0 {
		return fmt.Errorf("save verify reload returned empty problems")
	}
	expected := map[string]string{}
	if savedRec != nil {
		for _, sp := range savedRec.TaskExamProblemRecordList {
			key := strings.TrimSpace(sp.PaperId)
			if key == "" {
				key = strings.TrimSpace(sp.QuestionId)
			}
			if key == "" && sp.RawFields != nil {
				key = icveAnyString(sp.RawFields["id"])
			}
			if key != "" && strings.TrimSpace(sp.RecordAnswer) != "" {
				expected[key] = strings.TrimSpace(sp.RecordAnswer)
			}
		}
	}
	echoed := 0
	for _, vp := range vProbs {
		key := strings.TrimSpace(vp.PaperId)
		if key == "" {
			key = strings.TrimSpace(vp.QuestionId)
		}
		if key == "" && vp.RawFields != nil {
			key = icveAnyString(vp.RawFields["id"])
		}
		echo := strings.TrimSpace(vp.RecordAnswer)
		if echo == "" {
			echo = strings.TrimSpace(vp.Answer)
		}
		if want := expected[key]; want != "" && echo == want {
			echoed++
			continue
		}
		if len(expected) == 0 && echo != "" {
			echoed++
		}
	}
	if echoed == 0 {
		if emit != nil {
			emit("%s", formatICVELog(account, courseName, examLabel,
				"save API returned success but answer was not echoed back"))
			vp0 := vProbs[0]
			emit("%s", formatICVELog(account, courseName, examLabel,
				fmt.Sprintf("save verify first problem: paperId=%s questionId=%s problemRecordId=%s answer=%q recordAnswer=%q taskId=%s",
					vp0.PaperId, vp0.QuestionId, vp0.ProblemRecordId, vp0.Answer, vp0.RecordAnswer, verifyExam.TaskId)))
		}
		return fmt.Errorf("save API returned success but answer was not echoed back")
	}
	if emit != nil {
		emit("%s", formatICVELog(account, courseName, examLabel,
			fmt.Sprintf("save verify: %d/%d answers echoed", echoed, len(vProbs))))
	}
	return nil
}

func icveSaveExamAnswers(cache *icveApi.IcveUserCache, exam icveExamItem, rec *icveExamStartResp) error {
	return icveDoExamRequest(cache, exam, rec, false)
}

func icveSubmitExam(cache *icveApi.IcveUserCache, exam icveExamItem, rec *icveExamStartResp) error {
	return icveDoExamRequest(cache, exam, rec, true)
}

// ---------- 闂傚倷鐒﹀鎸庣閻愬搫绐楅幖娣妽閸嬵亪鏌涢弴銊ュ濠殿垰鐡ㄧ换婵囩箾閹傚缂?闂傚倷绀侀崥瀣熆濮椻偓瀹曨垶骞庨懞銉ュ墾濡炪倖鐗楃划宀€鈧灚鐓￠弻銈嗘叏閹邦兘鍋撳Δ鍛槬闁绘绮悡鏇㈢叓閸パ屽剰闁告梹姘ㄩ幉?----------

func runICVERealExamWork(setting consoleConfig.Setting, user consoleConfig.User, cache *icveApi.IcveUserCache, course icveAction.IcveCourse, randomAnswerOnFail int, submitThresholdPercent int, emit emitFn) {
	account := strings.TrimSpace(user.Account)
	cc := user.CoursesCustom
	tag := fmt.Sprintf("ICVE[%s]", course.CourseName)
	courseId := course.CourseId
	courseInfoId := course.CourseInfoId
	var courseCandidates []icveStudentCourseItem
	if candidates, err := icveFetchStudentCourses(cache); err != nil {
		emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 网页课程期次查询失败：%s", tag, err.Error())))
	} else if courseCandidates = candidates; false {
	} else if probed, n, ok := icveProbeHomeworkCourse(cache, course.CourseName, courseId, candidates); ok {
		oldCourseId, oldCourseInfoId := courseId, courseInfoId
		if probed.CourseId != "" {
			courseId = probed.CourseId
		}
		if probed.CourseInfoId != "" {
			courseInfoId = probed.CourseInfoId
		}
		emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 命中作业课程期次：courseId=%s courseInfoId=%s 作业数=%d（原 courseId=%s courseInfoId=%s）",
			tag, courseId, courseInfoId, n, oldCourseId, oldCourseInfoId)))
	} else {
		// probe 未命中时：尝试名字匹配
		want := strings.TrimSpace(course.CourseName)
		matched := false
		for _, c := range candidates {
			if (c.CourseId != "" && c.CourseId == course.CourseId) ||
				(want != "" && strings.TrimSpace(c.displayName()) == want) {
				oldCourseId, oldCourseInfoId := courseId, courseInfoId
				if c.CourseId != "" {
					courseId = c.CourseId
				}
				if c.CourseInfoId != "" {
					courseInfoId = c.CourseInfoId
				}
				emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 使用网页课程期次（无作业probe）courseId=%s courseInfoId=%s（原 courseId=%s courseInfoId=%s）",
					tag, courseId, courseInfoId, oldCourseId, oldCourseInfoId)))
				matched = true
				break
			}
		}
		if !matched {
			emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 未探测到作业课程期次，沿用核心库参数", tag)))
		}
	}

	endpoints := []string{
		"/teacher/exam/answeredExamList",
		"/teacher/homeworkExam/answeredExamList",
		"/teacher/homeworkExam/getAnsweredExamList",
	}
	categoryIds := []string{"1", "2", "3", "4"}

	seen := map[string]bool{}
	var allHomework, allQuiz, allExam []icveExamItem
	type icveCourseQueryPair struct {
		CourseId     string
		CourseInfoId string
		Label        string
	}
	queryPairs := []icveCourseQueryPair{{CourseId: courseId, CourseInfoId: courseInfoId, Label: "selected"}}
	querySeen := map[string]bool{courseId + "_" + courseInfoId: true}
	wantCourseName := strings.TrimSpace(course.CourseName)
	for _, c := range courseCandidates {
		match := (c.CourseId != "" && c.CourseId == course.CourseId) ||
			(wantCourseName != "" && strings.TrimSpace(c.displayName()) == wantCourseName) ||
			(wantCourseName != "" && strings.Contains(c.CourseInfoName, wantCourseName))
		if !match || c.CourseId == "" || c.CourseInfoId == "" {
			continue
		}
		key := c.CourseId + "_" + c.CourseInfoId
		if querySeen[key] {
			continue
		}
		querySeen[key] = true
		queryPairs = append(queryPairs, icveCourseQueryPair{CourseId: c.CourseId, CourseInfoId: c.CourseInfoId, Label: c.displayName()})
	}
	emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 作业页查询参数 courseId=%s courseInfoId=%s", tag, courseId, courseInfoId)))

	for _, qp := range queryPairs {
		emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] list course pair label=%s courseId=%s courseInfoId=%s", tag, qp.Label, qp.CourseId, qp.CourseInfoId)))
		for _, ep := range endpoints {
			for _, catId := range categoryIds {
				for _, flag := range []string{"1", "2", "3", "0"} {
					items, raw, err := icveFetchListByEndpointRaw(cache, qp.CourseId, qp.CourseInfoId, catId, flag, ep)
					if err != nil {
						emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 拉取任务列表失败 endpoint=%s catId=%s flag=%s：%s", tag, ep, catId, flag, err.Error())))
						continue
					}
					emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 列表 endpoint=%s catId=%s flag=%s 返回 %d 个", tag, ep, catId, flag, len(items))))
					if len(items) == 0 && ep == "/teacher/homeworkExam/answeredExamList" && catId == "3" && flag == "1" {
						emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] trainList raw endpoint=%s catId=%s flag=%s raw=%s", tag, ep, catId, flag, abbreviate(raw, 500))))
					}
					for _, item := range items {
						emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 列表项 endpoint=%s catId=%s flag=%s source=%s name=%s id=%s examId=%s taskId=%s recordId=%s categoryId=%s typeId=%s groupId=%d status=%s",
							tag, ep, catId, flag, item.Source, item.name(), item.Id, item.ExamId, item.TaskId, item.RecordId, item.CategoryId, item.TypeId, item.GroupId, item.Status)))
						key := item.ExamId + "_" + item.Id + "_" + item.TypeId + "_" + item.name()
						if seen[key] {
							continue
						}
						seen[key] = true
						cat := item.CategoryId
						if cat == "" {
							cat = catId
						}
						item.CategoryId = cat
						if item.CourseId == "" {
							item.CourseId = qp.CourseId
						}
						if item.CourseInfoId == "" {
							item.CourseInfoId = qp.CourseInfoId
						}
						kind := icveListItemKind(item, catId, ep)
						itemName := item.name()
						if strings.Contains(itemName, "考试") || strings.Contains(itemName, "鑰冭瘯") {
							kind = "exam"
						} else if strings.Contains(itemName, "作业") || strings.Contains(itemName, "浣滀笟") {
							kind = "work"
						}
						if icveNameContains(itemName, "\u8003\u8bd5") {
							kind = "exam"
						} else if icveNameContains(itemName, "\u4f5c\u4e1a") {
							kind = "work"
						}
						if ep == "/teacher/exam/answeredExamList" && cat == "3" {
							kind = "quiz"
						}
						if !icveExamKindEnabled(kind, cc) {
							emit("%s", formatICVELog(account, course.CourseName, item.name(),
								fmt.Sprintf("[%s] 跳过%s：账号管理中对应开关已关闭", tag, icveExamKindText(kind))))
							continue
						}
						if kind == "exam" {
							allExam = append(allExam, item)
						} else if kind == "quiz" {
							allQuiz = append(allQuiz, item)
						} else {
							allHomework = append(allHomework, item)
						}
					}
				}
			}
		}

		emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("[%s] 作业=%d 测验=%d 考试=%d", tag, len(allHomework), len(allQuiz), len(allExam))))

	}

	all := append(append(allHomework, allQuiz...), allExam...)
	for _, exam := range all {
		examTag := fmt.Sprintf("%s[%s]", tag, exam.name())
		emit("%s", formatICVELog(account, course.CourseName, exam.name(),
			fmt.Sprintf("[%s] source=%s id=%s examId=%s taskId=%s recordId=%s courseId=%s courseInfoId=%s categoryId=%s typeId=%s examStatus=%d groupId=%d status=%s",
				examTag, exam.Source, exam.Id, exam.ExamId, exam.TaskId, exam.RecordId, exam.CourseId, exam.CourseInfoId, exam.CategoryId, exam.TypeId, exam.ExamStatus, exam.GroupId, exam.Status)))

		if exam.isDone() {
			emit("%s", formatICVELog(account, course.CourseName, exam.name(), fmt.Sprintf("[%s] already done, skip", examTag)))
			continue
		}

		rec, diagInfo, err := icveStartExam(cache, exam)
		if err != nil {
			emit("%s", formatICVELog(account, course.CourseName, exam.name(), fmt.Sprintf("[%s] start exam failed: %s", examTag, err.Error())))
			if diagInfo != "" {
				emit("%s", formatICVELog(account, course.CourseName, exam.name(), fmt.Sprintf("[%s] diagnostic response: %s", examTag, abbreviate(diagInfo, 300))))
			}
			continue
		}

		if diagInfo != "" {
			var diagEnv struct {
				Code      int    `json:"code"`
				Msg       string `json:"msg"`
				RequestId string `json:"requestId"`
			}
			if jsonErr := json.Unmarshal([]byte(diagInfo), &diagEnv); jsonErr == nil && diagEnv.Msg != "" {
				emit("%s", formatICVELog(account, course.CourseName, exam.name(), fmt.Sprintf("[%s] 妤犵偛鍟胯ぐ鎾箵閹邦喓浠?msg: %s (code=%d)", examTag, diagEnv.Msg, diagEnv.Code)))
			}
		}

		problems := rec.problems()
		if len(problems) == 0 {
			fields := icveTopLevelKeys(diagInfo)
			emit("%s", formatICVELog(account, course.CourseName, exam.name(),
				fmt.Sprintf("[%s] empty problems, fields=%s raw=%s", examTag, fields, abbreviate(diagInfo, 200))))
			continue
		}

		emit("%s", formatICVELog(account, course.CourseName, exam.name(),
			fmt.Sprintf("[%s] loaded %d problems", examTag, len(problems))))
		emit("%s", formatICVELog(account, course.CourseName, exam.name(),
			fmt.Sprintf("[%s] rec.Id=%s taskId=%s examId=%s", examTag, rec.Id, rec.TaskId, exam.ExamId)))
		for idx, p := range problems {
			if strings.TrimSpace(p.Stem) == "" && len(p.Options) == 0 {
				emit("%s", formatICVELog(account, course.CourseName, exam.name(),
					fmt.Sprintf("[%s] problem %d parse empty: %s", examTag, idx+1, icveProblemRawDiag(p))))
				break
			}
		}
		emit("%s", formatICVELog(account, course.CourseName, icveExamLabel(exam),
			fmt.Sprintf("ICVE settings: autoExam=%d examAutoSubmit=%d randomAnswerOnFail=%d threshold=%d",
				cc.AutoExam, cc.ExamAutoSubmit, randomAnswerOnFail, submitThresholdPercent)))
		if cc.AutoExam == 0 {
			emit("%s", formatICVELog(account, course.CourseName, icveExamLabel(exam), "AutoExam=0 diagnostic mode, skip submit"))
			continue
		}

		examLabel := icveExamLabel(exam)
		for i := range problems {
			func(idx int) {
				defer func() {
					if r := recover(); r != nil {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d answer panic skipped: %v", idx+1, r)))
					}
				}()
				p := &problems[idx]
				icveLogQuestion(account, course.CourseName, examLabel, idx+1, p, emit)
				if strings.TrimSpace(p.RecordAnswer) != "" {
					return
				}
				if strings.TrimSpace(p.Stem) == "" && len(p.Options) == 0 && len(p.OptionsMap) == 0 {
					emit("%s", formatICVELog(account, course.CourseName, examLabel,
						fmt.Sprintf("problem %d has empty stem/options, skip answer", idx+1)))
					return
				}
				switch cc.AutoExam {
				case 1:
					if !hasAIConfig(setting) {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d AutoExam=1 but AI not configured, skip", idx+1)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					answers, err := answerICVEProblemWithAI(account, setting, course.CourseName, *p, func(msg string) {
						emit("%s", formatICVELog(account, course.CourseName, examLabel, msg))
					})
					if err != nil || len(answers) == 0 {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d AI returned no answer", idx+1)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					sanitized := sanitizeICVEAnswerCandidate(strings.Join(answers, ","))
					if sanitized == "" {
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					applyICVEAnswer(p, strings.Split(sanitized, ","))
					if strings.TrimSpace(p.RecordAnswer) == "" {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d AI answer mapping failed: raw=%q", idx+1, sanitized)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					emit("%s", formatICVELog(account, course.CourseName, examLabel,
						fmt.Sprintf("problem %d answer applied: %s", idx+1, p.RecordAnswer)))
				case 2:
					if !hasExternalQuestionBank(setting) {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d AutoExam=2 but external qbank not configured, skip", idx+1)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					answers2 := queryExternalQuestion(setting, QuestionBankQuestion{
						Type:       p.QuestionType,
						Content:    p.Stem,
						OptionsMap: icveQuestionBankOptions(*p),
						CourseName: course.CourseName,
					}, func(msg string) {
						emit("%s", formatICVELog(account, course.CourseName, examLabel, msg))
					})
					if len(answers2) == 0 {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d external qbank returned no answer", idx+1)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					sanitized2 := sanitizeICVEAnswerCandidate(strings.Join(answers2, ","))
					if sanitized2 == "" {
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					applyICVEAnswer(p, strings.Split(sanitized2, ","))
					if strings.TrimSpace(p.RecordAnswer) == "" {
						emit("%s", formatICVELog(account, course.CourseName, examLabel,
							fmt.Sprintf("problem %d external qbank answer mapping failed: raw=%q", idx+1, sanitized2)))
						icveApplyRandomFallback(account, course.CourseName, examLabel, idx+1, p, randomAnswerOnFail, emit)
						return
					}
					emit("%s", formatICVELog(account, course.CourseName, examLabel,
						fmt.Sprintf("problem %d answer applied: %s", idx+1, p.RecordAnswer)))
				default:
					emit("%s", formatICVELog(account, course.CourseName, examLabel,
						fmt.Sprintf("problem %d unsupported AutoExam=%d, skip", idx+1, cc.AutoExam)))
				}
			}(i)
		}

		rec.TaskExamProblemRecordList = problems

		answered := 0
		for _, p := range problems {
			if strings.TrimSpace(p.RecordAnswer) != "" {
				answered++
			}
		}
		total := len(problems)
		pct := 0
		if total > 0 {
			pct = answered * 100 / total
		}
		emit("%s", formatICVELog(account, course.CourseName, examLabel,
			fmt.Sprintf("answered %d/%d (%d%%)", answered, total, pct)))
		icveLogAnswerSnapshot(account, course.CourseName, examLabel, problems, emit)

		if answered == 0 {
			emit("%s", formatICVELog(account, course.CourseName, examLabel, "no answers filled, skip save"))
			continue
		}
		if pct < submitThresholdPercent {
			emit("%s", formatICVELog(account, course.CourseName, examLabel,
				fmt.Sprintf("answered %d%% below threshold %d%%, skip submit", pct, submitThresholdPercent)))
			continue
		}

		if err := icveSaveExamAnswers(cache, exam, rec); err != nil {
			if err == errNotInAnswerTime {
				emit("%s", formatICVELog(account, course.CourseName, examLabel, "not in answer time, skip"))
				continue
			}
			emit("%s", formatICVELog(account, course.CourseName, examLabel,
				fmt.Sprintf("save answers failed: %s", err.Error())))
			continue
		}
		emit("%s", formatICVELog(account, course.CourseName, examLabel, "answers saved"))

		if cc.ExamAutoSubmit == 1 {
			if err := icveSubmitExam(cache, exam, rec); err != nil {
				if err == errNotInAnswerTime {
					emit("%s", formatICVELog(account, course.CourseName, examLabel, "not in answer time for submit, skip"))
					continue
				}
				emit("%s", formatICVELog(account, course.CourseName, examLabel,
					fmt.Sprintf("submit exam failed: %s", err.Error())))
				continue
			}
			emit("%s", formatICVELog(account, course.CourseName, examLabel, "exam submitted"))
		}

		time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
	}
}

// ---------- helper functions ----------

func icveExamLabel(exam icveExamItem) string {
	if strings.TrimSpace(exam.ExamName) != "" {
		return strings.TrimSpace(exam.ExamName)
	}
	if strings.TrimSpace(exam.Name) != "" {
		return strings.TrimSpace(exam.Name)
	}
	if strings.TrimSpace(exam.Title) != "" {
		return strings.TrimSpace(exam.Title)
	}
	id := strings.TrimSpace(exam.Id)
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func icveLogQuestion(account, course, examLabel string, idx int, p *icveExamProblemRecord, emit emitFn) {
	stem := p.Stem
	if len(stem) > 80 {
		stem = stem[:80] + "..."
	}
	emit("%s", formatICVELog(account, course, examLabel,
		fmt.Sprintf("problem %d type=%s stem=%q optCount=%d answer=%q recordAnswer=%q",
			idx, p.QuestionType, stem, len(p.Options), p.Answer, p.RecordAnswer)))
}

func icveLogAnswerSnapshot(account, course, examLabel string, problems []icveExamProblemRecord, emit emitFn) {
	var parts []string
	for i, p := range problems {
		parts = append(parts, fmt.Sprintf("%d:%s", i+1, p.RecordAnswer))
	}
	emit("%s", formatICVELog(account, course, examLabel,
		fmt.Sprintf("answer snapshot: %s", strings.Join(parts, " "))))
}

func sanitizeICVEAnswerCandidate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// strip control characters
		var cb strings.Builder
		for _, r := range p {
			if r >= 32 {
				cb.WriteRune(r)
			}
		}
		p = strings.TrimSpace(cb.String())
		if p == "?" || p == "" {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

func icveNormalizeJudgeValue(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "\u6b63\u786e", "\u5bf9", "\u662f", "true", "t", "yes", "y", "a", "0":
		return "true"
	case "\u9519\u8bef", "\u9519", "\u5426", "false", "f", "no", "n", "b", "1":
		return "false"
	default:
		return ""
	}
}

func icveOptionSubmitValue(opt icveOptionItem, qtype string) string {
	if qtype == "judge" && strings.TrimSpace(opt.Label) != "" {
		return strings.TrimSpace(opt.Label)
	}
	return opt.Id
}

func icvePlainOptionText(s string) string {
	s = html.UnescapeString(strings.TrimSpace(s))
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
			continue
		case '>':
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func icveCompactMatchText(s string) string {
	s = strings.ToLower(icvePlainOptionText(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= '\u4e00' && r <= '\u9fff') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func icveAnswerLeadingLetter(s string) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) == 0 {
		return ""
	}
	first := runes[0]
	if first >= 'a' && first <= 'z' {
		first -= 'a' - 'A'
	}
	if first >= 'A' && first <= 'Z' {
		if len(runes) == 1 {
			return string(first)
		}
		switch runes[1] {
		case '.', ')', '、', '．', ':', '：', ' ':
			return string(first)
		}
	}
	plain := strings.ToUpper(icvePlainOptionText(s))
	for _, marker := range []string{"答案", "选项", "选择"} {
		if strings.Contains(plain, marker) {
			for _, r := range plain {
				if r >= 'A' && r <= 'Z' {
					return string(r)
				}
			}
		}
	}
	return ""
}

func icveMapAnswerToOption(answer string, opts []icveOptionItem, qtype string) string {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ""
	}

	var candidates []string
	if strings.Contains(answer, ",") {
		for _, p := range strings.Split(answer, ",") {
			candidates = append(candidates, strings.TrimSpace(p))
		}
	} else if qtype == "multiple" {
		runes := []rune(answer)
		allAlpha := true
		for _, r := range runes {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				allAlpha = false
				break
			}
		}
		if allAlpha && len(runes) > 1 {
			for _, r := range runes {
				candidates = append(candidates, strings.ToUpper(string(r)))
			}
		} else {
			candidates = []string{answer}
		}
	} else {
		candidates = []string{answer}
	}

	mapOne := func(cand string) string {
		cand = strings.TrimSpace(cand)
		if cand == "?" || cand == "" {
			return ""
		}
		if letter := icveAnswerLeadingLetter(cand); letter != "" {
			for _, opt := range opts {
				if strings.EqualFold(opt.Label, letter) {
					return icveOptionSubmitValue(opt, qtype)
				}
			}
			if len(letter) == 1 && letter[0] >= 'A' && letter[0] <= 'Z' && len(opts) > 0 {
				idx := int(letter[0] - 'A')
				if idx < len(opts) {
					return icveOptionSubmitValue(opts[idx], qtype)
				}
			}
		}
		runes := []rune(cand)
		if len(runes) > 2 {
			first := string(runes[:1])
			sep := string(runes[1:2])
			if (sep == "." || sep == ")" || sep == "、" || sep == "．" || sep == ":" || sep == "：") && first >= "A" && first <= "Z" {
				cand = first
			}
		}
		if qtype == "judge" {
			target := icveNormalizeJudgeValue(cand)
			if target != "" {
				for _, opt := range opts {
					if icveNormalizeJudgeValue(opt.Content) == target {
						return icveOptionSubmitValue(opt, qtype)
					}
				}
				if len(opts) >= 2 {
					if target == "true" {
						return icveOptionSubmitValue(opts[0], qtype)
					}
					return icveOptionSubmitValue(opts[1], qtype)
				}
			}
		}
		for _, opt := range opts {
			if strings.EqualFold(opt.Label, cand) {
				return icveOptionSubmitValue(opt, qtype)
			}
		}
		candLower := strings.ToLower(cand)
		candCompact := icveCompactMatchText(cand)
		for _, opt := range opts {
			if strings.ToLower(opt.Content) == candLower {
				return icveOptionSubmitValue(opt, qtype)
			}
		}
		for _, opt := range opts {
			optCompact := icveCompactMatchText(opt.Content)
			if optCompact != "" && candCompact != "" && (strings.Contains(candCompact, optCompact) || strings.Contains(optCompact, candCompact)) {
				return icveOptionSubmitValue(opt, qtype)
			}
			if opt.Content != "" && strings.Contains(candLower, strings.ToLower(opt.Content)) {
				return icveOptionSubmitValue(opt, qtype)
			}
		}
		// positional fallback: A→opts[0], B→opts[1], ...
		if len(cand) == 1 && cand[0] >= 'A' && cand[0] <= 'Z' && len(opts) > 0 {
			idx := int(cand[0] - 'A')
			if idx < len(opts) {
				return icveOptionSubmitValue(opts[idx], qtype)
			}
		}
		return ""
	}

	if qtype == "multiple" {
		seen := map[string]bool{}
		var ids []string
		for _, c := range candidates {
			id := mapOne(c)
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		return strings.Join(ids, ",")
	}
	return mapOne(candidates[0])
}

func applyICVEAnswer(p *icveExamProblemRecord, answers []string) {
	if len(answers) == 0 {
		return
	}
	qtype := p.QuestionType
	if qtype == "fill" || qtype == "short" || qtype == "essay" {
		text := strings.TrimSpace(strings.Join(answers, " "))
		if text == "" || text == "?" {
			return
		}
		result := icveBuildFillAnswer(p.Answer, text)
		if result == "" {
			return
		}
		p.RecordAnswer = result
		if p.RawFields != nil {
			p.RawFields["recordAnswer"] = result
		}
		return
	}
	joined := sanitizeICVEAnswerCandidate(strings.Join(answers, ","))
	if joined == "" {
		return
	}
	var mapped string
	if len(p.Options) > 0 {
		mapped = icveMapAnswerToOption(joined, p.Options, qtype)
	} else {
		mapped = joined
	}
	if strings.TrimSpace(mapped) == "" {
		return
	}
	p.RecordAnswer = mapped
	if p.RawFields != nil {
		p.RawFields["recordAnswer"] = mapped
	}
}

func icveQuestionBankOptions(q icveExamProblemRecord) map[string]string {
	hasLetter := false
	for k := range q.OptionsMap {
		if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
			hasLetter = true
			break
		}
	}
	if hasLetter {
		out := map[string]string{}
		for k, v := range q.OptionsMap {
			if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
				out[k] = v
			}
		}
		return out
	}
	if len(q.Options) > 0 {
		out := map[string]string{}
		alphas := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		for i, opt := range q.Options {
			if i < len(alphas) {
				out[string(alphas[i])] = opt.Content
			}
		}
		return out
	}
	if len(q.OptionsMap) > 0 {
		out := map[string]string{}
		alphas := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		nums := make([]int, 0, len(q.OptionsMap))
		for k := range q.OptionsMap {
			if n, err := strconv.Atoi(k); err == nil {
				nums = append(nums, n)
			}
		}
		sort.Ints(nums)
		for i, n := range nums {
			if i < len(alphas) {
				out[string(alphas[i])] = q.OptionsMap[strconv.Itoa(n)]
			}
		}
		return out
	}
	return map[string]string{}
}

func answerICVEProblemWithAI(account string, setting consoleConfig.Setting, courseName string, p icveExamProblemRecord, pfx func(string)) ([]string, error) {
	ai := setting.AiSetting
	options := icveQuestionBankOptions(p)
	lines := []string{
		"请回答这道题，只输出答案选项字母或答案文本。",
		"多选题请用逗号分隔，例如 A,C。",
		"课程：" + courseName,
		"题型：" + p.QuestionType,
		"题干：" + p.Stem,
	}
	if len(options) > 0 {
		keys := make([]string, 0, len(options))
		for k := range options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines = append(lines, "选项：")
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("%s. %s", k, options[k]))
		}
	}
	msg := []map[string]string{
		{"role": "system", "content": "你是课程测验答题助手。请严格按题目要求给出答案，不要解释。"},
		{"role": "user", "content": strings.Join(lines, "\n")},
	}
	if pfx != nil {
		pfx(fmt.Sprintf("调用AI：type=%s stem=%s optCount=%d", p.QuestionType, abbreviate(p.Stem, 80), len(options)))
	}
	return safeAnswerAIGet(account, ai.AiUrl, ai.Model, ai.AiType, msg, ai.APIKEY, nil, pfx)
}

func icveApplyRandomFallback(account, course, examLabel string, questionNo int, p *icveExamProblemRecord, randomAnswerOnFail int, emit emitFn) {
	if randomAnswerOnFail == 0 {
		return
	}
	switch p.QuestionType {
	case "fill", "short", "essay":
		return
	}
	keys := make([]string, 0, len(p.OptionsMap))
	for k := range p.OptionsMap {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	var chosen []string
	switch p.QuestionType {
	case "multiple":
		n := rand.Intn(3) + 1
		if n > len(keys) {
			n = len(keys)
		}
		perm := rand.Perm(len(keys))
		for i := 0; i < n; i++ {
			chosen = append(chosen, keys[perm[i]])
		}
		sort.Strings(chosen)
	default:
		chosen = []string{keys[rand.Intn(len(keys))]}
	}
	mapped := icveMapAnswerToOption(strings.Join(chosen, ","), p.Options, p.QuestionType)
	if mapped == "" {
		mapped = strings.Join(chosen, ",")
	}
	p.RecordAnswer = mapped
	if p.RawFields != nil {
		p.RawFields["recordAnswer"] = mapped
	}
	emit("%s", formatICVELog(account, course, examLabel,
		fmt.Sprintf("problem %d random fallback: %s", questionNo, mapped)))
}

func icveTopLevelKeys(raw string) string {
	if raw == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func icveMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
