package service

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	action "github.com/yatori-dev/yatori-go-core/aggregation/welearn"
	welearnAPI "github.com/yatori-dev/yatori-go-core/api/welearn"
	consoleConfig "yatori-go-console/config"
)

func formatWeLearnLog(account, course, chapter, item, message string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s] [WELEARN][%s] ",
		time.Now().Format("2006-01-02 15:04:05"), account))
	for _, part := range []string{course, chapter, item} {
		if strings.TrimSpace(part) != "" {
			b.WriteString("【")
			b.WriteString(part)
			b.WriteString("】")
		}
	}
	b.WriteString(message)
	return b.String()
}

func retryWeLearn[T any](times int, delay time.Duration, fn func() (T, error)) (out T, err error) {
	if times <= 0 {
		times = 1
	}
	for i := 0; i < times; i++ {
		out, err = safeWeLearnCall(fn)
		if err == nil {
			return out, nil
		}
		if i+1 < times {
			time.Sleep(delay)
		}
	}
	return out, err
}

func safeWeLearnCall[T any](fn func() (T, error)) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

func newWeLearnCache(user consoleConfig.User) *welearnAPI.WeLearnUserCache {
	return &welearnAPI.WeLearnUserCache{
		Account:  strings.TrimSpace(user.Account),
		Password: user.Password,
	}
}

func RunWeLearnSafe(setting consoleConfig.Setting, user consoleConfig.User, emit emitFn) {
	if emit == nil {
		emit = func(string, ...interface{}) {}
	}

	account := strings.TrimSpace(user.Account)
	cache := newWeLearnCache(user)
	emit("%s", formatWeLearnLog(account, "", "", "", "开始登录"))

	if _, err := retryWeLearn(3, 1200*time.Millisecond, func() (struct{}, error) {
		return struct{}{}, action.WeLearnLoginAction(cache)
	}); err != nil {
		emit("%s", formatWeLearnLog(account, "", "", "", "登录失败："+err.Error()))
		return
	}
	emit("%s", formatWeLearnLog(account, "", "", "", "登录成功，开始刷课"))

	courses, err := retryWeLearn(3, 1500*time.Millisecond, func() ([]action.WeLearnCourse, error) {
		return action.WeLearnPullCourseListAction(cache)
	})
	if err != nil {
		emit("%s", formatWeLearnLog(account, "", "", "", "拉取课程失败："+err.Error()))
		return
	}
	emit("%s", formatWeLearnLog(account, "", "", "", fmt.Sprintf("已拉取课程总数：%d", len(courses))))

	for _, course := range courses {
		if shouldSkipWeLearnCourse(user, course.Name) {
			continue
		}
		runWeLearnCourse(setting, user, cache, course, emit)
	}
	emit("%s", formatWeLearnLog(account, "", "", "", "任务完成"))
}

func shouldSkipWeLearnCourse(user consoleConfig.User, courseName string) bool {
	cc := user.CoursesCustom
	if len(cc.ExcludeCourses) != 0 && consoleConfig.CmpCourse(courseName, cc.ExcludeCourses) {
		return true
	}
	if len(cc.IncludeCourses) != 0 && !consoleConfig.CmpCourse(courseName, cc.IncludeCourses) {
		return true
	}
	return false
}

func runWeLearnCourse(setting consoleConfig.Setting, user consoleConfig.User, cache *welearnAPI.WeLearnUserCache, course action.WeLearnCourse, emit emitFn) {
	account := strings.TrimSpace(user.Account)
	emit("%s", formatWeLearnLog(account, course.Name, "", "", "开始学习课程"))

	chapters, err := retryWeLearn(3, 1200*time.Millisecond, func() ([]action.WeLearnChapter, error) {
		return action.WeLearnPullCourseChapterAction(cache, course)
	})
	if err != nil {
		emit("%s", formatWeLearnLog(account, course.Name, "", "", "拉取章节失败，已跳过本课程："+err.Error()))
		return
	}

	for _, chapter := range chapters {
		chapterName := chapter.Name
		if strings.TrimSpace(chapterName) == "" {
			chapterName = chapter.Unitname
		}
		points, err := retryWeLearn(3, 1200*time.Millisecond, func() ([]action.WeLearnPoint, error) {
			return action.WeLearnPullChapterPointAction(cache, course, chapter)
		})
		if err != nil {
			emit("%s", formatWeLearnLog(account, course.Name, chapterName, "", "拉取任务点失败，已跳过本章节："+err.Error()))
			continue
		}
		for _, point := range points {
			runWeLearnPoint(setting, user, cache, course, chapterName, point, emit)
		}
	}
	emit("%s", formatWeLearnLog(account, course.Name, "", "", "课程学习完毕"))
}

func runWeLearnPoint(setting consoleConfig.Setting, user consoleConfig.User, cache *welearnAPI.WeLearnUserCache, course action.WeLearnCourse, chapter string, point action.WeLearnPoint, emit emitFn) {
	_ = setting
	account := strings.TrimSpace(user.Account)
	item := point.Location
	if strings.TrimSpace(item) == "" {
		item = point.Name
	}
	if user.CoursesCustom.VideoModel == 0 {
		return
	}
	if !point.IsVisible {
		return
	}
	if isWeLearnCompleted(point.IsComplete) {
		return
	}

	switch user.CoursesCustom.VideoModel {
	case 1:
		submitWeLearnStudyTime(user, cache, course, chapter, point, item, emit)
	case 2:
		if _, err := retryWeLearn(3, 1200*time.Millisecond, func() (struct{}, error) {
			return struct{}{}, action.WeLearnCompletePointAction(cache, course, point)
		}); err != nil {
			emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "暴力完成失败："+err.Error()))
			return
		}
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "学习完毕"))
	default:
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, fmt.Sprintf("未知视频模式 %d，已跳过", user.CoursesCustom.VideoModel)))
	}
}

func isWeLearnCompleted(status string) bool {
	status = strings.TrimSpace(strings.ToLower(status))
	return status == "completed" || status == "已完成"
}

func submitWeLearnStudyTime(user consoleConfig.User, cache *welearnAPI.WeLearnUserCache, course action.WeLearnCourse, chapter string, point action.WeLearnPoint, item string, emit emitFn) {
	account := strings.TrimSpace(user.Account)
	state, err := retryWeLearn(3, 1200*time.Millisecond, func() (weLearnStudyState, error) {
		_, progress, session, total, scaledValue, callErr := action.WeLearnSubmitStudyTimeAction(cache, course, point)
		return weLearnStudyState{progressMeasure: progress, sessionTime: session, totalTime: total, scaled: scaledValue}, callErr
	})
	if err != nil {
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "获取学习状态失败："+err.Error()))
		return
	}
	progressMeasure := state.progressMeasure
	sessionTime := state.sessionTime
	totalTime := state.totalTime
	scaled := state.scaled

	endTime := parseWeLearnStudyTime(user.CoursesCustom.StudyTime, emit, account)
	if totalTime >= endTime {
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, fmt.Sprintf("当前学时已达到目标：%d/%d", totalTime, endTime)))
		return
	}

	for {
		api, err := retryWeLearn(3, 1200*time.Millisecond, func() (string, error) {
			return cache.KeepPointSessionPlan1Api(course.Cid, point.Id, course.Uid, course.ClassId, sessionTime, totalTime, 3, nil)
		})
		if err != nil {
			emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "提交学时失败："+err.Error()))
			return
		}
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, fmt.Sprintf("学时提交成功：%d/%d 服务器返回：%s", totalTime, endTime, api)))
		if sessionTime >= endTime {
			break
		}
		sessionTime += 60
		totalTime += 60
		time.Sleep(60 * time.Second)
	}

	if _, err := retryWeLearn(3, 1200*time.Millisecond, func() (string, error) {
		return cache.SubmitStudyPlan2Api(course.Cid, point.Id, course.Uid, scaled, course.ClassId, progressMeasure, "completed", 3, nil)
	}); err != nil {
		emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "最终提交失败："+err.Error()))
		return
	}
	emit("%s", formatWeLearnLog(account, course.Name, chapter, item, "学习完毕"))
}

type weLearnStudyState struct {
	progressMeasure int
	sessionTime     int
	totalTime       int
	scaled          string
}

func parseWeLearnStudyTime(raw string, emit emitFn, account string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1600
	}
	if fixedMinutes, err := parseWeLearnMinutes(raw); err == nil && fixedMinutes > 0 {
		return fixedMinutes * 60
	}
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		emit("%s", formatWeLearnLog(account, "", "", "", "studyTime 配置格式错误，请填 15、15分钟 或 15-30，已使用默认 1600 秒"))
		return 1600
	}
	minValue, err1 := parseWeLearnMinutes(parts[0])
	maxValue, err2 := parseWeLearnMinutes(parts[1])
	if err1 != nil || err2 != nil || minValue <= 0 || maxValue < minValue {
		emit("%s", formatWeLearnLog(account, "", "", "", "studyTime 配置范围无效，已使用默认 1600 秒"))
		return 1600
	}
	return (rand.Intn(maxValue-minValue+1) + minValue) * 60
}

func parseWeLearnMinutes(raw string) (int, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	for _, token := range []string{"分钟", "分", "mins", "minutes", "minute", "min", "m"} {
		s = strings.ReplaceAll(s, token, "")
	}
	return strconv.Atoi(strings.TrimSpace(s))
}
