package service

import (
	"fmt"

	action "github.com/yatori-dev/yatori-go-core/aggregation/welearn"
	"github.com/yatori-dev/yatori-go-core/api/welearn"
)

func getWeLearnCourses(po AccountPO) ([]CourseVO, error) {
	cache := &welearn.WeLearnUserCache{
		Account:  po.Account,
		Password: DecodePassword(po.PasswordEnc),
	}
	if err := action.WeLearnLoginAction(cache); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}
	list, err := action.WeLearnPullCourseListAction(cache)
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}
	vos := make([]CourseVO, 0, len(list))
	for _, c := range list {
		// 拉取章节任务点统计进度
		chapters, cerr := action.WeLearnPullCourseChapterAction(cache, c)
		var finCount, totalCount int
		if cerr == nil {
			for _, ch := range chapters {
				points, perr := action.WeLearnPullChapterPointAction(cache, c, ch)
				if perr != nil {
					continue
				}
				for _, p := range points {
					if !p.IsVisible {
						continue
					}
					totalCount++
					if p.IsComplete == "completed" || p.IsComplete == "已完成" {
						finCount++
					}
				}
			}
		}
		var jobRate float64
		if totalCount > 0 {
			jobRate = float64(finCount) / float64(totalCount) * 100
		}
		vos = append(vos, CourseVO{
			Platform:       "WELEARN",
			Key:            fmt.Sprintf("%s_%s", c.Cid, c.ClassId),
			CourseID:       c.Cid,
			CourseName:     c.Name,
			JobFinishCount: finCount,
			JobCount:       totalCount,
			JobRate:        jobRate,
			HasProgress:    totalCount > 0,
			IsStart:        true,
		})
	}
	return vos, nil
}
