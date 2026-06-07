package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yatori-dev/yatori-go-core/aggregation/haiqikeji"
	hqkjApi "github.com/yatori-dev/yatori-go-core/api/haiqikeji"
)

func getHqkjCourses(po AccountPO) ([]CourseVO, error) {
	cache := &hqkjApi.HqkjUserCache{
		PreUrl:   po.URL,
		Account:  po.Account,
		Password: DecodePassword(po.PasswordEnc),
	}
	if err := haiqikeji.HqkjLoginAction(cache); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}
	list, err := haiqikeji.HqkjCourseListAction(cache)
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}
	now := time.Now()
	vos := make([]CourseVO, 0, len(list))
	for i, c := range list {
		inTime := !c.StartDate.After(now) && !c.EndDate.Before(now)
		nodes, nerr := haiqikeji.HqkjNodeListAction(cache, c)
		var finCount, totalCount int
		if nerr == nil {
			for _, n := range nodes {
				if n.TabVideo <= 0 {
					continue
				}
				totalCount++
				progress, perr := haiqikeji.HqkjGetNodeProgressAction(cache, n)
				if perr == nil && progress >= 100 {
					finCount++
				}
			}
		}
		var jobRate float64
		if totalCount > 0 {
			jobRate = float64(finCount) / float64(totalCount) * 100
		}
		status := fmt.Sprintf("%s ~ %s", c.StartDate.Format("2006-01-02"), c.EndDate.Format("2006-01-02"))
		vos = append(vos, CourseVO{
			Platform:       "HQKJ",
			Key:            strconv.Itoa(i),
			CourseName:     c.Name,
			JobFinishCount: finCount,
			JobCount:       totalCount,
			JobRate:        jobRate,
			IsStart:        inTime,
			RawStatusText:  status,
		})
	}
	return vos, nil
}
