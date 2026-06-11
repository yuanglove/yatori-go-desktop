package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yatori-dev/yatori-go-core/aggregation/haiqikeji"
	hqkjApi "github.com/yatori-dev/yatori-go-core/api/haiqikeji"
)

// hqkjRetry runs fn up to 3 times with 1s delay, converting panics to errors.
func hqkjRetry[T any](fn func() (T, error)) (out T, err error) {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		out, err = hqkjSafeCall(fn)
		if err == nil {
			return out, nil
		}
		if i+1 < attempts {
			time.Sleep(1 * time.Second)
		}
	}
	return out, err
}

// hqkjSafeCall executes fn and converts any panic into an error.
func hqkjSafeCall[T any](fn func() (T, error)) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

func getHqkjCourses(po AccountPO) ([]CourseVO, error) {
	cache := &hqkjApi.HqkjUserCache{
		PreUrl:   po.URL,
		Account:  po.Account,
		Password: DecodePassword(po.PasswordEnc),
	}

	// Login with retry+recover
	if _, err := hqkjRetry(func() (struct{}, error) {
		return struct{}{}, haiqikeji.HqkjLoginAction(cache)
	}); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}

	// Course list with retry+recover
	list, err := hqkjRetry(func() ([]haiqikeji.HqkjCourse, error) {
		return haiqikeji.HqkjCourseListAction(cache)
	})
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}

	now := time.Now()
	vos := make([]CourseVO, 0, len(list))
	partialFail := 0

	for i, c := range list {
		inTime := !c.StartDate.After(now) && !c.EndDate.Before(now)

		// Node list with retry+recover — failure is non-fatal
		nodes, nerr := hqkjRetry(func() ([]haiqikeji.HqkjNode, error) {
			return haiqikeji.HqkjNodeListAction(cache, c)
		})

		var finCount, totalCount int
		hasProgressData := false

		if nerr != nil {
			partialFail++
		} else {
			for _, n := range nodes {
				if n.TabVideo <= 0 {
					continue
				}
				totalCount++
				// Progress per node — failure counts node as unfinished, not fatal
				progress, perr := hqkjRetry(func() (int, error) {
					return haiqikeji.HqkjGetNodeProgressAction(cache, n)
				})
				if perr == nil {
					hasProgressData = true
					if progress >= 100 {
						finCount++
					}
				} else {
					partialFail++
				}
			}
			if totalCount > 0 {
				hasProgressData = true
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
			HasProgress:    hasProgressData,
			IsStart:        inTime,
			RawStatusText:  status,
		})
	}

	// Return partial results with a warning appended to last course's status
	if partialFail > 0 && len(vos) > 0 {
		last := &vos[len(vos)-1]
		last.RawStatusText += fmt.Sprintf("（%d 个节点进度拉取失败，结果可能不完整）", partialFail)
	}

	return vos, nil
}
