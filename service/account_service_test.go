package service

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB 创建内存 SQLite，并替换全局 db
func setupTestDB(t *testing.T) func() {
	t.Helper()
	var err error
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := testDB.AutoMigrate(&AccountPO{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	old := db
	db = testDB
	return func() {
		db = old
	}
}

func baseReq(platform, account, url, password string) AccountReq {
	return AccountReq{
		AccountType: platform,
		Account:     account,
		URL:         url,
		Password:    password,
		CoursesCustom: CoursesCustom{
			VideoModel: 1,
			AutoExam:   0,
		},
	}
}

// 1. 同平台同入口同账号 → 拒绝
func TestAddAccount_DuplicateSamePlatformSameURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := AddAccount(baseReq("XUEXITONG", "user@test.com", "", "pwd")); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := AddAccount(baseReq("XUEXITONG", "user@test.com", "", "pwd2")); err == nil {
		t.Fatal("expected error for duplicate XUEXITONG account, got nil")
	}
}

// 2. 英华同账号不同入口 → 允许
func TestAddAccount_YinghuaDifferentURL_Allowed(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := AddAccount(baseReq("YINGHUA", "user@test.com", "https://a.yinghua.com", "pwd")); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := AddAccount(baseReq("YINGHUA", "user@test.com", "https://b.yinghua.com", "pwd")); err != nil {
		t.Fatalf("different URL should be allowed, got: %v", err)
	}
}

// 3. 英华同账号同入口 → 拒绝
func TestAddAccount_YinghuaSameURL_Blocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	url := "https://school.yinghua.com"
	if err := AddAccount(baseReq("YINGHUA", "user@test.com", url, "pwd")); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := AddAccount(baseReq("YINGHUA", "user@test.com", url, "pwd2")); err == nil {
		t.Fatal("expected error for duplicate YINGHUA+same URL, got nil")
	}
}

// 4. 不同平台同账号 → 允许
func TestAddAccount_DifferentPlatform_Allowed(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := AddAccount(baseReq("XUEXITONG", "user@test.com", "", "pwd")); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := AddAccount(baseReq("ENAEA", "user@test.com", "", "pwd")); err != nil {
		t.Fatalf("different platform should be allowed, got: %v", err)
	}
}

func TestUpdateAccount_DuplicateSamePlatformSameURL_Blocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	if err := AddAccount(baseReq("YINGHUA", "a@test.com", "https://school.yinghua.com", "pwd")); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := AddAccount(baseReq("YINGHUA", "b@test.com", "https://school.yinghua.com", "pwd")); err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	accounts, err := ListAccounts(NewTaskManager(func(string, string) {}))
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	var second AccountVO
	for _, a := range accounts {
		if a.Account == "b@test.com" {
			second = a
			break
		}
	}
	if second.UID == "" {
		t.Fatal("second account not found")
	}

	req := baseReq("YINGHUA", "a@test.com", "https://school.yinghua.com", "")
	req.UID = second.UID
	if err := UpdateAccount(req); err == nil {
		t.Fatal("expected duplicate update to be blocked, got nil")
	}
}

// 清理：确保测试不会产生文件副作用
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
