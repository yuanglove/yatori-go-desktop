package service

import (
	"fmt"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

// InitDB 初始化 SQLite，幂等，并发安全
func InitDB() error {
	dbOnce.Do(func() {
		path, err := DBPath()
		if err != nil {
			dbErr = err
			return
		}
		db, err = gorm.Open(sqlite.Open(path), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			dbErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}
		dbErr = db.AutoMigrate(&AccountPO{})
	})
	return dbErr
}

type AccountPO struct {
	UID           string `gorm:"primaryKey"`
	AccountType   string
	URL           string
	RemarkName    string
	Account       string
	PasswordEnc   string
	IsProxy       int
	InformEmails  string
	CoursesCustom string
}

type AccountVO struct {
	UID           string        `json:"uid"`
	AccountType   string        `json:"accountType"`
	URL           string        `json:"url"`
	RemarkName    string        `json:"remarkName"`
	Account       string        `json:"account"`
	IsProxy       int           `json:"isProxy"`
	IsRunning     bool          `json:"isRunning"`
	GUISupport    string        `json:"guiSupport"`
	CoursesCustom CoursesCustom `json:"coursesCustom"`
	InformEmails  []string      `json:"informEmails"`
}

type AccountReq struct {
	UID           string        `json:"uid"`
	AccountType   string        `json:"accountType"`
	URL           string        `json:"url"`
	RemarkName    string        `json:"remarkName"`
	Account       string        `json:"account"`
	Password      string        `json:"password"`
	IsProxy       int           `json:"isProxy"`
	InformEmails  []string      `json:"informEmails"`
	CoursesCustom CoursesCustom `json:"coursesCustom"`
}

type Dashboard struct {
	TotalAccounts int      `json:"totalAccounts"`
	RunningTasks  int      `json:"runningTasks"`
	ConfigPath    string   `json:"configPath"`
	ConfigOK      bool     `json:"configOK"`
	RecentLogs    []string `json:"recentLogs"`
}
