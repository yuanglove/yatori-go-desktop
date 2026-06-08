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
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"
	"github.com/thedevsaddam/gojsonq"
	xuexitong "github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	aiq "github.com/yatori-dev/yatori-go-core/que-core/aiq"
	external "github.com/yatori-dev/yatori-go-core/que-core/external"
	"io"
	"net/http"
	consoleConfig "yatori-go-console/config"
	xxtLogic "yatori-go-console/logic/xuexitong"
)

// emit 是日志回调，用 printf 格式
type emitFn func(format string, args ...interface{})

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

	if emit == nil {
		emit = func(f string, a ...interface{}) {}
	}

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
			sysLog("课程 [%s] 在排除列表，跳过", course.CourseName)
			excluded++
			continue
		}
		// 包含课程（按名称精确匹配；为空则全部学）
		if len(user.CoursesCustom.IncludeCourses) > 0 &&
			!consoleConfig.CmpCourse(course.CourseName, user.CoursesCustom.IncludeCourses) {
			sysLog("课程 [%s] 不在包含列表，跳过", course.CourseName)
			skipped++
			continue
		}
		if !course.IsStart {
			sysLog("课程 [%s] 还未开课，跳过", course.CourseName)
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
		emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", "课程已结束，跳过"))
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
			emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", "课程章节为空，跳过"))
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

	emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", fmt.Sprintf("获取课程章节成功（共 %d 个）", len(action.Knowledge))))

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

	emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, "", "", "正在学习该课程"))

	if user.CoursesCustom.VideoModel == 3 && len(model3Caches) > 0 {
		// 多任务点并发，用 nodeSem 控制并发
		var nodeLock sync.WaitGroup
		for index := range nodes {
			if ctx.Err() != nil {
				break
			}
			if isFinished(index) {
				tp := formatTaskPoint(pointAction.Knowledge[index])
				emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, tp, "", "任务点已完成，跳过"))
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
				tp := formatTaskPoint(pointAction.Knowledge[index])
				emit("%s", formatXXTLog("学习通", cache.Name, course.CourseName, tp, "", "任务点已完成，跳过"))
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
		xlog(tp, "", "无任何任务节点，跳过")
		return
	}

	// 视频/音频
	if videoDTOs != nil && user.CoursesCustom.VideoModel != 0 {
		// acquire nodeSem
		nodeSem <- struct{}{}
		nodeSemMu.Lock()
		(*nodeSemUsed)++
		used := *nodeSemUsed
		total := cap(nodeSem)
		nodeSemMu.Unlock()
		xlog(tp, "", fmt.Sprintf("开始视频任务，占用并发槽 %d/%d", used, total))
		defer func() {
			<-nodeSem
			nodeSemMu.Lock()
			if *nodeSemUsed > 0 {
				*nodeSemUsed--
			}
			used := *nodeSemUsed
			total := cap(nodeSem)
			nodeSemMu.Unlock()
			xlog(tp, "", fmt.Sprintf("视频任务结束，释放并发槽，当前 %d/%d", used, total))
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
		xlog(tp, "", "自动答题关闭，跳过章节测验")
	} else if cxChapterSw == 0 {
		xlog(tp, "", "章节测验开关关闭，跳过")
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
				xlog(tp, qa.Title, "章节测验已完成，跳过")
				continue
			}
			hasQ := len(qa.Short) + len(qa.Choice) + len(qa.Judge) + len(qa.Fill) + len(qa.TermExplanation) + len(qa.Essay)
			if hasQ == 0 {
				xlog(tp, qa.Title, "章节测验无题目，跳过")
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

	xlog("", fmt.Sprintf("开始检查课程作业/考试（CxWorkSw=%d CxExamSw=%d AutoExam=%d ExamAutoSubmit=%d）",
		cxWork, cxExam, user.CoursesCustom.AutoExam, user.CoursesCustom.ExamAutoSubmit))

	if user.CoursesCustom.AutoExam == 0 {
		xlog("", "AutoExam=0，跳过答题")
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
		if err := external.CheckApiQueRequest(setting.ApiQueSetting.Url, 5, nil); err != nil {
			xlog("", "外挂题库不可用: "+err.Error())
			return
		}
	}

	// 作业
	if cxWork == 1 {
		workList, err := xuexitong.PullWorkListAction(cache, *course)
		if err != nil {
			xlog("", "拉取作业列表失败，跳过: "+err.Error())
		} else {
			xlog("", fmt.Sprintf("拉取作业数量：%d", len(workList)))
			found := 0
			for _, work := range workList {
				if ctx.Err() != nil {
					return
				}
				if !(work.Status == "待做" || work.Status == "未交" || work.Status == "待重做") {
					continue
				}
				found++
				xlog(work.Name, "检测到待做作业")
				if err2 := xuexitong.EnterWorkAction(cache, &work); err2 != nil {
					xlog(work.Name, "进入作业失败: "+err2.Error())
					continue
				}
				safeWorkAction(ctx, setting, user, cache, course, work, emit)
			}
			if found == 0 {
				xlog("", "未检测到待做作业")
			}
		}
	}

	// 考试
	if cxExam == 1 {
		examList, err := xuexitong.PullExamListAction(cache, *course)
		if err != nil {
			xlog("", "拉取考试列表失败，跳过: "+err.Error())
		} else {
			xlog("", fmt.Sprintf("拉取考试数量：%d", len(examList)))
			found := 0
			for _, exam := range examList {
				if ctx.Err() != nil {
					return
				}
				if exam.Status != "待做" && exam.Status != "待重考" {
					continue
				}
				found++
				xlog(exam.Name, "检测到待做考试")
				if err2 := xuexitong.EnterExamAction(cache, &exam); err2 != nil {
					xlog(exam.Name, "进入考试失败: "+err2.Error())
					continue
				}
				safeExamAction(ctx, setting, user, cache, course, exam, emit)
			}
			if found == 0 {
				xlog("", "未检测到待做考试")
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
			question.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
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
	course *xuexitong.XueXiTCourse, exam xuexitong.XXTExam, emit emitFn) {
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
			question.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
				q.AnswerExternalGet(setting.ApiQueSetting.Url)
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
