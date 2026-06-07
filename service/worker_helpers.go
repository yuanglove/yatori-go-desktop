package service

import (
	"encoding/json"

	consoleActivity "yatori-go-console/web/activity"
	consoleConfig "yatori-go-console/config"
	consolePojo "yatori-go-console/entity/pojo"
)

// BuildUserFromPO 供 worker 子进程使用（已导出）
func BuildUserFromPO(po AccountPO) consoleConfig.User { return buildUser(po) }

// BuildActivity 供 worker 子进程使用（已导出）
func BuildActivity(po AccountPO) consoleActivity.Activity { return buildActivity(po) }

// buildUser 把 AccountPO 转为 consoleConfig.User（供 SafeRun 使用）
func buildUser(po AccountPO) consoleConfig.User {
	pwd := decodePassword(po.PasswordEnc)
	var cc CoursesCustom
	_ = json.Unmarshal([]byte(po.CoursesCustom), &cc)
	one, three := 1, 3
	cxNode := cc.CxNode
	if cxNode == nil { cxNode = &three }
	cxChapter := cc.CxChapterTestSw
	if cxChapter == nil { cxChapter = &one }
	cxWork := cc.CxWorkSw
	if cxWork == nil { cxWork = &one }
	cxExam := cc.CxExamSw
	if cxExam == nil { cxExam = &one }
	return consoleConfig.User{
		AccountType: po.AccountType, URL: po.URL, RemarkName: po.RemarkName,
		Account: po.Account, Password: pwd, IsProxy: po.IsProxy,
		CoursesCustom: consoleConfig.CoursesCustom{
			VideoModel: cc.VideoModel, AutoExam: cc.AutoExam, ExamAutoSubmit: cc.ExamAutoSubmit,
			ShuffleSw: cc.ShuffleSw, StudyTime: cc.StudyTime,
			IncludeCourses: cc.IncludeCourses, ExcludeCourses: cc.ExcludeCourses,
			CxNode: cxNode, CxChapterTestSw: cxChapter, CxWorkSw: cxWork, CxExamSw: cxExam,
		},
	}
}

// buildActivity 只为 XUEXITONG 构建 Activity（web/activity 路径，无 os.Exit）
func buildActivity(po AccountPO) consoleActivity.Activity {
	pwd := decodePassword(po.PasswordEnc)
	var cc CoursesCustom
	_ = json.Unmarshal([]byte(po.CoursesCustom), &cc)
	one, three := 1, 3
	cxNode := cc.CxNode
	if cxNode == nil { cxNode = &three }
	cxChapter := cc.CxChapterTestSw
	if cxChapter == nil { cxChapter = &one }
	cxWork := cc.CxWorkSw
	if cxWork == nil { cxWork = &one }
	cxExam := cc.CxExamSw
	if cxExam == nil { cxExam = &one }

	user := consoleConfig.User{
		AccountType: po.AccountType, URL: po.URL, RemarkName: po.RemarkName,
		Account: po.Account, Password: pwd, IsProxy: po.IsProxy,
		CoursesCustom: consoleConfig.CoursesCustom{
			VideoModel: cc.VideoModel, AutoExam: cc.AutoExam, ExamAutoSubmit: cc.ExamAutoSubmit,
			ShuffleSw: cc.ShuffleSw, StudyTime: cc.StudyTime,
			IncludeCourses: cc.IncludeCourses, ExcludeCourses: cc.ExcludeCourses,
			CxNode: cxNode, CxChapterTestSw: cxChapter, CxWorkSw: cxWork, CxExamSw: cxExam,
		},
	}
	userJSON, _ := json.Marshal(user)
	consolePO := consolePojo.UserPO{
		Uid: po.UID, AccountType: po.AccountType, Url: po.URL,
		Account: po.Account, Password: pwd, UserConfigJson: string(userJSON),
	}
	return consoleActivity.BuildUserActivity(consolePO)
}
