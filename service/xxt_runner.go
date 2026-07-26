package service

// SafeXueXiTongRunner 是 logic/xuexitong.UserBlock 的安全桌面版本。
// 改造要点：
//   - 所有 os.Exit/log.Fatal → emit error log + return
//   - 每个循环检查 ctx.Done()，支持软停止
//   - 完整支持 VideoModel 1/2/3、CxNode、IncludeCourses/ExcludeCourses
//   - 复用原项目导出的 Execute* 子函数（不复制）

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/tls"
	"encoding/json"
	"github.com/thedevsaddam/gojsonq"
	xuexitong "github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	aiq "github.com/yatori-dev/yatori-go-core/que-core/aiq"
	"io"
	"net/http"
	"net/url"
	"regexp"
	consoleConfig "yatori-go-console/config"
	xxtLogic "yatori-go-console/logic/xuexitong"
)

// emit 是日志回调，用 printf 格式
type emitFn func(format string, args ...interface{})

type XXTExamOptions struct {
	SubmitThreshold    int
	RandomAnswerOnFail int
	AutoExamCode       int
	ExamCodes          map[string]string
	AccountUID         string
	AccountName        string
}

var xxtExamOptionsStore sync.Map

// sleepCtx 可被 ctx cancel 中断的 sleep
func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// formatXXTLog 生成标准学习通日志行，格式：
// [时间] [平台][账号] 【课程】【任务点】【资源/标题】消息
func formatXXTLog(platform, account, course, taskPoint, item, message string) string {
	return fmt.Sprintf("[%s] [%s][%s] 【%s】【%s】【%s】%s",
		time.Now().Format("2006-01-02 15:04:05"),
		platform, account, course, taskPoint, item, message)
}

// formatTaskPoint 将 KnowledgeItem 的 Label+Name 组合为任务点标签
func formatTaskPoint(k xuexitong.KnowledgeItem) string {
	label := strings.TrimSpace(k.Label)
	name := strings.TrimSpace(k.Name)
	if label != "" && name != "" {
		return label + " " + name
	}
	if name != "" {
		return name
	}
	return label
}

// SafeRun 执行学习任务，返回 error（不会 os.Exit/log.Fatal）
func SafeRun(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	submitThreshold int, randomAnswerOnFail int, emit emitFn) error {
	return SafeRunWithExamOptions(ctx, setting, user, cache, XXTExamOptions{
		SubmitThreshold:    submitThreshold,
		RandomAnswerOnFail: randomAnswerOnFail,
		AutoExamCode:       1,
	}, emit)
}

func SafeRunWithExamOptions(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	examOptions XXTExamOptions, emit emitFn) error {

	if emit == nil {
		emit = func(f string, a ...interface{}) {}
	}
	if examOptions.SubmitThreshold <= 0 {
		examOptions.SubmitThreshold = 100
	}
	if cache != nil {
		key := fmt.Sprintf("%p", cache)
		xxtExamOptionsStore.Store(key, examOptions)
		defer xxtExamOptionsStore.Delete(key)
	}
	submitThreshold := examOptions.SubmitThreshold
	randomAnswerOnFail := examOptions.RandomAnswerOnFail

	// 拉取课程
	courseList, err := xuexitong.XueXiTPullCourseAction(cache)
	if err != nil {
		return fmt.Errorf("拉取课程失败: %w", err)
	}

	sysLog := func(f string, a ...interface{}) {
		emit("[%s] [系统] %s", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(f, a...))
	}
	sysLog("已拉取课程总数：%d", len(courseList))
	for _, c := range courseList {
		sysLog("课程 [%s] courseId=%s key=%s isStart=%v state=%d",
			c.CourseName, c.CourseID, c.Key, c.IsStart, c.State)
	}

	// 创建账号级节点并发信号量
	nodeConcurrency := 1
	if user.CoursesCustom.CxNode != nil && *user.CoursesCustom.CxNode > 0 {
		nodeConcurrency = *user.CoursesCustom.CxNode
	}
	if nodeConcurrency > 8 {
		nodeConcurrency = 8
	}
	nodeSem := make(chan struct{}, nodeConcurrency)
	var nodeSemMu sync.Mutex
	nodeSemUsed := 0
	sysLog("学习通视频任务点并发上限：%d", nodeConcurrency)

	// videoModel=3 多任务点：预先登录多份 cache
	var model3Caches []xuexitongApi.XueXiTUserCache
	if user.CoursesCustom.VideoModel == 3 {
		num := nodeConcurrency
		sysLog("多任务点模式，将预登录 %d 份 cache，并由并发槽控制实际视频并发", num)
		for i := 0; i < num; i++ {
			cp := *cache
			if i > 0 {
				xuexitong.ReLogin(&cp)
			}
			model3Caches = append(model3Caches, cp)
		}
	}

	// 统计过滤结果
	var included, excluded, skipped int
	var wg sync.WaitGroup

	for i := range courseList {
		if ctx.Err() != nil {
			break
		}
		course := courseList[i] // 避免闭包捕获问题

		// 排除课程（按名称精确匹配）
		if len(user.CoursesCustom.ExcludeCourses) > 0 &&
			consoleConfig.CmpCourse(course.CourseName, user.CoursesCustom.ExcludeCourses) {
			// 只统计过滤结果，避免启动时大量跳过日志刷屏。
			excluded++
			continue
		}
		// 包含课程（按名称精确匹配；为空则全部学）
		if len(user.CoursesCustom.IncludeCourses) > 0 &&
			!consoleConfig.CmpCourse(course.CourseName, user.CoursesCustom.IncludeCourses) {
			// 只统计过滤结果，避免启动时大量跳过日志刷屏。
			skipped++
			continue
		}
		if !course.IsStart {
			// 只统计过滤结果，避免启动时大量跳过日志刷屏。
			skipped++
			continue
		}
		included++

		if user.CoursesCustom.VideoModel == 1 {
			// 串行
			safeRunCourse(ctx, setting, user, cache, &course, model3Caches, nodeSem, &nodeSemMu, &nodeSemUsed, submitThreshold, randomAnswerOnFail, emit)
		} else {
			// videoModel=2（并发课程）或 videoModel=3（并发任务点，课程层面仍并发）
			wg.Add(1)
			go func(c xuexitong.XueXiTCourse) {
				defer wg.Done()
				safeRunCourse(ctx, setting, user, cache, &c, model3Caches, nodeSem, &nodeSemMu, &nodeSemUsed, submitThreshold, randomAnswerOnFail, emit)
			}(course)
		}
	}
	wg.Wait()

	sysLog("过滤结果：包含 %d 课程，排除 %d，跳过 %d", included, excluded, skipped)
	sysLog("所有待学习课程执行完毕")
	return nil
}

func safeRunCourse(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse,
	model3Caches []xuexitongApi.XueXiTUserCache, nodeSem chan struct{}, nodeSemMu *sync.Mutex, nodeSemUsed *int,
	submitThreshold int, randomAnswerOnFail int, emit emitFn) {

	if ctx.Err() != nil {
		return
	}
	if course.State == 1 {
		return
	}

	// 章节学习
	if err := safeRunChapter(ctx, setting, user, cache, course, model3Caches, nodeSem, nodeSemMu, nodeSemUsed, submitThreshold, randomAnswerOnFail, emit); err != nil {
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", "章节学习错误: "+err.Error()))
	}
	// 作业/考试
	safeRunWorkAndExam(ctx, setting, user, cache, course, submitThreshold, emit)
	emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", "课程学习完毕"))
}

func safeRunChapter(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse,
	model3Caches []xuexitongApi.XueXiTUserCache, nodeSem chan struct{}, nodeSemMu *sync.Mutex, nodeSemUsed *int,
	submitThreshold int, randomAnswerOnFail int, emit emitFn) error {

	// 只在进度未完成且课程未结束时执行
	if !(course.JobRate < 100 && course.State != 1) {
		return nil
	}

	key, _ := strconv.Atoi(course.Key)
	action, _, err := xuexitong.PullCourseChapterAction(cache, course.Cpi, key)
	if err != nil {
		if strings.Contains(err.Error(), "课程章节为空") {
			return nil
		}
		return fmt.Errorf("拉取章节信息失败: %w", err)
	}

	if user.CoursesCustom.ShuffleSw == 1 {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(action.Knowledge), func(i, j int) {
			action.Knowledge[i], action.Knowledge[j] = action.Knowledge[j], action.Knowledge[i]
		})
	}

	// 章节数量只用于内部判断，不逐课程输出，避免启动时刷屏。

	var nodes []int
	for _, item := range action.Knowledge {
		nodes = append(nodes, item.ID)
	}

	courseId, _ := strconv.Atoi(course.CourseID)
	userId, _ := strconv.Atoi(cache.UserID)
	pointAction, err := xuexitong.ChapterFetchPointAction(cache, nodes, &action, key, userId, course.Cpi, courseId)
	if err != nil {
		return fmt.Errorf("探测节点完成情况失败: %w", err)
	}

	isFinished := func(index int) bool {
		if index < 0 || index >= len(pointAction.Knowledge) {
			return false
		}
		i := pointAction.Knowledge[index]
		if i.PointTotal == 0 && i.PointFinished == 0 {
			_ = xuexitong.EnterChapterForwardCallAction(cache,
				strconv.Itoa(courseId), strconv.Itoa(key),
				strconv.Itoa(pointAction.Knowledge[index].ID), strconv.Itoa(course.Cpi))
		}
		return i.PointTotal >= 0 && i.PointTotal == i.PointFinished
	}

	// 具体进度由视频提交日志体现。

	if user.CoursesCustom.VideoModel == 3 && len(model3Caches) > 0 {
		// 多任务点并发，用 nodeSem 控制并发
		var nodeLock sync.WaitGroup
		for index := range nodes {
			if ctx.Err() != nil {
				break
			}
			if isFinished(index) {
				// 已完成任务点只跳过，不逐条输出。
				continue
			}
			nodeLock.Add(1)
			go func(index int) {
				defer nodeLock.Done()
				var nodeCache xuexitongApi.XueXiTUserCache
				if len(model3Caches) > 0 {
					nodeCache = model3Caches[index%len(model3Caches)]
				} else {
					nodeCache = *cache
				}
				safeRunNode(ctx, setting, user, &nodeCache, course, pointAction, action, nodes, index, key, courseId, nodeSem, nodeSemMu, nodeSemUsed, submitThreshold, randomAnswerOnFail, emit)
			}(index)
		}
		nodeLock.Wait()
	} else {
		// videoModel=1 或 2（章节内串行任务点）
		for index := range nodes {
			if ctx.Err() != nil {
				break
			}
			if isFinished(index) {
				// 已完成任务点只跳过，不逐条输出。
				continue
			}
			safeRunNode(ctx, setting, user, cache, course, pointAction, action, nodes, index, key, courseId, nodeSem, nodeSemMu, nodeSemUsed, submitThreshold, randomAnswerOnFail, emit)
		}
	}
	return nil
}

func safeRunNode(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse,
	pointAction xuexitong.ChaptersList, action xuexitong.ChaptersList,
	nodes []int, index, key, courseId int, nodeSem chan struct{}, nodeSemMu *sync.Mutex, nodeSemUsed *int, submitThreshold int, randomAnswerOnFail int, emit emitFn) {

	if ctx.Err() != nil {
		return
	}

	// xlog：标准学习通日志，带任务点/资源名
	xlog := func(tp, item, msg string) {
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, tp, item, msg))
	}

	var curKnowledge xuexitong.KnowledgeItem
	if index >= 0 && index < len(pointAction.Knowledge) {
		curKnowledge = pointAction.Knowledge[index]
	}
	tp := formatTaskPoint(curKnowledge)

	defer func() {
		if r := recover(); r != nil {
			xlog(tp, "", fmt.Sprintf("任务点 panic: %v", r))
		}
	}()

	_, fetchCards, err := xuexitong.ChapterFetchCardsAction(cache, &action, nodes, index, courseId, key, course.Cpi)
	if err != nil {
		xlog(tp, "", "拉取卡片失败: "+err.Error())
		return
	}

	videoDTOs, workDTOs, documentDTOs, hyperlinkDTOs, liveDTOs, bbsDTOs := xuexitongApi.ParsePointDto(fetchCards)

	if videoDTOs == nil && workDTOs == nil && documentDTOs == nil &&
		hyperlinkDTOs == nil && liveDTOs == nil && bbsDTOs == nil {
		return
	}

	// 视频/音频
	if videoDTOs != nil && user.CoursesCustom.VideoModel != 0 {
		// acquire nodeSem
		nodeSem <- struct{}{}
		nodeSemMu.Lock()
		(*nodeSemUsed)++
		nodeSemMu.Unlock()
		defer func() {
			<-nodeSem
			nodeSemMu.Lock()
			if *nodeSemUsed > 0 {
				(*nodeSemUsed)--
			}
			nodeSemMu.Unlock()
		}()
		for _, v := range videoDTOs {
			if ctx.Err() != nil {
				return
			}
			card, enc, err := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, v.KnowledgeID, v.CardIndex, course.Cpi)
			if err != nil {
				if strings.Contains(err.Error(), "没有历史人脸") ||
					strings.Contains(err.Error(), "活体检测失败") {
					xlog(tp, "", "人脸识别失败（"+err.Error()+"），跳过此任务点")
					return
				}
				if strings.Contains(err.Error(), "章节未开放") {
					xlog(tp, "", "章节未开放，跳过")
					break
				}
				xlog(tp, "", "视频卡片加载失败: "+err.Error())
				return
			}
			v.AttachmentsDetection(card)
			if !v.IsJob {
				continue
			}
			v.Enc = enc
			xxtLogic.ExecuteVideo(user, cache, course, curKnowledge, &v, key, course.Cpi)
			sleepCtx(ctx, time.Duration(rand.Intn(51)+10)*time.Second)
		}
	}

	// 文档
	if documentDTOs != nil && user.CoursesCustom.VideoModel != 0 {
		for _, d := range documentDTOs {
			if ctx.Err() != nil {
				return
			}
			card, _, err := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, d.KnowledgeID, d.CardIndex, course.Cpi)
			if err != nil {
				if strings.Contains(err.Error(), "章节未开放") {
					break
				}
				if strings.Contains(err.Error(), "没有历史人脸") {
					xlog(tp, "", "人脸识别失败，跳过")
					return
				}
				xlog(tp, "", "文档卡片加载失败: "+err.Error())
				return
			}
			d.AttachmentsDetection(card)
			if !d.IsJob {
				continue
			}
			xxtLogic.ExecuteDocument(user, cache, course, curKnowledge, &d)
			sleepCtx(ctx, 5*time.Second)
		}
	}

	// 章节测验（workDTOs）
	cxChapterSw := 1
	if user.CoursesCustom.CxChapterTestSw != nil {
		cxChapterSw = *user.CoursesCustom.CxChapterTestSw
	}
	if workDTOs == nil {
		// 无章节测验，静默跳过
	} else if user.CoursesCustom.AutoExam == 0 {
		// 自动答题关闭时不逐条输出章节测验跳过日志。
	} else if cxChapterSw == 0 {
		// 章节测验关闭时不逐条输出跳过日志。
	} else {
		xlog(tp, "", fmt.Sprintf("检测到章节测验，共 %d 个", len(workDTOs)))
		for _, wd := range workDTOs {
			if ctx.Err() != nil {
				return
			}
			mobileCard, _, err2 := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, wd.KnowledgeID, wd.CardIndex, course.Cpi)
			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					xlog(tp, "", "章节测验章节未开放，跳过")
					continue
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					xlog(tp, "", "人脸识别失败，跳过章节测验")
					return
				}
				xlog(tp, "", "章节测验卡片加载失败: "+err2.Error())
				continue
			}
			flag, _ := wd.AttachmentsDetection(mobileCard)
			qa, err3 := xuexitong.ParseWorkQuestionAction(cache, &wd)
			if err3 != nil {
				if strings.Contains(err3.Error(), "已截止，不能作答") {
					xlog(tp, "", "章节测验已截止，跳过")
					continue
				}
				xlog(tp, "", "解析章节测验题目失败: "+err3.Error())
				continue
			}
			if !flag {
				continue
			}
			hasQ := len(qa.Short) + len(qa.Choice) + len(qa.Judge) + len(qa.Fill) + len(qa.TermExplanation) + len(qa.Essay)
			if hasQ == 0 {
				continue
			}
			safeChapterTestAction(ctx, setting, user, cache, course, curKnowledge, qa, submitThreshold, randomAnswerOnFail, emit)
		}
	}

	// 外链
	if hyperlinkDTOs != nil && user.CoursesCustom.VideoModel != 0 {
		for _, h := range hyperlinkDTOs {
			if ctx.Err() != nil {
				return
			}
			card, _, err := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, h.KnowledgeID, h.CardIndex, course.Cpi)
			if err != nil {
				if strings.Contains(err.Error(), "章节未开放") {
					break
				}
				if strings.Contains(err.Error(), "没有历史人脸") {
					xlog(tp, "", "人脸识别失败，跳过")
					return
				}
				xlog(tp, "", "外链卡片加载失败: "+err.Error())
				return
			}
			h.AttachmentsDetection(card)
			xxtLogic.ExecuteHyperlink(user, cache, course, curKnowledge, &h)
			sleepCtx(ctx, 5*time.Second)
		}
	}

	// 直播
	if liveDTOs != nil && user.CoursesCustom.VideoModel != 0 {
		for _, l := range liveDTOs {
			if ctx.Err() != nil {
				return
			}
			card, _, err := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, l.KnowledgeID, l.CardIndex, course.Cpi)
			if err != nil {
				if strings.Contains(err.Error(), "章节未开放") {
					break
				}
				if strings.Contains(err.Error(), "没有历史人脸") {
					xlog(tp, "", "人脸识别失败，跳过")
					return
				}
				xlog(tp, "", "直播卡片加载失败: "+err.Error())
				return
			}
			l.AttachmentsDetection(card)
			if !l.IsJob {
				continue
			}
			xxtLogic.ExecuteLive(user, cache, course, curKnowledge, &l)
			sleepCtx(ctx, 5*time.Second)
		}
	}

	// 讨论（BBS）
	if bbsDTOs != nil && user.CoursesCustom.AutoExam != 0 {
		for _, b := range bbsDTOs {
			if ctx.Err() != nil {
				return
			}
			card, _, err := xuexitong.PageMobileChapterCardAction(
				cache, key, courseId, b.KnowledgeID, b.CardIndex, course.Cpi)
			if err != nil {
				if strings.Contains(err.Error(), "章节未开放") {
					break
				}
				if strings.Contains(err.Error(), "没有历史人脸") {
					xlog(tp, "", "人脸识别失败，跳过")
					return
				}
				xlog(tp, "", "讨论卡片加载失败: "+err.Error())
				return
			}
			b.AttachmentsDetection(card)
			if !b.IsJob {
				continue
			}
			xxtLogic.ExecuteBBS(user, cache, setting, course, curKnowledge, &b)
			sleepCtx(ctx, 5*time.Second)
		}
	}
}

// safeRunWorkAndExam 安全版作业/考试执行
func safeRunWorkAndExam(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse, submitThreshold int, emit emitFn) {

	getInt := func(p *int, d int) int {
		if p == nil {
			return d
		}
		return *p
	}
	cxWork := getInt(user.CoursesCustom.CxWorkSw, 1)
	cxExam := getInt(user.CoursesCustom.CxExamSw, 1)

	xlog := func(item, msg string) {
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", item, msg))
	}
	examOptions := xxtOptionsFor(cache, submitThreshold)

	if user.CoursesCustom.AutoExam == 0 {
		return
	}
	if ctx.Err() != nil {
		return
	}

	// 检查 AI/题库可用性
	if user.CoursesCustom.AutoExam == 1 {
		if err := aiq.AICheck(setting.AiSetting.AiUrl, setting.AiSetting.Model,
			setting.AiSetting.APIKEY, setting.AiSetting.AiType); err != nil {
			xlog("", fmt.Sprintf("AI（%s）不可用: %s", setting.AiSetting.AiType, err.Error()))
			return
		}
	} else if user.CoursesCustom.AutoExam == 2 {
		if !hasExternalQuestionBank(setting) {
			xlog("", "外部题库未配置，跳过作业/考试自动答题")
			return
		}
	}

	// 作业
	if cxWork == 1 {
		workList, err := xuexitong.PullWorkListAction(cache, *course)
		if err != nil {
			xlog("", "拉取作业列表失败，跳过: "+err.Error())
		} else {
			for _, work := range workList {
				if ctx.Err() != nil {
					return
				}
				if !(work.Status == "待做" || work.Status == "未交" || work.Status == "待重做") {
					continue
				}
				xlog(work.Name, "检测到待做作业")
				if err2 := xuexitong.EnterWorkAction(cache, &work); err2 != nil {
					xlog(work.Name, "进入作业失败: "+err2.Error())
					continue
				}
				safeWorkAction(ctx, setting, user, cache, course, work, emit)
			}
		}
	}

	// 考试
	if cxExam == 1 {
		examList, err := xuexitong.PullExamListAction(cache, *course)
		if err != nil {
			xlog("", "拉取考试列表失败，跳过: "+err.Error())
		} else {
			for _, exam := range examList {
				if ctx.Err() != nil {
					return
				}
				if exam.Status != "待做" && exam.Status != "待重考" {
					continue
				}
				xlog(exam.Name, "检测到待做考试")
				examCode := xxtResolveExamCode(cache, &exam, examOptions, xlog)
				if err2 := enterXXTExamWithCode(cache, &exam, examCode, examOptions, xlog); err2 != nil {
					xlog(exam.Name, "enter exam failed: "+err2.Error())
					continue
				}
				safeExamAction(ctx, setting, user, cache, course, exam, examOptions, emit)
			}
		}
	}
}

func safeWorkAction(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse, work xuexitong.XXTWork, emit emitFn) {
	xlog := func(msg string) {
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", work.Name, msg))
	}
	xlog(fmt.Sprintf("开始写作业（共 %d 题）", work.QuestionTotal))
	for i := range work.QuestionTotal {
		if ctx.Err() != nil {
			return
		}
		question, err := work.PullWorkQuestionAction(cache, i)
		if err != nil {
			xlog("拉取作业题目失败: " + err.Error())
			return
		}
		xlog(fmt.Sprintf("正在回答第 %d 题", i+1))
		switch user.CoursesCustom.AutoExam {
		case 1:
			if err3 := question.WriteQuestionForAIAction(cache, setting.AiSetting.AiUrl,
				setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY); err3 != nil {
				xlog("AI回答作业失败: " + err3.Error())
			}
		case 2:
			answerExternalWorkQuestion(setting, &question, course.CourseName, xlog)
		case 3:
			if err3 := question.WriteQuestionForXXTAIAction(cache, question.ClassId, question.CourseId, question.Cpi); err3 != nil {
				xlog("内置AI回答作业失败: " + err3.Error())
			}
		}
		isLast := work.QuestionTotal == i+1
		shouldSubmit := isLast && (user.CoursesCustom.ExamAutoSubmit == 1 || user.CoursesCustom.ExamAutoSubmit == 2)
		result, err := question.SubmitWorkAnswerAction(cache, shouldSubmit)
		if err != nil {
			xlog("作业提交失败: " + err.Error())
		} else {
			xlog(fmt.Sprintf("第 %d 题提交成功: %s", i+1, result))
		}
	}
	xlog("作业完成")
}

func safeExamAction(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse, exam xuexitong.XXTExam, examOptions XXTExamOptions, emit emitFn) {
	xlog := func(msg string) {
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", exam.Name, msg))
	}
	xlog(fmt.Sprintf("开始考试（共 %d 题）", exam.QuestionTotal))
	for i := range exam.QuestionTotal {
		if ctx.Err() != nil {
			return
		}
		question, err := exam.PullExamQuestionAction(cache, i)
		if err != nil {
			xlog("拉取考试题目失败: " + err.Error())
			return
		}
		xlog(fmt.Sprintf("正在回答第 %d/%d 题", i+1, exam.QuestionTotal))
		switch user.CoursesCustom.AutoExam {
		case 1:
			if err3 := question.WriteQuestionForAIAction(cache, setting.AiSetting.AiUrl,
				setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY); err3 != nil {
				xlog("AI回答考试失败: " + err3.Error())
			}
		case 2:
			answerExternalExamQuestion(setting, &question, course.CourseName, xlog)
		case 3:
			if err3 := question.WriteQuestionForXXTAIAction(cache, question.ClassId, question.CourseId, question.Cpi); err3 != nil {
				xlog("内置AI回答考试失败: " + err3.Error())
			}
		}
		isLast := exam.QuestionTotal == i+1
		shouldSubmit := isLast && (user.CoursesCustom.ExamAutoSubmit == 1 || user.CoursesCustom.ExamAutoSubmit == 2)
		result, err := question.SubmitExamAnswerAction(cache, shouldSubmit)
		if err != nil {
			xlog("考试答案提交失败: " + err.Error())
		} else {
			xlog(fmt.Sprintf("第 %d 题提交: %s", i+1, result))
		}
	}
	xlog("考试完成")
}

// hasAIConfig 判断 AI 答题是否配置完整

func xxtOptionsFor(cache *xuexitongApi.XueXiTUserCache, submitThreshold int) XXTExamOptions {
	opts := XXTExamOptions{SubmitThreshold: submitThreshold, AutoExamCode: 1}
	if cache != nil {
		if v, ok := xxtExamOptionsStore.Load(fmt.Sprintf("%p", cache)); ok {
			if vv, ok2 := v.(XXTExamOptions); ok2 {
				opts = vv
			}
		}
	}
	if opts.SubmitThreshold <= 0 {
		opts.SubmitThreshold = 100
	}
	return opts
}

func xxtResolveExamCode(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, opts XXTExamOptions, xlog func(item, msg string)) (code string) {
	defer func() {
		if r := recover(); r != nil {
			xlog(exam.Name, fmt.Sprintf("auto exam code panic: %v", r))
			code = xxtExamCodeFor(exam.Name, opts)
		}
	}()
	if opts.AutoExamCode == 1 {
		xlog(exam.Name, "auto exam code: start")
		code, source := xxtAutoFetchExamCode(cache, exam, func(msg string) { xlog(exam.Name, msg) })
		if code != "" {
			xlog(exam.Name, "auto exam code found source="+source)
			return code
		}
		if fallback := xxtExamCodeFor(exam.Name, opts); fallback != "" {
			xlog(exam.Name, "auto exam code not found, using configured fallback")
			return fallback
		}
		xlog(exam.Name, "auto exam code not found")
		return ""
	}
	code = xxtExamCodeFor(exam.Name, opts)
	if code != "" {
		xlog(exam.Name, "using configured exam code")
	}
	return code
}

func xxtExamCodeFor(examName string, opts XXTExamOptions) string {
	name := strings.TrimSpace(examName)
	for k, v := range opts.ExamCodes {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" && strings.Contains(name, strings.TrimSpace(k)) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func xxtAutoFetchExamCode(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, diag func(string)) (string, string) {
	if exam == nil {
		return "", ""
	}
	if diag == nil {
		diag = func(string) {}
	}
	diag(fmt.Sprintf("auto exam code: rawURL=%s taskRefId=%s answerId=%s", xxtShortText(exam.RawURL, 120), exam.TaskRefId, exam.AnswerId))
	if code, src := xxtExtractExamCodeFromParams(exam.Params); code != "" {
		return code, src
	}
	if code, src := xxtExtractExamCode(exam.RawURL); code != "" {
		return code, "raw_url:" + src
	}
	if cache != nil && strings.TrimSpace(exam.TaskRefId) != "" {
		enterURL := xxtBuildExamEnterURL(exam)
		diag("auto exam code: fetching enter page with timeout")
		html, finalURL, err := xxtExamGETWithFinalURL(cache, enterURL, "", 12*time.Second)
		if err != nil {
			diag("auto exam code: enter fetch failed: " + err.Error())
			return "", ""
		}
		diag("auto exam code: enter finalURL=" + xxtShortText(finalURL, 160))
		xxtFillExamFieldsFromRaw(exam, finalURL)
		xxtFillExamFieldsFromRaw(exam, html)
		if dumpPath, err := xxtSaveExamEnterDebugDump(exam, enterURL, finalURL, html); err == nil && dumpPath != "" {
			diag("auto exam code: saved enter html=" + dumpPath)
		}
		if code, src := xxtExtractExamCode(finalURL); code != "" {
			return code, "enter_final_url:" + src
		}
		if code, src := xxtExtractExamCode(html); code != "" {
			return code, "enter_html:" + src
		}
		if xxtRequiresManualExamCode(html) {
			diag("auto exam code: page requires code but input value is blank")
		}
		if jumpURL := xxtExtractExamJumpURL(html); jumpURL != "" {
			diag("auto exam code: follow examJumpUrl=" + xxtShortText(jumpURL, 160))
			jumpHTML, jumpFinalURL, jumpErr := xxtExamGETWithFinalURL(cache, xxtAbsoluteXXTURL(jumpURL), finalURL, 12*time.Second)
			if jumpErr != nil {
				diag("auto exam code: examJumpUrl fetch failed: " + jumpErr.Error())
			} else {
				diag("auto exam code: examJumpUrl finalURL=" + xxtShortText(jumpFinalURL, 160))
				xxtFillExamFieldsFromRaw(exam, jumpFinalURL)
				xxtFillExamFieldsFromRaw(exam, jumpHTML)
				if dumpPath, err := xxtSaveExamEnterDebugDump(exam, xxtAbsoluteXXTURL(jumpURL), jumpFinalURL, jumpHTML); err == nil && dumpPath != "" {
					diag("auto exam code: saved jump html=" + dumpPath)
				}
				if code, src := xxtExtractExamCode(jumpFinalURL); code != "" {
					return code, "exam_jump_url:" + src
				}
				if code, src := xxtExtractExamCode(jumpHTML); code != "" {
					return code, "exam_jump_html:" + src
				}
			}
		}
		diag("auto exam code: no code in enter page, html=" + xxtShortText(html, 200))
	}
	return "", ""
}

func xxtBuildExamEnterURL(exam *xuexitong.XXTExam) string {
	q := url.Values{}
	q.Set("taskrefId", exam.TaskRefId)
	q.Set("msgId", exam.MsgId)
	q.Set("courseId", exam.CourseId)
	q.Set("userId", exam.UserId)
	q.Set("clazzId", exam.ClazzId)
	q.Set("type", exam.Type)
	q.Set("enc_task", exam.EncTask)
	return "https://mooc1-api.chaoxing.com/exam-ans/android/mtaskmsgspecial?" + q.Encode()
}

func xxtExtractExamCodeFromParams(params map[string]string) (string, string) {
	for _, key := range []string{"code", "examCode", "exam_code", "examPwd", "examPassword", "examPasswordCode", "password", "pwd", "verifyCode", "examVerifyCode", "validateCode"} {
		if v := strings.TrimSpace(params[key]); xxtLooksLikeExamCode(v) {
			return v, "param:" + key
		}
	}
	for k, v := range params {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "code") || strings.Contains(lk, "pwd") {
			if vv := strings.TrimSpace(v); xxtLooksLikeExamCode(vv) {
				return vv, "param:" + k
			}
		}
	}
	return "", ""
}

func xxtExtractExamCodeFromValues(values url.Values) (string, string) {
	for _, key := range []string{"code", "examCode", "exam_code", "examPwd", "examPassword", "examPasswordCode", "verifyCode", "examVerifyCode", "validateCode"} {
		if v := strings.TrimSpace(values.Get(key)); xxtLooksLikeExamCode(v) {
			return v, key
		}
	}
	for key, vals := range values {
		lk := strings.ToLower(key)
		if !(strings.Contains(lk, "examcode") || strings.Contains(lk, "exam_code") || strings.Contains(lk, "exampwd") || strings.Contains(lk, "verifycode") || strings.Contains(lk, "validatecode")) {
			continue
		}
		for _, v := range vals {
			if vv := strings.TrimSpace(v); xxtLooksLikeExamCode(vv) {
				return vv, key
			}
		}
	}
	return "", ""
}

func xxtExtractExamCode(raw string) (string, string) {
	if strings.TrimSpace(raw) == "" {
		return "", ""
	}
	if u, err := url.Parse(raw); err == nil {
		if code, src := xxtExtractExamCodeFromValues(u.Query()); code != "" {
			return code, "query:" + src
		}
	}
	patterns := []string{
		`(?i)(?:examCode|examPwd|examPassword|verifyCode|validateCode|code|pwd|password)["'\s:=]+([A-Za-z][A-Za-z0-9_-]{5,31})`,
		`\bt\d{6,10}\b`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(raw)
		if len(m) > 1 && xxtLooksLikeExamCode(m[1]) {
			return strings.TrimSpace(m[1]), "regex"
		}
		if len(m) == 1 && xxtLooksLikeExamCode(m[0]) {
			return strings.TrimSpace(m[0]), "regex"
		}
	}
	return "", ""
}

func xxtExtractExamJumpURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	patterns := []string{
		`(?is)<input[^>]+id=["']examJumpUrl["'][^>]+value=["']([^"']+)["']`,
		`(?is)<input[^>]+value=["']([^"']+)["'][^>]+id=["']examJumpUrl["']`,
		`(?is)examJumpUrl[^>]+value=["']([^"']+)["']`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(raw); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(strings.ReplaceAll(m[1], "&amp;", "&"))
		}
	}
	return ""
}

func xxtRequiresManualExamCode(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	return regexp.MustCompile(`(?is)var\s+needcode\s*=\s*1\b`).MatchString(raw) ||
		regexp.MustCompile(`(?is)<input[^>]+id=["']code["'][^>]*placeholder=["'][^"']*考试码`).MatchString(raw)
}

func xxtAbsoluteXXTURL(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "&amp;", "&"))
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return "https://mooc1-api.chaoxing.com" + raw
	}
	return "https://mooc1-api.chaoxing.com/" + raw
}

func xxtSaveExamEnterDebugDump(exam *xuexitong.XXTExam, requestURL, finalURL, html string) (string, error) {
	if exam == nil || strings.TrimSpace(html) == "" {
		return "", nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	debugDir := filepath.Join(dir, "debug", "xxt-exam")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return "", err
	}
	name := xxtSafeFilePart(exam.Name)
	if name == "" {
		name = "exam"
	}
	task := xxtSafeFilePart(exam.TaskRefId)
	if task == "" {
		task = "task"
	}
	path := filepath.Join(debugDir, fmt.Sprintf("%s-%s.html", name, task))
	content := strings.Builder{}
	content.WriteString("<!-- Yatori XXT exam auto-code debug dump\n")
	content.WriteString("examName: " + exam.Name + "\n")
	content.WriteString("taskRefId: " + exam.TaskRefId + "\n")
	content.WriteString("answerId: " + exam.AnswerId + "\n")
	content.WriteString("courseId: " + exam.CourseId + "\n")
	content.WriteString("classId: " + exam.ClazzId + "\n")
	content.WriteString("cpi: " + exam.Cpi + "\n")
	content.WriteString("requestURL: " + requestURL + "\n")
	content.WriteString("finalURL: " + finalURL + "\n")
	content.WriteString("-->\n")
	content.WriteString(html)
	return path, os.WriteFile(path, []byte(content.String()), 0644)
}

func xxtSafeFilePart(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 80 {
			break
		}
	}
	return strings.Trim(b.String(), "._- ")
}

func xxtFillExamFieldsFromRaw(exam *xuexitong.XXTExam, raw string) {
	if exam == nil || strings.TrimSpace(raw) == "" {
		return
	}
	if u, err := url.Parse(raw); err == nil {
		q := u.Query()
		xxtSetIfEmpty(&exam.ExamRelationId, q.Get("examId"))
		xxtSetIfEmpty(&exam.ExamRelationId, q.Get("testPaperId"))
		xxtSetIfEmpty(&exam.AnswerId, q.Get("examAnswerId"))
		xxtSetIfEmpty(&exam.AnswerId, q.Get("testUserRelationId"))
		xxtSetIfEmpty(&exam.Cpi, q.Get("cpi"))
	}
	for _, pair := range [][2]string{{"captchaCaptchaId", "CaptchaCaptchaId"}, {"testPaperId", "ExamRelationId"}, {"testUserRelationId", "AnswerId"}, {"cpi", "Cpi"}, {"enc", "Enc"}, {"encRemainTime", "EncRemainTime"}, {"encLastUpdateTime", "EncLastUpdateTime"}} {
		re := regexp.MustCompile(`(?is)<input[^>]+(?:id|name)=["']` + regexp.QuoteMeta(pair[0]) + `["'][^>]+value=["']([^"']*)["']`)
		if m := re.FindStringSubmatch(raw); len(m) > 1 {
			switch pair[1] {
			case "CaptchaCaptchaId":
				xxtSetIfEmpty(&exam.CaptchaCaptchaId, m[1])
			case "ExamRelationId":
				xxtSetIfEmpty(&exam.ExamRelationId, m[1])
			case "AnswerId":
				xxtSetIfEmpty(&exam.AnswerId, m[1])
			case "Cpi":
				xxtSetIfEmpty(&exam.Cpi, m[1])
			case "Enc":
				xxtSetIfEmpty(&exam.Enc, m[1])
			case "EncRemainTime":
				xxtSetIfEmpty(&exam.EncRemainTime, m[1])
			case "EncLastUpdateTime":
				xxtSetIfEmpty(&exam.EncLastUpdateTime, m[1])
			}
		}
	}
}

func xxtSetIfEmpty(dst *string, v string) {
	if dst != nil && strings.TrimSpace(*dst) == "" && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}

func xxtLooksLikeExamCode(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 4 || len(v) > 32 {
		return false
	}
	if strings.ContainsAny(v, " \t\r\n/?&=#<>\"'") {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Za-z][A-Za-z0-9_-]+$`, v)
	return matched
}

func enterXXTExamWithCode(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, examCode string, opts XXTExamOptions, xlog func(item, msg string)) error {
	if strings.TrimSpace(examCode) != "" {
		if err := xxtEnterExamWithCodeDirect(cache, exam, examCode, xlog); err == nil && xxtExamReady(exam) {
			return nil
		} else if err != nil {
			xlog(exam.Name, "direct exam-code entry failed: "+err.Error())
		}
	}
	err := xuexitong.EnterExamAction(cache, exam)
	if err != nil {
		if strings.TrimSpace(examCode) == "" {
			xlog(exam.Name, "exam needs code, waiting for user input")
			if code, waitErr := WaitForXXTExamCode(opts.AccountUID, opts.AccountName, exam.Name, exam.TaskRefId, 10*time.Minute); waitErr == nil && strings.TrimSpace(code) != "" {
				xlog(exam.Name, "received exam code from popup")
				return xxtEnterExamWithCodeDirect(cache, exam, code, xlog)
			} else if waitErr != nil {
				xlog(exam.Name, "wait exam code failed: "+waitErr.Error())
			}
			return err
		}
		xlog(exam.Name, "enter failed, retry with exam code: "+err.Error())
		if err2 := tryXXTExamCode(cache, exam, examCode); err2 != nil {
			return err2
		}
		err = xuexitong.EnterExamAction(cache, exam)
		if err != nil {
			return err
		}
	}
	if xxtExamReady(exam) {
		return nil
	}
	if strings.TrimSpace(examCode) == "" {
		xlog(exam.Name, "exam needs code, waiting for user input")
		if code, waitErr := WaitForXXTExamCode(opts.AccountUID, opts.AccountName, exam.Name, exam.TaskRefId, 10*time.Minute); waitErr == nil && strings.TrimSpace(code) != "" {
			xlog(exam.Name, "received exam code from popup")
			return xxtEnterExamWithCodeDirect(cache, exam, code, xlog)
		} else if waitErr != nil {
			xlog(exam.Name, "wait exam code failed: "+waitErr.Error())
		}
		msg := fmt.Sprintf("exam entry incomplete and no exam code: paper=%s answer=%s cpi=%s encLen=%d", exam.ExamRelationId, exam.AnswerId, exam.Cpi, len(exam.Enc))
		xlog(exam.Name, msg)
		return fmt.Errorf("%s", msg)
	}
	xlog(exam.Name, "exam entry incomplete, trying exam code validation")
	if err := xxtEnterExamWithCodeDirect(cache, exam, examCode, xlog); err != nil {
		if err2 := tryXXTExamCode(cache, exam, examCode); err2 != nil {
			return err
		}
		err = xuexitong.EnterExamAction(cache, exam)
		if err != nil {
			return err
		}
	}
	if !xxtExamReady(exam) {
		msg := fmt.Sprintf("exam code validation done but entry still incomplete: paper=%s answer=%s cpi=%s encLen=%d", exam.ExamRelationId, exam.AnswerId, exam.Cpi, len(exam.Enc))
		xlog(exam.Name, msg)
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func xxtEnterExamWithCodeDirect(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, examCode string, xlog func(item, msg string)) error {
	if cache == nil || exam == nil {
		return fmt.Errorf("cache or exam is nil")
	}
	code := strings.TrimSpace(examCode)
	if code == "" {
		return fmt.Errorf("exam code is empty")
	}
	enterURL := xxtBuildExamEnterURL(exam)
	enterHTML, refererURL, err := xxtExamGETWithFinalURL(cache, enterURL, "", 12*time.Second)
	if err != nil {
		return err
	}
	xxtFillExamFieldsFromRaw(exam, refererURL)
	xxtFillExamFieldsFromRaw(exam, enterHTML)
	if exam.QuestionTotal <= 0 {
		exam.QuestionTotal = xxtExtractExamQuestionTotal(enterHTML)
	}
	if xlog != nil {
		xlog(exam.Name, fmt.Sprintf("direct exam-code entry: paper=%s answer=%s cpi=%s captcha=%v", exam.ExamRelationId, exam.AnswerId, exam.Cpi, exam.CaptchaCaptchaId != ""))
	}
	if strings.TrimSpace(exam.ExamRelationId) == "" || strings.TrimSpace(exam.AnswerId) == "" || strings.TrimSpace(exam.Cpi) == "" {
		return fmt.Errorf("entry page missing required ids")
	}
	if strings.TrimSpace(exam.CaptchaCaptchaId) != "" {
		slider := xuexitong.XueXiTSlider{
			CaptchaId: exam.CaptchaCaptchaId,
			Referer:   refererURL,
		}
		var validate string
		var passErr error
		for i := 0; i < 5; i++ {
			validate, passErr = slider.Pass(cache)
			if passErr == nil && strings.TrimSpace(validate) != "" {
				exam.Validate = validate
				break
			}
			if passErr != nil && !strings.Contains(passErr.Error(), `"result":false`) {
				break
			}
		}
		if strings.TrimSpace(exam.Validate) == "" && passErr != nil {
			return fmt.Errorf("captcha pass failed: %w", passErr)
		}
	}
	paperHTML, err := xxtPullExamPaperWithCode(cache, exam, code, refererURL)
	if err != nil {
		return err
	}
	lower := strings.ToLower(paperHTML)
	if strings.Contains(paperHTML, "考试码错误") || strings.Contains(paperHTML, "考试码不正确") || strings.Contains(paperHTML, "考试码为空") ||
		strings.Contains(lower, "code error") {
		return fmt.Errorf("exam code rejected: %s", xxtShortText(paperHTML, 160))
	}
	if dumpPath, dumpErr := xxtSaveExamEnterDebugDump(exam, "phone/start with code", refererURL, paperHTML); dumpErr == nil && dumpPath != "" && xlog != nil {
		xlog(exam.Name, "direct exam-code entry: saved paper html="+dumpPath)
	}
	if exam.QuestionTotal <= 0 {
		exam.QuestionTotal = xxtExtractExamQuestionTotal(paperHTML)
	}
	qsEntity, err := xuexitong.HtmlQuestionTurnEntity(paperHTML)
	if err != nil {
		return err
	}
	exam.Enc = qsEntity.Enc
	exam.EncRemainTime = qsEntity.EncRemainTime
	exam.EncLastUpdateTime = qsEntity.EncLastUpdateTime
	if !xxtExamReady(exam) {
		return fmt.Errorf("paper parsed but missing enc fields: encLen=%d remain=%s update=%s", len(exam.Enc), exam.EncRemainTime, exam.EncLastUpdateTime)
	}
	return nil
}

func xxtExtractExamQuestionTotal(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	patterns := []string{
		`当前第\s*\d+\s*题\s*[;；]\s*共\s*(\d+)\s*题`,
		`共\s*(\d+)\s*题`,
		`(\d+)\s*/\s*(\d+)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(raw); len(m) > 1 {
			idx := 1
			if strings.Contains(p, `/`) && len(m) > 2 {
				idx = 2
			}
			n, _ := strconv.Atoi(m[idx])
			if n > 0 && n < 1000 {
				return n
			}
		}
	}
	return 0
}

func xxtPullExamPaperWithCode(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, examCode, referer string) (string, error) {
	q := url.Values{}
	q.Set("courseId", exam.CourseId)
	q.Set("classId", exam.ClazzId)
	q.Set("examId", exam.ExamRelationId)
	q.Set("source", "0")
	q.Set("examAnswerId", exam.AnswerId)
	q.Set("cpi", exam.Cpi)
	q.Set("keyboardDisplayRequiresUserAction", "1")
	q.Set("imei", xuexitongApi.IMEI)
	q.Set("captchavalidate", exam.Validate)
	q.Set("jt", "0")
	q.Set("_v", fmt.Sprintf("%.16f", rand.Float64()))
	q.Set("code", strings.TrimSpace(examCode))
	urlStr := "https://mooc1-api.chaoxing.com/exam-ans/exam/phone/start?" + q.Encode() + "&faceDetectionResult&cxcid&cxtime&signt&_signcode=3&_signc=0&_signe=3-1&signk"
	return xxtExamGET(cache, urlStr, referer)
}

func xxtExamReady(exam *xuexitong.XXTExam) bool {
	return strings.TrimSpace(exam.ExamRelationId) != "" &&
		strings.TrimSpace(exam.AnswerId) != "" &&
		strings.TrimSpace(exam.Cpi) != "" &&
		strings.TrimSpace(exam.Enc) != "" &&
		strings.TrimSpace(exam.EncRemainTime) != "" &&
		strings.TrimSpace(exam.EncLastUpdateTime) != ""
}

func tryXXTExamCode(cache *xuexitongApi.XueXiTUserCache, exam *xuexitong.XXTExam, examCode string) error {
	code := url.QueryEscape(strings.TrimSpace(examCode))
	baseParams := "examId=" + url.QueryEscape(exam.TaskRefId) +
		"&examAnswerId=" + url.QueryEscape(exam.AnswerId) +
		"&courseId=" + url.QueryEscape(exam.CourseId) +
		"&classId=" + url.QueryEscape(exam.ClazzId) +
		"&source=0&code=" + code
	candidates := []string{
		"https://mooc1-api.chaoxing.com/exam-ans/exam/phone/startOp?" + baseParams,
		"https://mooc1-api.chaoxing.com/exam-ans/exam/phone/restartOp?" + baseParams,
		"https://mooc1-api.chaoxing.com/exam-ans/exam/phone/task-exam?courseId=" + url.QueryEscape(exam.CourseId) +
			"&classId=" + url.QueryEscape(exam.ClazzId) +
			"&taskrefId=" + url.QueryEscape(exam.TaskRefId) +
			"&examAnswerId=" + url.QueryEscape(exam.AnswerId) +
			"&cpi=" + url.QueryEscape(exam.Cpi) +
			"&ut=s&code=" + code + "&reset=true&protocol_v=1",
	}
	var lastErr error
	for _, u := range candidates {
		body, err := xxtExamGET(cache, u, "")
		if err != nil {
			lastErr = err
			continue
		}
		if xxtExamCodeAccepted(body) {
			return nil
		}
		lower := strings.ToLower(body)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "false") || strings.Contains(body, "??") || strings.Contains(body, "??") {
			lastErr = fmt.Errorf("exam code endpoint rejected: %s", xxtShortText(body, 160))
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return nil
}

func xxtExamCodeAccepted(body string) bool {
	s := strings.TrimSpace(body)
	if s == "" {
		return true
	}
	if strings.Contains(s, `"status":true`) || strings.Contains(s, `"success":true`) || strings.Contains(s, `"result":true`) || strings.Contains(s, `"code":0`) || strings.Contains(s, "reVersionTestStartNew") || strings.Contains(s, "exam/phone/start") || strings.Contains(s, "questionWrap") || strings.Contains(s, "testPaperId") {
		return true
	}
	if strings.Contains(strings.ToLower(s), "code error") || strings.Contains(strings.ToLower(s), "error") || strings.Contains(s, "?????") || strings.Contains(s, "????") || strings.Contains(s, "?????") {
		return false
	}
	return len(s) < 20
}

func xxtExamGETWithFinalURL(cache *xuexitongApi.XueXiTUserCache, urlStr, referer string, timeout time.Duration) (string, string, error) {
	finalURL := ""
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			finalURL = req.URL.String()
			if len(via) > 0 {
				for _, c := range via[0].Cookies() {
					req.AddCookie(c)
				}
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", xuexitongApi.XXTEXAMUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,application/json,text/javascript,*/*;q=0.8")
	req.Header.Set("X-Requested-With", "com.chaoxing.mobile")
	req.Header.Set("Accept-Language", "zh-CN,en-US;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	for _, c := range cache.GetCookies() {
		req.AddCookie(c)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", finalURL, err
	}
	defer res.Body.Close()
	if finalURL == "" && res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL.String()
	}
	body, _ := io.ReadAll(res.Body)
	return string(body), finalURL, nil
}

func xxtExamGET(cache *xuexitongApi.XueXiTUserCache, urlStr, referer string) (string, error) {
	body, _, err := xxtExamGETWithFinalURL(cache, urlStr, referer, 12*time.Second)
	return body, err
}

func xxtShortText(raw string, n int) string {
	s := strings.Join(strings.Fields(raw), " ")
	if len([]rune(s)) > n {
		return string([]rune(s)[:n]) + "..."
	}
	return s
}

func hasAIConfig(setting consoleConfig.Setting) bool {
	ai := setting.AiSetting
	return strings.TrimSpace(ai.APIKEY) != "" &&
		strings.TrimSpace(ai.Model) != "" &&
		fmt.Sprintf("%v", ai.AiType) != ""
}

// hasExternalQuestionBank 判断外部题库是否配置
func hasExternalQuestionBank(setting consoleConfig.Setting) bool {
	return strings.TrimSpace(setting.ApiQueSetting.Url) != ""
}

func queryExternalQuestion(setting consoleConfig.Setting, q QuestionBankQuestion, pfx func(string)) []string {
	qb := GetRuntimeQuestionBankSetting(setting.ApiQueSetting.Url)
	result, err := QueryQuestionBank(qb, q)
	if err != nil {
		if pfx != nil {
			pfx("外部题库获取答案失败: " + err.Error())
		}
		return nil
	}
	return result.Answers
}

func answerExternalChoice(setting consoleConfig.Setting, q *xuexitongApi.ChoiceQue, courseName string, pfx func(string)) bool {
	answers := queryExternalQuestion(setting, QuestionBankQuestion{
		Type:       q.Type.String(),
		Content:    q.Text,
		OptionsMap: q.Options,
		CourseName: courseName,
	}, pfx)
	answers = normalizeChoiceAnswers(*q, answers, pfx)
	if len(answers) == 0 {
		return false
	}
	q.SetAnswers(answers)
	return true
}

func answerExternalJudge(setting consoleConfig.Setting, q *xuexitongApi.JudgeQue, courseName string, pfx func(string)) bool {
	answers := queryExternalQuestion(setting, QuestionBankQuestion{
		Type:       q.Type.String(),
		Content:    q.Text,
		OptionsMap: q.Options,
		CourseName: courseName,
	}, pfx)
	answers = normalizeJudgeAnswers(answers, pfx)
	if len(answers) == 0 {
		return false
	}
	q.SetAnswers(answers)
	return true
}

func answerExternalSetter(setting consoleConfig.Setting, setter interface{ SetAnswers([]string) }, typeName, content string, options []string, courseName string, pfx func(string)) bool {
	answers := queryExternalQuestion(setting, QuestionBankQuestion{
		Type:       typeName,
		Content:    content,
		Options:    options,
		CourseName: courseName,
	}, pfx)
	if len(answers) == 0 {
		return false
	}
	setter.SetAnswers(answers)
	return true
}

func answerExternalWorkQuestion(setting consoleConfig.Setting, q *xuexitong.XXTWorkQuestion, courseName string, pfx func(string)) bool {
	answers := queryExternalQuestion(setting, QuestionBankQuestion{
		Type:       q.Question.Type,
		Content:    q.Question.Content,
		Options:    q.Question.Options,
		CourseName: courseName,
	}, pfx)
	if len(answers) == 0 {
		return false
	}
	q.Question.Answers = answers
	return true
}

func answerExternalExamQuestion(setting consoleConfig.Setting, q *xuexitong.XXTExamQuestion, courseName string, pfx func(string)) bool {
	answers := queryExternalQuestion(setting, QuestionBankQuestion{
		Type:       q.Question.Type,
		Content:    q.Question.Content,
		Options:    q.Question.Options,
		CourseName: courseName,
	}, pfx)
	if len(answers) == 0 {
		return false
	}
	q.Question.Answers = answers
	return true
}

func answerWorkOrExamExternal(setting consoleConfig.Setting, qa xuexitongApi.Question, courseName string, pfx func(string)) {
	for i := range qa.Choice {
		_ = answerExternalChoice(setting, &qa.Choice[i], courseName, pfx)
	}
	for i := range qa.Judge {
		_ = answerExternalJudge(setting, &qa.Judge[i], courseName, pfx)
	}
	for i := range qa.Fill {
		q := &qa.Fill[i]
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, courseName, pfx)
	}
	for i := range qa.Short {
		q := &qa.Short[i]
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, courseName, pfx)
	}
	for i := range qa.TermExplanation {
		q := &qa.TermExplanation[i]
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, courseName, pfx)
	}
	for i := range qa.Essay {
		q := &qa.Essay[i]
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, courseName, pfx)
	}
	for i := range qa.Matching {
		q := &qa.Matching[i]
		options := append([]string{}, q.Options...)
		options = append(options, q.Selects...)
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, options, courseName, pfx)
	}
	for i := range qa.Other {
		q := &qa.Other[i]
		_ = answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, courseName, pfx)
	}
}

// randomChoiceAnswer 从选择题选项中随机选一个，返回答案列表、日志描述、是否成功
func randomChoiceAnswer(q xuexitongApi.ChoiceQue) ([]string, string, bool) {
	keys := make([]string, 0, len(q.Options))
	for _, k := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		if q.Options[k] != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, "", false
	}
	k := keys[rand.Intn(len(keys))]
	return []string{q.Options[k]}, fmt.Sprintf("%s=%s", k, q.Options[k]), true
}

// normalizeChoiceAnswers 将 AI 返回的答案列表映射到选项文本。
func normalizeChoiceAnswers(q xuexitongApi.ChoiceQue, answers []string, pfx func(string)) []string {
	optParts := []string{}
	for _, k := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		if v := q.Options[k]; v != "" {
			optParts = append(optParts, k+"="+v)
		}
	}
	if len(optParts) > 0 {
		pfx("选择题选项：" + strings.Join(optParts, ", "))
	}
	var out []string
	for _, a := range answers {
		upper := strings.ToUpper(strings.TrimSpace(a))
		if len(upper) == 1 && upper[0] >= 'A' && upper[0] <= 'N' {
			if text := q.Options[upper]; text != "" {
				pfx(fmt.Sprintf("选择题答案匹配：AI=%s -> %s", upper, text))
				out = append(out, text)
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

// normalizeJudgeAnswers 将 AI 返回的判断题答案统一为"正确"或"错误"。
func normalizeJudgeAnswers(answers []string, pfx func(string)) []string {
	trueSet := map[string]bool{"正确": true, "对": true, "是": true, "true": true, "√": true, "✓": true, "a": true, "A": true}
	falseSet := map[string]bool{"错误": true, "错": true, "否": true, "false": true, "×": true, "x": true, "X": true, "b": true, "B": true}
	var out []string
	for _, a := range answers {
		trimmed := strings.TrimSpace(a)
		if trueSet[trimmed] {
			pfx(fmt.Sprintf("判断题答案匹配：AI=%s -> 正确", trimmed))
			out = append(out, "正确")
		} else if falseSet[trimmed] {
			pfx(fmt.Sprintf("判断题答案匹配：AI=%s -> 错误", trimmed))
			out = append(out, "错误")
		} else {
			out = append(out, a)
		}
	}
	return out
}

// safeChapterTestAction 安全版章节测验答题
func safeChapterTestAction(ctx context.Context, setting consoleConfig.Setting,
	user *consoleConfig.User, cache *xuexitongApi.XueXiTUserCache,
	course *xuexitong.XueXiTCourse, knowledge xuexitong.KnowledgeItem,
	qa xuexitongApi.Question, submitThreshold int, randomAnswerOnFail int, emit emitFn) {

	ai := setting.AiSetting
	aiTypeStr := fmt.Sprintf("%v", ai.AiType)
	tp := formatTaskPoint(knowledge)
	stopStart, stopEnd := 1, 2
	tokenBad := false
	answered := 0

	// pfx：标准格式日志，item 固定为测验标题
	pfx := func(msg string) {
		emit("%s", formatXXTLog("学习通", cache.UserID, course.CourseName, tp, qa.Title, msg))
	}

	// 配置检查：未配置则跳过答题
	switch user.CoursesCustom.AutoExam {
	case 0:
		pfx("AutoExam=0，跳过答题")
		return
	case 1:
		if !hasAIConfig(setting) {
			pfx("AI配置不完整，跳过答题")
			return
		}
		pfx("正在AI自动写章节作业...")
	case 2:
		if !hasExternalQuestionBank(setting) {
			pfx("外部题库未配置，跳过答题")
			return
		}
		pfx("正在外挂题库自动答题")
	case 3:
		pfx("正在内置AI自动答题")
	}

	// callAI 调 AI 取答案，经 normalize 后写入题目
	callAI := func(typeName string, aiMsg interface{}, setter func([]string) bool) {
		if ctx.Err() != nil || tokenBad {
			return
		}
		pfx(fmt.Sprintf("调用AI：平台=%s model=%s 题型=%s", aiTypeStr, ai.Model, typeName))
		answers, err := safeAnswerAIGet(cache.UserID, ai.AiUrl, ai.Model, ai.AiType, aiMsg, ai.APIKEY, nil, pfx)
		if err != nil {
			if err.Error() == "invalid_token" {
				tokenBad = true
				pfx("Token 无效，后续题目跳过 AI 答题")
			} else {
				pfx(fmt.Sprintf("AI答题失败（%s）: %s", typeName, err.Error()))
			}
			return
		}
		if len(answers) == 0 {
			return
		}
		wrote := setter(answers)
		if wrote {
			pfx(fmt.Sprintf("已写入答案：题型=%s answers=%v", typeName, answers))
			answered++
		}
		sleepCtx(ctx, time.Duration(rand.Intn(stopEnd-stopStart)+stopStart)*time.Second)
	}
	answerQ := func(fn func()) {
		if ctx.Err() != nil {
			return
		}
		fn()
		sleepCtx(ctx, time.Duration(rand.Intn(stopEnd-stopStart)+stopStart)*time.Second)
	}

	for i := range qa.Choice {
		q := &qa.Choice[i]
		gotAnswer := false
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXChoiceQue: *q})
			callAI("选择题", msg, func(a []string) bool {
				norm := normalizeChoiceAnswers(*q, a, pfx)
				if len(norm) == 0 {
					return false
				}
				q.SetAnswers(norm)
				gotAnswer = true
				return true
			})
		case 2:
			answerQ(func() {
				answerExternalChoice(setting, q, course.CourseName, pfx)
				if len(q.Answers) > 0 {
					answered++
					gotAnswer = true
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXChoiceQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.Answers) > 0 {
					answered++
					gotAnswer = true
				}
			})
		}
		// 选择题随机兜底：AI/题库未获取到答案时随机选一项
		if !gotAnswer && len(q.Options) > 0 {
			if randomAnswerOnFail == 1 {
				if randAnswers, logText, ok := randomChoiceAnswer(*q); ok {
					q.SetAnswers(randAnswers)
					answered++
					pfx(fmt.Sprintf("选择题未获取到有效答案，已随机选择：%s", logText))
				}
			} else {
				pfx("选择题未获取到有效答案，随机兜底关闭，跳过")
			}
		}
	}
	for i := range qa.Judge {
		q := &qa.Judge[i]
		gotAnswer := false
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXJudgeQue: *q})
			callAI("判断题", msg, func(a []string) bool {
				norm := normalizeJudgeAnswers(a, pfx)
				if len(norm) == 0 {
					return false
				}
				q.SetAnswers(norm)
				gotAnswer = true
				return true
			})
		case 2:
			answerQ(func() {
				answerExternalJudge(setting, q, course.CourseName, pfx)
				if len(q.Answers) > 0 {
					answered++
					gotAnswer = true
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXJudgeQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.Answers) > 0 {
					answered++
					gotAnswer = true
				}
			})
		}
		// 判断题随机兜底
		if !gotAnswer {
			if randomAnswerOnFail == 1 {
				choices := []string{"正确", "错误"}
				ans := choices[rand.Intn(2)]
				q.SetAnswers([]string{ans})
				answered++
				pfx(fmt.Sprintf("判断题未获取到有效答案，已随机选择：%s", ans))
			} else {
				pfx("判断题未获取到有效答案，随机兜底关闭，跳过")
			}
		}
	}
	for i := range qa.Fill {
		q := &qa.Fill[i]
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXFillQue: *q})
			callAI("填空题", msg, func(a []string) bool {
				q.SetAnswers(a)
				return len(q.OpFromAnswer) > 0
			})
		case 2:
			answerQ(func() {
				answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, course.CourseName, pfx)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXFillQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		}
	}
	for i := range qa.Short {
		q := &qa.Short[i]
		if q.OpFromAnswer == nil {
			q.OpFromAnswer = make(map[string][]string)
		}
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXShortQue: *q})
			callAI("简答题", msg, func(a []string) bool {
				q.SetAnswers(a)
				return len(q.OpFromAnswer) > 0
			})
		case 2:
			answerQ(func() {
				answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, course.CourseName, pfx)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXShortQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		}
	}
	for i := range qa.TermExplanation {
		q := &qa.TermExplanation[i]
		if q.OpFromAnswer == nil {
			q.OpFromAnswer = make(map[string][]string)
		}
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXTermExplanationQue: *q})
			callAI("名词解释", msg, func(a []string) bool {
				q.SetAnswers(a)
				return len(q.OpFromAnswer) > 0
			})
		case 2:
			answerQ(func() {
				answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, course.CourseName, pfx)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXTermExplanationQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		}
	}
	for i := range qa.Essay {
		q := &qa.Essay[i]
		if q.OpFromAnswer == nil {
			q.OpFromAnswer = make(map[string][]string)
		}
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXEssayQue: *q})
			callAI("论述题", msg, func(a []string) bool {
				q.SetAnswers(a)
				return len(q.OpFromAnswer) > 0
			})
		case 2:
			answerQ(func() {
				answerExternalSetter(setting, q, q.Type.String(), q.Text, nil, course.CourseName, pfx)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXEssayQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.OpFromAnswer) > 0 {
					answered++
				}
			})
		}
	}
	for i := range qa.Matching {
		q := &qa.Matching[i]
		switch user.CoursesCustom.AutoExam {
		case 1:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXMatchingQue: *q})
			callAI("连线题", msg, func(a []string) bool {
				q.SetAnswers(a)
				return len(q.Answers) > 0
			})
		case 2:
			answerQ(func() {
				options := append([]string{}, q.Options...)
				options = append(options, q.Selects...)
				answerExternalSetter(setting, q, q.Type.String(), q.Text, options, course.CourseName, pfx)
				if len(q.Answers) > 0 {
					answered++
				}
			})
		case 3:
			msg := xuexitong.AIProblemMessage(qa.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXMatchingQue: *q})
			answerQ(func() {
				q.AnswerXXTAIGet(cache, qa.ClassId, qa.CourseId, qa.Cpi, msg)
				if len(q.Answers) > 0 {
					answered++
				}
			})
		}
	}

	// 提交前修正 + 快照
	totalQ := len(qa.Choice) + len(qa.Judge) + len(qa.Fill) + len(qa.Short) + len(qa.TermExplanation) + len(qa.Essay) + len(qa.Matching)
	xuexitong.AnswerFixedPattern(qa.Choice, qa.Judge)

	// 答案快照
	choiceSnap := []string{}
	for _, c := range qa.Choice {
		choiceSnap = append(choiceSnap, fmt.Sprintf("%s:%v", c.Qid, c.Answers))
	}
	judgeSnap := []string{}
	for _, c := range qa.Judge {
		judgeSnap = append(judgeSnap, fmt.Sprintf("%s:%v", c.Qid, c.Answers))
	}
	fillSnap := []string{}
	for _, c := range qa.Fill {
		fillSnap = append(fillSnap, fmt.Sprintf("%s:%v", c.Qid, c.OpFromAnswer))
	}
	pfx(fmt.Sprintf("提交前答案快照：choice=%s, judge=%s, fill=%s",
		strings.Join(choiceSnap, "|"), strings.Join(judgeSnap, "|"), strings.Join(fillSnap, "|")))

	var resultStr string
	shouldSubmit := false

	if tokenBad || answered == 0 {
		pfx(fmt.Sprintf("AI 不可用或无有效答题（answered=%d tokenBad=%v），已禁止提交，仅保存", answered, tokenBad))
		resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(cache, qa, false)
	} else {
		switch user.CoursesCustom.ExamAutoSubmit {
		case 0:
			shouldSubmit = false
		case 1:
			shouldSubmit = true
		case 2:
			thresh := submitThreshold
			if thresh <= 0 {
				thresh = 100
			}
			if totalQ > 0 {
				pct := answered * 100 / totalQ
				shouldSubmit = pct >= thresh
				pfx(fmt.Sprintf("答题完成率：%d/%d = %d%%，提交阈值：%d%%，%s", answered, totalQ, pct, thresh,
					map[bool]string{true: "将提交", false: "仅保存"}[shouldSubmit]))
			}
		}
		resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(cache, qa, shouldSubmit)
	}

	statusOK := false
	if v := gojsonq.New().JSONString(resultStr).Find("status"); v != nil {
		if b, ok := v.(bool); ok {
			statusOK = b
		}
	}
	if statusOK {
		pfx("章节作业答题完毕，服务器返回：" + resultStr)
	} else {
		pfx("章节作业答题保存/提交失败，服务器返回：" + resultStr)
	}
}

// safeAnswerAIGet 调 OpenAI-compatible API 获取回复，解析为 []string 答案列表。
func safeAnswerAIGet(userID, aiUrl, model string, aiType interface{}, msg interface{}, apiKey string, _ interface{}, pfx func(string)) ([]string, error) {
	apiKey = strings.TrimSpace(apiKey)

	endpoint := strings.TrimRight(strings.TrimSpace(fmt.Sprintf("%v", aiUrl)), "/")
	aiTypeStr := fmt.Sprintf("%v", aiType)
	if aiTypeStr == "SILICON" || endpoint == "" || strings.Contains(endpoint, "cloud.siliconflow") {
		endpoint = "https://api.siliconflow.cn/v1/chat/completions"
	} else if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	keyLen := len(apiKey)
	keyPrefix := apiKey
	if keyLen > 8 {
		keyPrefix = apiKey[:8] + "..."
	}
	pfx(fmt.Sprintf("AI Key 长度：%d 前缀：%s 接口：%s model：%s", keyLen, keyPrefix, endpoint, model))
	if strings.Contains(apiKey, "/") || strings.HasPrefix(apiKey, "Qwen") || strings.HasPrefix(apiKey, "deepseek") {
		pfx(fmt.Sprintf("API Key 字段疑似填成了模型名（前缀：%s），请检查全局设置 → API Key 字段", keyPrefix))
		return nil, fmt.Errorf("invalid_token")
	}

	type oaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var messages []oaiMsg
	if rawMsgs, err2 := json.Marshal(msg); err2 == nil {
		var arr []map[string]interface{}
		if json.Unmarshal(rawMsgs, &arr) == nil {
			for _, m := range arr {
				role := fmt.Sprintf("%v", m["role"])
				content := fmt.Sprintf("%v", m["content"])
				if role == "" {
					role = "user"
				}
				messages = append(messages, oaiMsg{Role: role, Content: content})
			}
		}
	}
	if len(messages) == 0 {
		messages = []oaiMsg{{Role: "user", Content: fmt.Sprintf("%v", msg)}}
	}

	bodyMap := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0,
		"stream":      false,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	rawBytes, _ := io.ReadAll(resp.Body)
	raw := string(rawBytes)

	if resp.StatusCode == 401 || strings.Contains(raw, "Invalid token") || strings.Contains(raw, "Unauthorized") {
		pfx(fmt.Sprintf("AI Token 无效（%d）：%s，请检查 API Key", resp.StatusCode, raw[:min(len(raw), 200)]))
		return nil, fmt.Errorf("invalid_token")
	}
	if code := gojsonq.New().JSONString(raw).Find("code"); code != nil {
		codeVal := fmt.Sprintf("%v", code)
		if codeVal != "0" && codeVal != "<nil>" {
			msg2 := gojsonq.New().JSONString(raw).Find("message")
			pfx(fmt.Sprintf("AI 请求失败 code=%s message=%v，返回：%s", codeVal, msg2, raw[:min(len(raw), 300)]))
			return nil, fmt.Errorf("api_error_%s", codeVal)
		}
	}

	content := raw
	if v := gojsonq.New().JSONString(raw).Find("choices.[0].message.content"); v != nil {
		content = fmt.Sprintf("%v", v)
	}

	preview := content
	if len([]rune(preview)) > 500 {
		preview = string([]rune(preview)[:500]) + "..."
	}
	pfx("AI 原始回复：" + preview)

	answers, parseErr := parseAIAnswers(content)
	if parseErr != nil || len(answers) == 0 {
		pfx("AI 回复解析失败，跳过此题（不随机提交）。原文：" + preview)
		return nil, fmt.Errorf("parse_failed")
	}
	pfx(fmt.Sprintf("AI 解析答案：%v", answers))
	return answers, nil
}

// parseAIAnswers 从 AI 回复中提取答案列表，支持多种格式：
//
//	JSON array:  ["A"] / ["房缩期"]（整段或嵌入文本中）
//	JSON object: {"answer":"A"} / {"answer":["A","C"]} / {"answers":["A","C"]}
//	Markdown 包裹的 JSON
//	纯文本：房缩期
func parseAIAnswers(content string) ([]string, error) {
	s := strings.TrimSpace(content)
	// 去除 markdown 代码块
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.TrimSpace(s)

	// 尝试整段 JSON array
	if arr, ok := tryParseArray(s); ok {
		return arr, nil
	}

	// 从文本中截取第一个 [ 到最后一个 ] 尝试 array
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			if arr, ok := tryParseArray(s[i : j+1]); ok {
				return arr, nil
			}
		}
	}

	// 尝试 JSON object
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			obj := s[i : j+1]
			for _, key := range []string{"answer", "answers", "result"} {
				if v := gojsonq.New().JSONString(obj).Find(key); v != nil {
					switch val := v.(type) {
					case string:
						if val != "" {
							return []string{val}, nil
						}
					case []interface{}:
						var out []string
						for _, item := range val {
							out = append(out, fmt.Sprintf("%v", item))
						}
						if len(out) > 0 {
							return out, nil
						}
					}
				}
			}
		}
	}

	// 纯文本兜底
	plain := strings.TrimSpace(strings.Trim(s, `"'`))
	if plain != "" && !strings.ContainsAny(plain, "{[") {
		return []string{plain}, nil
	}

	return nil, fmt.Errorf("cannot parse: %s", s[:min(len(s), 100)])
}

func tryParseArray(s string) ([]string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil, false
	}
	var arr []string
	if json.Unmarshal([]byte(s), &arr) == nil && len(arr) > 0 {
		return arr, true
	}
	var arri []interface{}
	if json.Unmarshal([]byte(s), &arri) == nil && len(arri) > 0 {
		var out []string
		for _, v := range arri {
			out = append(out, fmt.Sprintf("%v", v))
		}
		return out, true
	}
	return nil, false
}

// TestAI 发一个最小 AI 请求，验证配置是否可用，供 Settings 页"测试 AI"按钮使用
func TestAI(aiUrl, model, aiType, apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	endpoint := strings.TrimRight(strings.TrimSpace(aiUrl), "/")
	if aiType == "SILICON" || endpoint == "" || strings.Contains(endpoint, "cloud.siliconflow") {
		endpoint = "https://api.siliconflow.cn/v1/chat/completions"
	} else if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	type oaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	bodyMap := map[string]interface{}{
		"model":       model,
		"messages":    []oaiMsg{{Role: "user", Content: `只返回纯 JSON，不要 Markdown 格式。返回：{"answer":"A"}`}},
		"temperature": 0,
		"stream":      false,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络错误: %w", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 16*1024)
	n, _ := resp.Body.Read(buf)
	raw := string(buf[:n])

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("Token 无效（401），请检查 API Key")
	}
	if code := gojsonq.New().JSONString(raw).Find("code"); code != nil {
		codeVal := fmt.Sprintf("%v", code)
		if codeVal != "0" && codeVal != "<nil>" {
			msg2 := gojsonq.New().JSONString(raw).Find("message")
			return "", fmt.Errorf("请求失败 code=%s message=%v endpoint=%s model=%s", codeVal, msg2, endpoint, model)
		}
	}
	if v := gojsonq.New().JSONString(raw).Find("choices.[0].message.content"); v != nil {
		return fmt.Sprintf("AI 测试通过，返回：%v", v), nil
	}
	return "", fmt.Errorf("响应无 choices，原文：%s", raw[:min(len(raw), 300)])
}
