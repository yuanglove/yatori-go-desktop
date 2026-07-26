package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type XXTExamCodeRequest struct {
	ID        string `json:"id"`
	UID       string `json:"uid"`
	Account   string `json:"account"`
	ExamName  string `json:"examName"`
	TaskRefID string `json:"taskRefId"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	Status    string `json:"status"` // pending / answered / cancelled
	Code      string `json:"code,omitempty"`
}

var xxtExamCodeRequestMu sync.Mutex

func xxtExamCodeRequestPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "xxt-exam-code-requests-v2.json"), nil
}

func readXXTExamCodeRequestsLocked() ([]XXTExamCodeRequest, error) {
	path, err := xxtExamCodeRequestPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []XXTExamCodeRequest{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return []XXTExamCodeRequest{}, nil
	}
	var list []XXTExamCodeRequest
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	out := list[:0]
	for _, r := range list {
		if r.Status == "pending" && r.ExpiresAt > 0 && r.ExpiresAt < now {
			r.Status = "cancelled"
		}
		// 保留最近一天内的记录，避免状态文件无限增长。
		if r.CreatedAt == 0 || now-r.CreatedAt <= 24*3600 || r.Status == "pending" {
			out = append(out, r)
		}
	}
	return out, nil
}

func writeXXTExamCodeRequestsLocked(list []XXTExamCodeRequest) error {
	path, err := xxtExamCodeRequestPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func WaitForXXTExamCode(uid, account, examName, taskRefID string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	now := time.Now()
	id := fmt.Sprintf("%s:%s:%s:%d", strings.TrimSpace(uid), strings.TrimSpace(taskRefID), strings.TrimSpace(examName), now.UnixNano())
	if strings.Trim(id, ": ") == "" {
		id = fmt.Sprintf("xxt:%d", now.UnixNano())
	}
	req := XXTExamCodeRequest{
		ID:        id,
		UID:       uid,
		Account:   account,
		ExamName:  examName,
		TaskRefID: taskRefID,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(timeout).Unix(),
		Status:    "pending",
	}

	xxtExamCodeRequestMu.Lock()
	list, err := readXXTExamCodeRequestsLocked()
	if err == nil {
		next := make([]XXTExamCodeRequest, 0, len(list)+1)
		for _, item := range list {
			if item.Status == "pending" &&
				strings.TrimSpace(item.UID) == strings.TrimSpace(uid) &&
				strings.TrimSpace(item.TaskRefID) == strings.TrimSpace(taskRefID) &&
				strings.TrimSpace(item.ExamName) == strings.TrimSpace(examName) {
				item.Status = "cancelled"
			}
			next = append(next, item)
		}
		list = append(next, req)
		err = writeXXTExamCodeRequestsLocked(list)
	}
	xxtExamCodeRequestMu.Unlock()
	if err != nil {
		return "", err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		xxtExamCodeRequestMu.Lock()
		list, err := readXXTExamCodeRequestsLocked()
		xxtExamCodeRequestMu.Unlock()
		if err != nil {
			return "", err
		}
		for _, r := range list {
			if r.ID != id {
				continue
			}
			switch r.Status {
			case "answered":
				code := strings.TrimSpace(r.Code)
				if code == "" {
					return "", fmt.Errorf("考试码为空")
				}
				return code, nil
			case "cancelled":
				return "", fmt.Errorf("用户取消输入考试码")
			}
		}
	}
	return "", fmt.Errorf("等待考试码超时")
}

func ListPendingXXTExamCodeRequests() ([]XXTExamCodeRequest, error) {
	xxtExamCodeRequestMu.Lock()
	defer xxtExamCodeRequestMu.Unlock()
	list, err := readXXTExamCodeRequestsLocked()
	if err != nil {
		return nil, err
	}
	out := make([]XXTExamCodeRequest, 0)
	for _, r := range list {
		if r.Status == "pending" {
			out = append(out, r)
		}
	}
	return out, nil
}

func AnswerXXTExamCodeRequest(id, code string) error {
	id = strings.TrimSpace(id)
	code = strings.TrimSpace(code)
	if id == "" {
		return fmt.Errorf("请求 ID 不能为空")
	}
	if code == "" {
		return fmt.Errorf("考试码不能为空")
	}
	return updateXXTExamCodeRequest(id, "answered", code)
}

func CancelXXTExamCodeRequest(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("请求 ID 不能为空")
	}
	return updateXXTExamCodeRequest(id, "cancelled", "")
}

func updateXXTExamCodeRequest(id, status, code string) error {
	xxtExamCodeRequestMu.Lock()
	defer xxtExamCodeRequestMu.Unlock()
	list, err := readXXTExamCodeRequestsLocked()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID == id {
			list[i].Status = status
			list[i].Code = code
			return writeXXTExamCodeRequestsLocked(list)
		}
	}
	return fmt.Errorf("未找到考试码请求")
}
