package service

import (
	"fmt"
	xuexitong "github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
)

func getXueXiTongCourses(po AccountPO) ([]CourseVO, error) {
	act := buildActivity(po)
	if act == nil {
		return nil, fmt.Errorf("构建 Activity 失败")
	}
	if err := act.Login(); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}
	cache, ok := act.GetUserCache().(*xuexitongApi.XueXiTUserCache)
	if !ok || cache == nil {
		return nil, fmt.Errorf("获取 cache 失败")
	}
	list, err := xuexitong.XueXiTPullCourseAction(cache)
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}
	vos := make([]CourseVO, 0, len(list))
	for _, c := range list {
		vos = append(vos, CourseVO{
			Platform:       "XUEXITONG",
			Key:            c.Key,
			CourseID:       c.CourseID,
			CourseName:     c.CourseName,
			CourseTeacher:  c.CourseTeacher,
			JobFinishCount: c.JobFinishCount,
			JobCount:       c.JobCount,
			JobRate:        c.JobRate,
			HasProgress:    true,
			State:          c.State,
			IsStart:        c.IsStart,
		})
	}
	return vos, nil
}
