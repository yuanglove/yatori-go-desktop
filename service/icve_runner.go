package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	icveAction "github.com/yatori-dev/yatori-go-core/aggregation/icve"
	icveApi "github.com/yatori-dev/yatori-go-core/api/icve"
	consoleConfig "yatori-go-console/config"
)

func nonEmptyErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func formatICVELog(account, course, node, message string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s] [ICVE][%s] ",
		time.Now().Format("2006-01-02 15:04:05"), account))
	for _, part := range []string{course, node} {
		if strings.TrimSpace(part) != "" {
			b.WriteString("【")
			b.WriteString(part)
			b.WriteString("】")
		}
	}
	b.WriteString(message)
	return b.String()
}

func RunIcveSafe(setting consoleConfig.Setting, user consoleConfig.User, randomAnswerOnFail int, submitThresholdPercent int, emit emitFn) {
	if emit == nil {
		emit = func(string, ...interface{}) {}
	}
	_ = setting

	account := strings.TrimSpace(user.Account)
	cookie := strings.TrimSpace(user.Password)
	emit("%s", formatICVELog(account, "", "", "开始 Cookie 登录"))
	if cookie == "" || len(cookie) <= 30 {
		emit("%s", formatICVELog(account, "", "", "Cookie 为空或长度过短：智慧职教目前只支持 Cookie 登录，请把浏览器复制的完整 Cookie 填到密码/Cookie 字段"))
		return
	}
	cache := &icveApi.IcveUserCache{Account: account, Password: cookie}
	if _, err := icveRetry(func() (struct{}, error) {
		return struct{}{}, icveAction.IcveCookieLogin(cache)
	}); err != nil {
		emit("%s", formatICVELog(account, "", "", "Cookie 登录失败："+nonEmptyErr(err)))
		return
	}
	emit("%s", formatICVELog(account, "", "", "Cookie 登录成功，开始刷课"))

	courses, err := icveRetry(func() ([]icveAction.IcveCourse, error) {
		return icveAction.PullZYKCourseAction(cache)
	})
	if err != nil {
		emit("%s", formatICVELog(account, "", "", "拉取课程失败："+nonEmptyErr(err)))
		return
	}
	if merged, n, err := icveMergeWebCourses(cache, courses); err == nil {
		if n > 0 {
			emit("%s", formatICVELog(account, "", "", fmt.Sprintf("已补充网页课程：%d 门（含已修课程）", n)))
		}
		courses = merged
	} else {
		emit("%s", formatICVELog(account, "", "", "补充网页课程失败，沿用核心库课程："+nonEmptyErr(err)))
	}
	emit("%s", formatICVELog(account, "", "", fmt.Sprintf("已拉取课程总数：%d", len(courses))))

	totalNodes, submitted, failed := 0, 0, 0
	handledCourses := 0
	for _, course := range courses {
		emit("%s", formatICVELog(account, course.CourseName, "", fmt.Sprintf("课程候选：courseId=%s courseInfoId=%s status=%s", course.CourseId, course.CourseInfoId, course.Status)))
		if reason := icveCourseSkipReason(user, course.CourseName); reason != "" {
			emit("%s", formatICVELog(account, course.CourseName, "", "课程过滤跳过："+reason))
			continue
		}
		handledCourses++
		runICVECourse(setting, user, cache, course, emit, randomAnswerOnFail, submitThresholdPercent, &totalNodes, &submitted, &failed)
	}
	if len(courses) > 0 && handledCourses == 0 {
		cc := user.CoursesCustom
		emit("%s", formatICVELog(account, "", "", fmt.Sprintf("未处理任何课程：请检查账号管理里的包含课程/排除课程设置。include=%v exclude=%v", cc.IncludeCourses, cc.ExcludeCourses)))
	}
	emit("%s", formatICVELog(account, "", "", fmt.Sprintf("任务完成：任务点 %d，提交成功 %d，失败 %d", totalNodes, submitted, failed)))
}

func shouldSkipICVECourse(user consoleConfig.User, courseName string) bool {
	return icveCourseSkipReason(user, courseName) != ""
}

func icveCourseSkipReason(user consoleConfig.User, courseName string) string {
	cc := user.CoursesCustom
	if len(cc.ExcludeCourses) != 0 && icveCourseMatchesList(courseName, cc.ExcludeCourses) {
		return fmt.Sprintf("命中排除课程列表 exclude=%v", cc.ExcludeCourses)
	}
	if len(cc.IncludeCourses) != 0 && !icveCourseMatchesList(courseName, cc.IncludeCourses) {
		return fmt.Sprintf("不在包含课程列表 include=%v", cc.IncludeCourses)
	}
	return ""
}

func icveCourseMatchesList(courseName string, list []string) bool {
	if consoleConfig.CmpCourse(courseName, list) {
		return true
	}
	courseName = strings.TrimSpace(courseName)
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" || courseName == "" {
			continue
		}
		if strings.Contains(courseName, item) || strings.Contains(item, courseName) {
			return true
		}
	}
	return false
}

func icveMergeWebCourses(cache *icveApi.IcveUserCache, coreCourses []icveAction.IcveCourse) ([]icveAction.IcveCourse, int, error) {
	webCourses, err := icveFetchStudentCourses(cache)
	if err != nil {
		return coreCourses, 0, err
	}
	merged := append([]icveAction.IcveCourse{}, coreCourses...)
	seen := map[string]bool{}
	for _, c := range merged {
		seen[icveCourseKey(c.CourseId, c.CourseInfoId, c.CourseName)] = true
	}
	added := 0
	for _, c := range webCourses {
		courseId := strings.TrimSpace(c.CourseId)
		courseInfoId := strings.TrimSpace(c.CourseInfoId)
		courseName := strings.TrimSpace(c.displayName())
		if courseId == "" || courseInfoId == "" || courseName == "" {
			continue
		}
		key := icveCourseKey(courseId, courseInfoId, courseName)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, icveAction.IcveCourse{
			Id:           c.Id,
			CourseId:     courseId,
			CourseInfoId: courseInfoId,
			CourseName:   courseName,
			WeekStr:      c.CourseInfoName,
			Status:       c.Status,
		})
		added++
	}
	return merged, added, nil
}

func icveCourseKey(courseId, courseInfoId, courseName string) string {
	if strings.TrimSpace(courseId) != "" || strings.TrimSpace(courseInfoId) != "" {
		return strings.TrimSpace(courseId) + "_" + strings.TrimSpace(courseInfoId)
	}
	return "name:" + strings.TrimSpace(courseName)
}

func runICVECourse(setting consoleConfig.Setting, user consoleConfig.User, cache *icveApi.IcveUserCache, course icveAction.IcveCourse, emit emitFn, randomAnswerOnFail int, submitThresholdPercent int, totalNodes, submitted, failed *int) {
	account := strings.TrimSpace(user.Account)
	emit("%s", formatICVELog(account, course.CourseName, "", "开始学习课程"))
	if course.Status == "3" {
		emit("%s", formatICVELog(account, course.CourseName, "", "课程已结束：跳过资源任务点，仅探测作业/测验"))
	} else {
		nodes, err := icveCallWithTimeout(90*time.Second, func() ([]icveAction.IcveCourseNode, error) {
			return pullICVENodesFast(cache, course)
		})
		if err != nil {
			emit("%s", formatICVELog(account, course.CourseName, "", "拉取任务点失败，继续探测作业/测验："+nonEmptyErr(err)))
			*failed++
		} else {
			stats := collectICVENodeStats(user, nodes)
			emit("%s", formatICVELog(account, course.CourseName, "", stats.String()))

			for _, node := range nodes {
				if !shouldHandleICVENode(user, node) || node.Speed >= 100 {
					continue
				}
				*totalNodes++
				kind := icveNodeKind(node)
				if err := submitICVENode(cache, &node); err != nil {
					*failed++
					emit("%s", formatICVELog(account, course.CourseName, node.Name, kind+"提交异常："+nonEmptyErr(err)))
					continue
				}
				*submitted++
				emit("%s", formatICVELog(account, course.CourseName, node.Name, "学习记录提交成功（"+kind+"）"))
			}
		}
	}
	// 真实作业/考试系统（与资源库任务点学习记录独立）
	runICVERealExamWork(setting, user, cache, course, randomAnswerOnFail, submitThresholdPercent, emit)
	emit("%s", formatICVELog(account, course.CourseName, "", "课程学习完毕"))
}

func shouldHandleICVENode(user consoleConfig.User, node icveAction.IcveCourseNode) bool {
	if !isICVEProgressNode(node.FileType) {
		return false
	}
	// VideoModel=0 时跳过所有资源任务点（视频/音频/文档/测验全部受此控制）
	if user.CoursesCustom.VideoModel == 0 {
		return false
	}
	return true
}

type icveNodeStats struct {
	total                int
	video                int
	chapterTest          int
	work                 int
	exam                 int
	doc                  int
	other                int
	skipVideo            int
	skipSwitch           int
	completed            int
	toHandle             int
	completedChapterTest int
	completedWork        int
	completedExam        int
	completedDoc         int
	completedVideo       int
	pendingChapterTest   int
	pendingWork          int
	pendingExam          int
	pendingDoc           int
	pendingVideo         int
	unknownTypes         map[string]int
}

func collectICVENodeStats(user consoleConfig.User, nodes []icveAction.IcveCourseNode) icveNodeStats {
	stats := icveNodeStats{total: len(nodes), unknownTypes: map[string]int{}}
	for _, node := range nodes {
		kind := icveNodeKind(node)
		switch kind {
		case "视频/音频":
			stats.video++
		case "章节测验":
			stats.chapterTest++
		case "作业":
			stats.work++
		case "考试":
			stats.exam++
		case "文档资料":
			stats.doc++
		default:
			stats.other++
			stats.unknownTypes[strings.TrimSpace(node.FileType)]++
		}
		if !shouldHandleICVENode(user, node) {
			stats.skipVideo++
			continue
		}
		if node.Speed >= 100 {
			stats.completed++
			switch kind {
			case "章节测验":
				stats.completedChapterTest++
			case "作业":
				stats.completedWork++
			case "考试":
				stats.completedExam++
			case "文档资料":
				stats.completedDoc++
			case "视频/音频":
				stats.completedVideo++
			}
			continue
		}
		stats.toHandle++
		switch kind {
		case "章节测验":
			stats.pendingChapterTest++
		case "作业":
			stats.pendingWork++
		case "考试":
			stats.pendingExam++
		case "文档资料":
			stats.pendingDoc++
		case "视频/音频":
			stats.pendingVideo++
		}
	}
	return stats
}

func (s icveNodeStats) String() string {
	return fmt.Sprintf("资源任务点：视频/音频 %d 个，文档 %d 个，测验 %d 个，待提交 %d 个",
		s.video+s.work+s.exam, s.doc, s.chapterTest, s.toHandle)
}

func (s icveNodeStats) detailLines() []string {
	return nil
}

func isICVEVideoNode(fileType string) bool {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "mp4", "mp3":
		return true
	default:
		return false
	}
}

func icveIntValue(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func isICVETestNode(fileType string) bool {
	t := strings.TrimSpace(fileType)
	return t == "测验" || strings.EqualFold(t, "test") || strings.EqualFold(t, "quiz")
}

func icveNodeKind(node icveAction.IcveCourseNode) string {
	name := strings.TrimSpace(node.Name)
	fileType := strings.TrimSpace(node.FileType)
	lowerName := strings.ToLower(name)
	switch {
	case isICVEVideoNode(fileType):
		return "视频/音频"
	case strings.Contains(name, "考试") || strings.Contains(lowerName, "exam"):
		return "考试"
	case strings.Contains(name, "作业") || strings.Contains(lowerName, "homework") || strings.Contains(lowerName, "work"):
		return "作业"
	case isICVETestNode(fileType):
		return "章节测验"
	case icveDocLike(fileType) || strings.EqualFold(fileType, "zip"):
		return "文档资料"
	default:
		return "任务点"
	}
}

func submitICVENode(cache *icveApi.IcveUserCache, node *icveAction.IcveCourseNode) error {
	total, err := resolveICVENodeTotal(cache, node)
	if err != nil {
		return err
	}
	if total <= 0 {
		total = 1
	}
	_, err = icveRetry(func() (string, error) {
		return cache.SubmitZYKStudyTimeApi(node.CourseInfoId, "", node.ParentId, total, node.Id, cache.UserId, total, total, total, 3, nil)
	})
	return err
}

func resolveICVENodeTotal(cache *icveApi.IcveUserCache, node *icveAction.IcveCourseNode) (int, error) {
	if node.TotalNum > 0 {
		return int(node.TotalNum), nil
	}
	if strings.EqualFold(strings.TrimSpace(node.FileType), "zip") || strings.TrimSpace(node.FileType) == "测验" {
		return 1, nil
	}
	info, err := icveRetry(func() (string, error) {
		return cache.PullZykNodeInfoApi(node.Id)
	})
	if err != nil {
		return 0, err
	}
	if v, ok := jsonPathString(info, "data.urlShort"); ok {
		node.FileUrl = v
	}
	if strings.EqualFold(node.FileType, "mp3") {
		return 60, nil
	}
	if strings.EqualFold(node.FileType, "mp4") || icveDocLike(node.FileType) {
		if node.FileUrl == "" {
			return 1, nil
		}
		status, err := icveRetry(func() (string, error) {
			return cache.PullZykNodeDurationApi(node.FileUrl)
		})
		if err != nil {
			return 0, err
		}
		if strings.EqualFold(node.FileType, "mp4") {
			if s, ok := jsonPathString(status, "args.duration"); ok {
				return parseICVEDuration(s)
			}
		}
		if n, ok := jsonPathFloat(status, "args.page_count"); ok {
			if n <= 0 {
				return 1, nil
			}
			return int(n), nil
		}
	}
	return 1, nil
}

func icveDocLike(fileType string) bool {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "pdf", "doc", "docx", "ppt", "pptx":
		return true
	default:
		return false
	}
}

func parseICVEDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1, nil
	}
	if !strings.Contains(s, ":") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			total := int(math.Ceil(f))
			if total <= 0 {
				total = 1
			}
			return total, nil
		}
	}
	parts := strings.Split(s, ":")
	total := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil {
			f, ferr := strconv.ParseFloat(part, 64)
			if ferr != nil {
				return 0, err
			}
			n = int(math.Ceil(f))
		}
		total = total*60 + n
	}
	if total <= 0 {
		total = 1
	}
	return total, nil
}

func jsonPathString(raw, path string) (string, bool) {
	v, ok := jsonPathValue(raw, path)
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, strings.TrimSpace(t) != ""
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		return fmt.Sprint(t), true
	}
}

func jsonPathFloat(raw, path string) (float64, bool) {
	v, ok := jsonPathValue(raw, path)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func jsonPathValue(raw, path string) (interface{}, bool) {
	var root interface{}
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, false
	}
	cur := root
	for _, key := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = obj[key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
