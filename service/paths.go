package service

import (
	"os"
	"path/filepath"
)

// DataDir 返回用户数据目录：%APPDATA%/yatori-go-console
func DataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "yatori-go-console")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// DefaultConfigPath 返回默认 config.yaml 路径
func DefaultConfigPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// DBPath 返回 SQLite 数据库路径
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "yatori.db"), nil
}

// LogDir 返回日志目录路径，确保目录存在
func LogDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(dir, "assets", "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return logDir, nil
}
