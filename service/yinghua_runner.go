package service

// SafeYingHuaRun 是英华学堂任务的安全桌面版本。
// 改造要点：os.Exit/log.Fatal → emit error + return，支持 ctx 软停止。

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/thedevsaddam/gojsonq"
	"github.com/yatori-dev/yatori-go-core/aggregation/yinghua"
	yinghuaApi "github.com/yatori-dev/yatori-go-core/api/yinghua"
	"github.com/yatori-dev/yatori-go-core/que-core/aiq"
	"github.com/yatori-dev/yatori-go-core/que-core/external"
	consoleConfig "yatori-go-console/config"
)

func SafeYingHuaRun(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache, emit emitFn) error {

	if emit == nil {
		emit = func(f string, a ...interface{}) {}
	}

	log := func(course, node, msg string) {
		emit("[%s] [英华][%s] 【%s】【%s】%s",
			time.Now().Format("2006-01-02 15:04:05"),
			cache.Account, course, node, msg)
	}

	courseList, err := yinghua.CourseListAction(cache)
	if err != nil {
		return fmt.Errorf("拉取课程失败: %w", err)
	}
	log("", "", fmt.Sprintf("已拉取课程总数：%d", len(courseList)))

	// 课程级并发限制：从 CxNode 读取，clamp 到 1-8，默认 1
	concurrency := 1
	if user.CoursesCustom.CxNode != nil && *user.CoursesCustom.CxNode > 0 {
		concurrency = *user.CoursesCustom.CxNode
	}
	if concurrency > 8 {
		concurrency = 8
	}
	log("", "", fmt.Sprintf("【并发配置】实际并发数=%d videoModel=%d", concurrency, user.CoursesCustom.VideoModel))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range courseList {
		if ctx.Err() != nil {
			break
		}
		course := courseList[i]

		if len(user.CoursesCustom.ExcludeCourses) > 0 &&
			consoleConfig.CmpCourse(course.Name, user.CoursesCustom.ExcludeCourses) {
			log(course.Name, "", "在排除列表，跳过")
			continue
		}
		if len(user.CoursesCustom.IncludeCourses) > 0 &&
			!consoleConfig.CmpCourse(course.Name, user.CoursesCustom.IncludeCourses) {
			log(course.Name, "", "不在包含列表，跳过")
			continue
		}
		if time.Now().Before(course.StartDate) {
			log(course.Name, "", "课程还未开始，跳过")
			continue
		}

		if user.CoursesCustom.VideoModel == 2 {
			sem <- struct{}{}
			wg.Add(1)
			go func(c yinghua.YingHuaCourse) {
				defer wg.Done()
				defer func() { <-sem }()
				yinghuaRunCourse(ctx, setting, user, cache, &c, log)
			}(course)
		} else {
			yinghuaRunCourse(ctx, setting, user, cache, &course, log)
		}
	}
	wg.Wait()

	log("", "", "所有课程执行完毕")
	return nil
}

func yinghuaRunCourse(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse,
	log func(course, node, msg string)) {

	nodeList, err := yinghua.VideosListAction(cache, *course)
	if err != nil {
		log(course.Name, "", "拉取节点列表失败: "+err.Error())
		return
	}

	var videosWg sync.WaitGroup
	redAns := 0

	for _, node := range nodeList {
		if ctx.Err() != nil {
			break
		}
		n := node
		switch user.CoursesCustom.VideoModel {
		case 1:
			yinghuaVideo(ctx, setting, user, cache, course, n, log)
		case 2:
			videosWg.Add(1)
			go func() {
				defer videosWg.Done()
				yinghuaVideoViolence(ctx, setting, user, cache, course, n, log)
			}()
		case 3:
			if n.ErrorMessage == "检测到可能使用并行播放刷课" {
				redAns++
			}
			yinghuaVideoBadRed(ctx, setting, user, cache, course, n, log)
		}
		yinghuaWork(ctx, setting, user, cache, course, n, log)
		yinghuaExam(ctx, setting, user, cache, course, n, log)
	}
	videosWg.Wait()

	// 去红模式递归
	if user.CoursesCustom.VideoModel == 3 && redAns != 0 && ctx.Err() == nil {
		yinghuaRunCourse(ctx, setting, user, cache, course, log)
	}
	log(course.Name, "", "课程学习完毕")
}

func yinghuaVideo(ctx context.Context, _ consoleConfig.Setting,
	_ *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse, node yinghua.YingHuaNode,
	log func(string, string, string)) {

	if !node.TabVideo || int(node.Progress) == 100 {
		return
	}
	log(course.Name, node.Name, "开始学习视频")
	t := node.ViewedDuration
	studyId := "0"
	for {
		if ctx.Err() != nil {
			return
		}
		t += 5
		if node.Progress == 100 {
			log(course.Name, node.Name, "学习完毕")
			break
		}
		sub, err := yinghua.SubmitStudyTimeAction(cache, node.Id, studyId, t)
		if err != nil {
			log(course.Name, node.Name, "提交学时异常: "+err.Error())
		}
		yinghua.LoginTimeoutAfreshAction(cache, sub)

		msgVal := gojsonq.New().JSONString(sub).Find("msg")
		msg, ok := msgVal.(string)
		if !ok || msg == "" {
			sleepCtx(ctx, 10*time.Second)
			continue
		}
		if msg != "提交学时成功!" {
			reg := regexp.MustCompile(`该课程解锁时间【[^【]*】未到!`)
			if reg.MatchString(msg) {
				log(course.Name, node.Name, "课程未到解锁时间，跳过")
				break
			}
			sleepCtx(ctx, 10*time.Second)
			continue
		}
		if v := gojsonq.New().JSONString(sub).Find("result.data.studyId"); v != nil {
			if f, ok2 := v.(float64); ok2 {
				studyId = strconv.Itoa(int(f))
			}
		}
		log(course.Name, node.Name, fmt.Sprintf("提交成功 %d/%d (%.1f%%)",
			t, node.VideoDuration, float32(t)/float32(node.VideoDuration)*100))
		sleepCtx(ctx, 5*time.Second)
		if t >= node.VideoDuration {
			break
		}
	}
}

func yinghuaVideoViolence(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse, node yinghua.YingHuaNode,
	log func(string, string, string)) {

	if !node.TabVideo || int(node.Progress) == 100 {
		return
	}
	log(course.Name, node.Name, "暴力模式学习视频")
	t := node.ViewedDuration
	studyId := "0"
	for {
		if ctx.Err() != nil {
			return
		}
		t += node.VideoDuration
		sub, err := yinghua.SubmitStudyTimeAction(cache, node.Id, studyId, t)
		if err != nil {
			log(course.Name, node.Name, "提交学时异常: "+err.Error())
		}
		yinghua.LoginTimeoutAfreshAction(cache, sub)
		msgVal := gojsonq.New().JSONString(sub).Find("msg")
		msg, _ := msgVal.(string)
		if msg == "提交学时成功!" {
			log(course.Name, node.Name, "暴力模式提交成功")
			break
		}
		if msg != "" {
			log(course.Name, node.Name, "暴力模式提交状态："+msg)
		}
		sleepCtx(ctx, 5*time.Second)
	}
}

func yinghuaVideoBadRed(ctx context.Context, _ consoleConfig.Setting,
	_ *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse, node yinghua.YingHuaNode,
	log func(string, string, string)) {

	if !node.TabVideo || int(node.Progress) == 100 {
		return
	}
	if node.ErrorMessage == "检测到可能使用并行播放刷课" {
		log(course.Name, node.Name, "检测到标红，执行去红模式")
	}
	// 去红：单次提交完整时长
	sub, err := yinghua.SubmitStudyTimeAction(cache, node.Id, "0", node.VideoDuration)
	if err != nil {
		log(course.Name, node.Name, "去红提交异常: "+err.Error())
		return
	}
	yinghua.LoginTimeoutAfreshAction(cache, sub)
	msg, _ := gojsonq.New().JSONString(sub).Find("msg").(string)
	log(course.Name, node.Name, "去红提交："+msg)
	sleepCtx(ctx, 3*time.Second)
}

func yinghuaWork(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse, node yinghua.YingHuaNode,
	log func(string, string, string)) {

	if user.CoursesCustom.AutoExam == 0 || !node.TabWork || ctx.Err() != nil {
		return
	}
	if user.CoursesCustom.AutoExam == 1 {
		if err := aiq.AICheck(setting.AiSetting.AiUrl, setting.AiSetting.Model,
			setting.AiSetting.APIKEY, setting.AiSetting.AiType); err != nil {
			log(course.Name, node.Name, "AI不可用，跳过作业: "+err.Error())
			return
		}
	} else if user.CoursesCustom.AutoExam == 2 {
		if err := external.CheckApiQueRequest(setting.ApiQueSetting.Url, 3, nil); err != nil {
			log(course.Name, node.Name, "外置题库不可用，跳过作业: "+err.Error())
			return
		}
	}
	detail, _ := yinghua.WorkDetailAction(cache, node.Id)
	if len(detail) == 0 {
		return
	}
	for _, work := range detail {
		if ctx.Err() != nil {
			return
		}
		var err error
		switch user.CoursesCustom.AutoExam {
		case 1:
			err = yinghua.StartWorkAction(cache, work,
				setting.AiSetting.AiUrl, setting.AiSetting.Model,
				setting.AiSetting.APIKEY, setting.AiSetting.AiType,
				user.CoursesCustom.ExamAutoSubmit)
		case 2:
			err = yinghua.StartWorkForExternalAction(cache, setting.ApiQueSetting.Url, work, user.CoursesCustom.ExamAutoSubmit)
		}
		if err != nil {
			log(course.Name, node.Name, "作业执行失败: "+err.Error())
			continue
		}
		if user.CoursesCustom.ExamAutoSubmit == 1 {
			if s, err2 := yinghua.WorkedFinallyScoreAction(cache, work); err2 == nil {
				log(course.Name, node.Name, fmt.Sprintf("作业完成，最高分：%s / %.0f", s, work.Score))
			}
		} else {
			log(course.Name, node.Name, "作业完成，请手动提交")
		}
	}
}

func yinghuaExam(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *yinghuaApi.YingHuaUserCache,
	course *yinghua.YingHuaCourse, node yinghua.YingHuaNode,
	log func(string, string, string)) {

	if user.CoursesCustom.AutoExam == 0 || !node.TabExam || ctx.Err() != nil {
		return
	}
	if user.CoursesCustom.AutoExam == 1 {
		if err := aiq.AICheck(setting.AiSetting.AiUrl, setting.AiSetting.Model,
			setting.AiSetting.APIKEY, setting.AiSetting.AiType); err != nil {
			log(course.Name, node.Name, "AI不可用，跳过考试: "+err.Error())
			return
		}
	} else if user.CoursesCustom.AutoExam == 2 {
		if err := external.CheckApiQueRequest(setting.ApiQueSetting.Url, 3, nil); err != nil {
			log(course.Name, node.Name, "外置题库不可用，跳过考试: "+err.Error())
			return
		}
	}
	detail, _ := yinghua.ExamDetailAction(cache, node.Id)
	if len(detail) == 0 {
		return
	}
	for _, exam := range detail {
		if ctx.Err() != nil {
			return
		}
		var err error
		switch user.CoursesCustom.AutoExam {
		case 1:
			err = yinghua.StartExamAction(cache, exam,
				setting.AiSetting.AiUrl, setting.AiSetting.Model,
				setting.AiSetting.APIKEY, setting.AiSetting.AiType,
				user.CoursesCustom.ExamAutoSubmit)
		case 2:
			err = yinghua.StartExamForExternalAction(cache, exam, setting.ApiQueSetting.Url, user.CoursesCustom.ExamAutoSubmit)
		}
		if err != nil {
			log(course.Name, node.Name, "考试执行失败: "+err.Error())
			continue
		}
		if user.CoursesCustom.ExamAutoSubmit == 1 {
			if s, err2 := yinghua.ExamFinallyScoreAction(cache, exam); err2 == nil {
				log(course.Name, node.Name, fmt.Sprintf("考试完成，最高分：%s / %.0f", s, exam.Score))
			}
		} else {
			log(course.Name, node.Name, "考试完成，请手动提交")
		}
	}
}
