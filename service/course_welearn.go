package service

import (
	"fmt"
	"strings"
	"time"

	action "github.com/yatori-dev/yatori-go-core/aggregation/welearn"
	"github.com/yatori-dev/yatori-go-core/api/welearn"
)

func getWeLearnCourses(po AccountPO) ([]CourseVO, error) {
	cache := &welearn.WeLearnUserCache{
		Account:  po.Account,
		Password: DecodePassword(po.PasswordEnc),
	}
	if _, err := retryWeLearn(3, 1200*time.Millisecond, func() (struct{}, error) {
		return struct{}{}, action.WeLearnLoginAction(cache)
	}); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}
	list, err := retryWeLearn(3, 1500*time.Millisecond, func() ([]action.WeLearnCourse, error) {
		return action.WeLearnPullCourseListAction(cache)
	})
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}
	vos := make([]CourseVO, 0, len(list))
	for _, c := range list {
		chapters, cerr := retryWeLearn(3, 1200*time.Millisecond, func() ([]action.WeLearnChapter, error) {
			return action.WeLearnPullCourseChapterAction(cache, c)
		})
		var finCount, totalCount int
		rawStatus := ""
		if cerr == nil {
			for _, ch := range chapters {
				points, perr := retryWeLearn(3, 1200*time.Millisecond, func() ([]action.WeLearnPoint, error) {
					return action.WeLearnPullChapterPointAction(cache, c, ch)
				})
				if perr != nil {
					chapterName := strings.TrimSpace(ch.Name)
					if chapterName == "" {
						chapterName = ch.Unitname
					}
					if rawStatus == "" {
						rawStatus = "部分章节任务点拉取失败：" + chapterName
					}
					continue
				}
				for _, p := range points {
					if !p.IsVisible {
						continue
					}
					totalCount++
					if isWeLearnCompleted(p.IsComplete) {
						finCount++
					}
				}
			}
		} else {
			rawStatus = "章节拉取失败：" + cerr.Error()
		}
		var jobRate float64
		if totalCount > 0 {
			jobRate = float64(finCount) / float64(totalCount) * 100
		} else if c.Per > 0 {
			jobRate = float64(c.Per)
			if rawStatus == "" {
				rawStatus = "按平台进度展示"
			}
		}
		vos = append(vos, CourseVO{
			Platform:       "WELEARN",
			Key:            fmt.Sprintf("%s_%s", c.Cid, c.ClassId),
			CourseID:       c.Cid,
			CourseName:     c.Name,
			JobFinishCount: finCount,
			JobCount:       totalCount,
			JobRate:        jobRate,
			HasProgress:    totalCount > 0 || c.Per > 0,
			IsStart:        true,
			RawStatusText:  rawStatus,
		})
	}
	return vos, nil
}
