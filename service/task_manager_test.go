package service

import "testing"

func TestTaskManager_PreventDuplicate(t *testing.T) {
	mgr := NewTaskManager(nil)
	mgr.mu.Lock()
	mgr.tasks["uid-1"] = &taskEntry{status: TaskStatus{UID: "uid-1", State: StateRunning}}
	mgr.mu.Unlock()

	if !mgr.IsRunning("uid-1") {
		t.Fatal("uid-1 应为 running")
	}
	if mgr.IsRunning("uid-2") {
		t.Fatal("uid-2 未启动，应为 false")
	}
}

func TestTaskManager_StopIdempotent(t *testing.T) {
	mgr := NewTaskManager(nil)
	mgr.Stop("not-exist") // 不应 panic

	mgr.mu.Lock()
	mgr.tasks["uid-2"] = &taskEntry{
		status:  TaskStatus{UID: "uid-2", State: StateRunning},
		stopped: false,
		logDone: make(chan struct{}),
	}
	mgr.mu.Unlock()

	mgr.Stop("uid-2")
	mgr.Stop("uid-2") // 幂等，不 panic

	for _, s := range mgr.Statuses() {
		if s.State == StateRunning {
			t.Errorf("停止后 %s 仍为 running", s.UID)
		}
	}
}

func TestTaskManager_StopAll(t *testing.T) {
	mgr := NewTaskManager(nil)
	for _, uid := range []string{"a", "b", "c"} {
		u := uid
		mgr.mu.Lock()
		mgr.tasks[u] = &taskEntry{
			status:  TaskStatus{UID: u, State: StateRunning},
			logDone: make(chan struct{}),
		}
		mgr.mu.Unlock()
	}
	mgr.StopAll()
	for _, s := range mgr.Statuses() {
		if s.State == StateRunning {
			t.Errorf("StopAll 后 %s 仍为 running", s.UID)
		}
	}
}

func TestDecodePassword(t *testing.T) {
	plain := "test-password-123"
	// base64("test-password-123") = "dGVzdC1wYXNzd29yZC0xMjM="
	enc := "b64:dGVzdC1wYXNzd29yZC0xMjM="
	if got := decodePassword(enc); got != plain {
		t.Errorf("want %q got %q", plain, got)
	}
	if got := decodePassword("plain"); got != "plain" {
		t.Error("非混淆密码应原样返回")
	}
}
