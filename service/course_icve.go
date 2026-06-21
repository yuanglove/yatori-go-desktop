package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	icveAction "github.com/yatori-dev/yatori-go-core/aggregation/icve"
	icveApi "github.com/yatori-dev/yatori-go-core/api/icve"
	coreUtils "github.com/yatori-dev/yatori-go-core/utils"
)

func icveRetry[T any](fn func() (T, error)) (out T, err error) {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		out, err = icveSafeCall(fn)
		if err == nil {
			return out, nil
		}
		if i+1 < attempts {
			time.Sleep(1 * time.Second)
		}
	}
	return out, err
}

func icveSafeCall[T any](fn func() (T, error)) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

func icveCallWithTimeout[T any](timeout time.Duration, fn func() (T, error)) (out T, err error) {
	type result struct {
		out T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, e := icveSafeCall(fn)
		ch <- result{out: v, err: e}
	}()
	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(timeout):
		return out, fmt.Errorf("调用超时（%s）", timeout)
	}
}

func getICVECourses(po AccountPO) ([]CourseVO, error) {
	cookie := strings.TrimSpace(DecodePassword(po.PasswordEnc))
	if cookie == "" || len(cookie) <= 30 {
		return nil, fmt.Errorf("智慧职教只支持 Cookie 登录，请先在账号管理中填写或自动获取完整 Cookie")
	}

	cache := &icveApi.IcveUserCache{
		Account:  po.Account,
		Password: cookie,
	}
	if _, err := icveCallWithTimeout(15*time.Second, func() (struct{}, error) {
		return struct{}{}, icveAction.IcveCookieLogin(cache)
	}); err != nil {
		return nil, fmt.Errorf("Cookie 登录失败: %w", err)
	}

	list, err := icveCallWithTimeout(15*time.Second, func() ([]icveAction.IcveCourse, error) {
		return icveAction.PullZYKCourseAction(cache)
	})
	if err != nil {
		return nil, fmt.Errorf("拉取课程失败: %w", err)
	}

	vos := make([]CourseVO, 0, len(list))
	partialFail := 0

	type courseResult struct {
		index int
		vo    CourseVO
		err   error
	}
	resultCh := make(chan courseResult, len(list))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for i, c := range list {
		wg.Add(1)
		go func(i int, c icveAction.IcveCourse) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cacheCopy := *cache
			nodes, nerr := icveCallWithTimeout(55*time.Second, func() ([]icveAction.IcveCourseNode, error) {
				return pullICVENodesFast(&cacheCopy, c)
			})
			vo := buildICVECourseVO(c, nodes, nerr)
			resultCh <- courseResult{index: i, vo: vo, err: nerr}
		}(i, c)
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]*CourseVO, len(list))
	timeout := time.After(75 * time.Second)
collect:
	for {
		select {
		case r, ok := <-resultCh:
			if !ok {
				break collect
			}
			results[r.index] = &r.vo
			if r.err != nil {
				partialFail++
			}
		case <-timeout:
			partialFail += len(list)
			break collect
		}
	}

	for i, c := range list {
		if results[i] != nil {
			vos = append(vos, *results[i])
			continue
		}
		vo := baseICVECourseVO(c)
		if vo.RawStatusText == "" {
			vo.RawStatusText = "任务点待同步"
		} else {
			vo.RawStatusText += " / 任务点待同步"
		}
		vos = append(vos, vo)
	}

	if partialFail > 0 && len(vos) > 0 {
		last := &vos[len(vos)-1]
		if last.RawStatusText == "" {
			last.RawStatusText = fmt.Sprintf("%d 门课程任务点待同步", partialFail)
		} else {
			last.RawStatusText += fmt.Sprintf("（另有 %d 门课程任务点待同步）", partialFail)
		}
	}
	return vos, nil
}

func buildICVECourseVO(c icveAction.IcveCourse, nodes []icveAction.IcveCourseNode, nerr error) CourseVO {
	vo := baseICVECourseVO(c)

	for _, n := range nodes {
		if !isICVEProgressNode(n.FileType) {
			continue
		}
		vo.JobCount++
		vo.HasProgress = true
		if n.Speed >= 100 {
			vo.JobFinishCount++
		}
	}
	if vo.JobCount > 0 {
		vo.JobRate = float64(vo.JobFinishCount) / float64(vo.JobCount) * 100
	}
	if nerr != nil {
		status := "任务点部分同步失败"
		if vo.JobCount == 0 {
			status = "任务点待同步"
		}
		if vo.RawStatusText == "" {
			vo.RawStatusText = status
		} else {
			vo.RawStatusText += " / " + status
		}
	}
	return vo
}

func baseICVECourseVO(c icveAction.IcveCourse) CourseVO {
	rawStatus := strings.TrimSpace(c.WeekStr)
	if c.Status == "3" {
		if rawStatus == "" {
			rawStatus = "课程已结束"
		} else {
			rawStatus += " / 课程已结束"
		}
	}
	return CourseVO{
		Platform:      "ICVE",
		Key:           c.Id,
		CourseID:      c.CourseId,
		CourseName:    c.CourseName,
		CourseTeacher: c.TeacherName,
		IsStart:       c.Status != "3",
		RawStatusText: rawStatus,
	}
}

type icveNodeRaw struct {
	ID                 string `json:"id"`
	CourseID           string `json:"courseId"`
	CourseInfoID       string `json:"courseInfoId"`
	ParentID           string `json:"parentId"`
	Name               string `json:"name"`
	FileType           string `json:"fileType"`
	IsLook             bool   `json:"isLook"`
	StudentStudyRecord *struct {
		Speed float64 `json:"speed"`
	} `json:"studentStudyRecord"`
}

func pullICVENodesFast(cache *icveApi.IcveUserCache, course icveAction.IcveCourse) ([]icveAction.IcveCourseNode, error) {
	rootURL := "https://zyk.icve.com.cn/prod-api/teacher/courseContent/studyMoudleList?courseInfoId=" + url.QueryEscape(course.CourseInfoId)
	raw, err := icveGET(cache, rootURL, 8*time.Second)
	if err != nil {
		return nil, err
	}
	var roots []icveNodeRaw
	if err := decodeICVENodeList(raw, &roots); err != nil {
		return nil, fmt.Errorf("解析根节点失败: %w", err)
	}

	nodes := make([]icveAction.IcveCourseNode, 0, 128)
	visited := map[string]bool{}
	var firstErr error
	for _, root := range roots {
		if err := collectICVENodes(cache, course, root, 1, visited, &nodes); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
		if len(nodes) >= 1000 {
			break
		}
	}
	if len(nodes) > 0 {
		return nodes, firstErr
	}
	return nodes, firstErr
}

func collectICVENodes(cache *icveApi.IcveUserCache, course icveAction.IcveCourse, root icveNodeRaw, level int, visited map[string]bool, out *[]icveAction.IcveCourseNode) error {
	if root.ID == "" || visited[root.ID] || len(*out) >= 1000 || level > 8 {
		return nil
	}
	visited[root.ID] = true

	switch root.FileType {
	case "父节点", "子节点", "":
		childURL := fmt.Sprintf(
			"https://zyk.icve.com.cn/prod-api/teacher/courseContent/studyList?level=%d&parentId=%s&courseInfoId=%s",
			level,
			url.QueryEscape(root.ID),
			url.QueryEscape(course.CourseInfoId),
		)
		raw, err := icveGET(cache, childURL, 8*time.Second)
		if err != nil {
			return err
		}
		var children []icveNodeRaw
		if err := decodeICVENodeList(raw, &children); err != nil {
			return fmt.Errorf("解析子节点失败: %w", err)
		}
		var firstErr error
		for _, child := range children {
			if err := collectICVENodes(cache, course, child, level+1, visited, out); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	default:
		speed := 0.0
		if root.StudentStudyRecord != nil {
			speed = root.StudentStudyRecord.Speed
		}
		*out = append(*out, icveAction.IcveCourseNode{
			Id:           root.ID,
			CourseId:     root.CourseID,
			CourseInfoId: root.CourseInfoID,
			ParentId:     root.ParentID,
			Name:         root.Name,
			FileType:     root.FileType,
			IsLook:       root.IsLook,
			Speed:        normalizeICVESpeed(speed, root.IsLook),
		})
	}
	return nil
}

func normalizeICVESpeed(speed float64, isLook bool) float64 {
	if speed > 0 {
		return speed
	}
	return 0
}

func decodeICVENodeList(raw string, out *[]icveNodeRaw) error {
	if err := json.Unmarshal([]byte(raw), out); err == nil {
		return nil
	}
	var wrapped struct {
		Data json.RawMessage `json:"data"`
		Rows json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err != nil {
		return err
	}
	for _, candidate := range []json.RawMessage{wrapped.Data, wrapped.Rows} {
		if len(candidate) == 0 || string(candidate) == "null" {
			continue
		}
		if err := json.Unmarshal(candidate, out); err == nil {
			return nil
		}
		var nested struct {
			List []icveNodeRaw `json:"list"`
			Rows []icveNodeRaw `json:"rows"`
			Data []icveNodeRaw `json:"data"`
		}
		if err := json.Unmarshal(candidate, &nested); err == nil {
			switch {
			case len(nested.List) > 0:
				*out = nested.List
				return nil
			case len(nested.Rows) > 0:
				*out = nested.Rows
				return nil
			case len(nested.Data) > 0:
				*out = nested.Data
				return nil
			}
		}
	}
	return fmt.Errorf("未识别的节点列表结构")
}

func icveGET(cache *icveApi.IcveUserCache, urlStr string, timeout time.Duration) (string, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if cache.IpProxySW && cache.ProxyIP != "" {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP)
		}
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", coreUtils.DefaultUserAgent)
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "zyk.icve.com.cn")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Referer", "https://zyk.icve.com.cn/")
	req.Header.Add("Origin", "https://zyk.icve.com.cn")
	req.Header.Add("Authorization", "Bearer "+cache.ZYKAccessToken)
	for _, cookie := range cache.Cookies {
		req.AddCookie(cookie)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}
	return string(body), nil
}

func isICVEProgressNode(fileType string) bool {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "mp4", "mp3", "zip", "pdf", "doc", "docx", "ppt", "pptx", "测验":
		return true
	default:
		return false
	}
}
