package service

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

func sysLog(format string, args ...interface{}) string {
	return fmt.Sprintf("[%s] [系统] %s",
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...))
}

type TaskState string

const (
	StateRunning TaskState = "running"
	StateStopped TaskState = "stopped"
	StateFailed  TaskState = "failed"
)

type TaskStatus struct {
	UID       string    `json:"uid"`
	Account   string    `json:"account"`
	Platform  string    `json:"platform"`
	State     TaskState `json:"state"`
	StartTime string    `json:"startTime,omitempty"`
	LastLog   string    `json:"lastLog,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type taskEntry struct {
	mu      sync.Mutex
	status  TaskStatus
	cmd     *exec.Cmd
	stopped bool
	logDone chan struct{}
}

type LogEmitter func(uid, msg string)

type TaskManager struct {
	mu         sync.RWMutex
	tasks      map[string]*taskEntry
	emitter    LogEmitter
	maxWorkers int
}

const maxConcurrentAccountTasks = 5

func NewTaskManager(emitter LogEmitter) *TaskManager {
	return &TaskManager{tasks: make(map[string]*taskEntry), emitter: emitter, maxWorkers: maxConcurrentAccountTasks}
}

func (m *TaskManager) SetConfigPath(_ string) {}

// SetMaxWorkers remains for compatibility with older callers. Desktop task
// concurrency is deliberately fixed at five accounts.
func (m *TaskManager) SetMaxWorkers(_ int) {
	m.mu.Lock()
	m.maxWorkers = maxConcurrentAccountTasks
	m.mu.Unlock()
}

func (m *TaskManager) HasRunningTasks() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runningCount() > 0
}

func (m *TaskManager) runningCount() int {
	count := 0
	for _, e := range m.tasks {
		e.mu.Lock()
		if e.status.State == StateRunning {
			count++
		}
		e.mu.Unlock()
	}
	return count
}

func (m *TaskManager) Start(uid string) error {
	po, err := getAccountPO(uid)
	if err != nil {
		return fmt.Errorf("账号不存在: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.tasks[uid]; ok {
		e.mu.Lock()
		state := e.status.State
		e.mu.Unlock()
		if state == StateRunning {
			return fmt.Errorf("账号 %s 已在运行中", uid)
		}
	}

	running := m.runningCount()
	if running >= m.maxWorkers {
		return fmt.Errorf("当前运行任务数已达上限：%d", m.maxWorkers)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行路径: %w", err)
	}

	cmd := exec.Command(exe, "--worker", "--uid", uid)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("无法创建 stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 worker 失败: %w", err)
	}

	e := &taskEntry{
		status: TaskStatus{
			UID:       uid,
			Account:   po.Account,
			Platform:  po.AccountType,
			State:     StateRunning,
			StartTime: time.Now().Format("2006-01-02 15:04:05"),
		},
		cmd:     cmd,
		stopped: false,
		logDone: make(chan struct{}),
	}
	m.tasks[uid] = e

	if m.emitter != nil {
		m.emitter(uid, sysLog("worker 并发：当前 %d/%d", running+1, m.maxWorkers))
		m.emitter(uid, sysLog("worker 子进程已启动 pid=%d", cmd.Process.Pid))
	}

	go m.pipeLog(uid, e, stdout)
	go func() {
		_ = cmd.Wait()
		m.mu.RLock()
		entry, ok := m.tasks[uid]
		m.mu.RUnlock()
		if ok {
			entry.mu.Lock()
			if entry.status.State == StateRunning {
				entry.status.State = StateStopped
			}
			entry.mu.Unlock()
		}
	}()

	return nil
}

func (m *TaskManager) Stop(uid string) {
	m.mu.RLock()
	e, ok := m.tasks[uid]
	m.mu.RUnlock()
	if !ok {
		return
	}

	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.stopped = true
	pid := 0
	if e.cmd != nil && e.cmd.Process != nil {
		pid = e.cmd.Process.Pid
	}
	e.mu.Unlock()

	select {
	case <-e.logDone:
	default:
		close(e.logDone)
	}

	if pid > 0 {
		if out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput(); err != nil {
			if m.emitter != nil {
				m.emitter(uid, sysLog("taskkill pid=%d 失败: %v output=%s", pid, err, string(out)))
			}
		}
		if e.cmd.Process != nil {
			_ = e.cmd.Process.Kill()
		}
	}

	e.mu.Lock()
	e.status.State = StateStopped
	e.mu.Unlock()

	if m.emitter != nil {
		m.emitter(uid, sysLog("已强制停止任务 pid=%d", pid))
	}
}

func (m *TaskManager) StopAll() {
	m.mu.RLock()
	uids := make([]string, 0, len(m.tasks))
	for uid := range m.tasks {
		uids = append(uids, uid)
	}
	m.mu.RUnlock()
	for _, uid := range uids {
		m.Stop(uid)
	}
}

func (m *TaskManager) pipeLog(uid string, e *taskEntry, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-e.logDone:
			for scanner.Scan() {
			}
			return
		default:
		}
		e.mu.Lock()
		stopped := e.stopped
		e.mu.Unlock()
		if stopped {
			for scanner.Scan() {
			}
			return
		}
		line := NormalizeLogText(scanner.Text())
		if m.emitter != nil {
			m.emitter(uid, line)
		}
		m.setLog(uid, line)
	}
}

func (m *TaskManager) Statuses() []TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]TaskStatus, 0, len(m.tasks))
	for _, e := range m.tasks {
		e.mu.Lock()
		out = append(out, e.status)
		e.mu.Unlock()
	}
	return out
}

func (m *TaskManager) IsRunning(uid string) bool {
	m.mu.RLock()
	e, ok := m.tasks[uid]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status.State == StateRunning
}

func (m *TaskManager) setLog(uid, msg string) {
	m.mu.RLock()
	e, ok := m.tasks[uid]
	m.mu.RUnlock()
	if !ok {
		return
	}
	e.mu.Lock()
	e.status.LastLog = msg
	e.mu.Unlock()
}

func DecodePassword(enc string) string { return decodePassword(enc) }
func decodePassword(enc string) string {
	const prefix = "b64:"
	if len(enc) <= len(prefix) {
		return enc
	}
	b, err := base64.StdEncoding.DecodeString(enc[len(prefix):])
	if err != nil {
		return enc
	}
	return string(b)
}
