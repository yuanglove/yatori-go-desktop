package service

import (
	"fmt"

	"github.com/yatori-dev/yatori-go-core/aggregation/yinghua"
	yinghuaApi "github.com/yatori-dev/yatori-go-core/api/yinghua"
)

func getYingHuaCourses(po AccountPO) ([]CourseVO, error) {
	if po.URL == "" {
		return nil, fmt.Errorf("英华 URL 为空，请在账号管理中填写学校入口地址")
	}
	if po.PasswordEnc == "" {
		return nil, fmt.Errorf("英华密码未保存")
	}
	EnsureCoreRuntime()
	cache := &yinghuaApi.YingHuaUserCache{
		PreUrl:   po.URL,
		Account:  po.Account,
		Password: DecodePassword(po.PasswordEnc),
	}
	if err := yinghua.YingHuaLoginAction(cache); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}
	list, err := yinghua.CourseListAction(cache)
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}
	vos := make([]CourseVO, 0, len(list))
	for _, c := range list {
		nodes, nerr := yinghua.VideosListAction(cache, c)
		var finCount, totalCount int
		if nerr == nil {
			for _, n := range nodes {
				if !n.TabVideo {
					continue
				}
				totalCount++
				if int(n.Progress) >= 100 {
					finCount++
				}
			}
		}
		var jobRate float64
		if totalCount > 0 {
			jobRate = float64(finCount) / float64(totalCount) * 100
		}
		id := fmt.Sprintf("%v", c.Id)
		vos = append(vos, CourseVO{
			Platform:       "YINGHUA",
			Key:            id,
			CourseID:       id,
			CourseName:     c.Name,
			JobFinishCount: finCount,
			JobCount:       totalCount,
			JobRate:        jobRate,
			HasProgress:    totalCount > 0,
			IsStart:        !c.StartDate.IsZero(),
		})
	}
	return vos, nil
}
