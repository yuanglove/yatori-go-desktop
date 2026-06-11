package service

import "fmt"

type CourseVO struct {
	Platform       string  `json:"platform"`
	Key            string  `json:"key"`
	CourseID       string  `json:"courseId"`
	CourseName     string  `json:"courseName"`
	CourseTeacher  string  `json:"courseTeacher"`
	JobFinishCount int     `json:"jobFinishCount"`
	JobCount       int     `json:"jobCount"`
	JobRate        float64 `json:"jobRate"`
	HasProgress    bool    `json:"hasProgress"`
	State          int     `json:"state"`
	IsStart        bool    `json:"isStart"`
	RawStatusText  string  `json:"rawStatusText,omitempty"`
}

func GetCourses(uid string) (vos []CourseVO, retErr error) {
	// Ensure a panic inside any platform handler never propagates to Wails.
	defer func() {
		if r := recover(); r != nil {
			vos = nil
			retErr = fmt.Errorf("课程进度拉取时发生内部错误: %v", r)
		}
	}()

	if err := InitDB(); err != nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}
	po, err := GetAccountPO(uid)
	if err != nil {
		return nil, fmt.Errorf("账号不存在: %w", err)
	}
	switch po.AccountType {
	case "XUEXITONG":
		return getXueXiTongCourses(po)
	case "YINGHUA":
		return getYingHuaCourses(po)
	case "HQKJ":
		return getHqkjCourses(po)
	case "WELEARN":
		return getWeLearnCourses(po)
	default:
		return nil, fmt.Errorf("暂不支持 %s 平台课程进度拉取", po.AccountType)
	}
}
