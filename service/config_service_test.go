package service

import (
	"os"
	"testing"
)

func TestValidateConfig_OK(t *testing.T) {
	cfg := defaultConfig()
	if errs := ValidateConfig(cfg); len(errs) != 0 {
		t.Fatalf("期望无错误，得到: %v", errs)
	}
}

func TestValidateConfig_BadLogLevel(t *testing.T) {
	cfg := defaultConfig()
	cfg.Setting.BasicSetting.LogLevel = "VERBOSE"
	if errs := ValidateConfig(cfg); len(errs) == 0 {
		t.Fatal("期望有错误，但无错误返回")
	}
}

func TestValidateConfig_EmailMissingHost(t *testing.T) {
	cfg := defaultConfig()
	cfg.Setting.EmailInform.Sw = 1
	cfg.Setting.EmailInform.SMTPPort = 465
	if errs := ValidateConfig(cfg); len(errs) == 0 {
		t.Fatal("期望有 SMTP Host 错误")
	}
}

func TestLoadConfig_NotExist(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("不存在时应返回默认配置，得到错误: %v", err)
	}
	if cfg.Setting.BasicSetting.LogLevel != "INFO" {
		t.Error("默认 LogLevel 应为 INFO")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	f, err := os.CreateTemp("", "yatori-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	cfg := defaultConfig()
	cfg.Setting.BasicSetting.LogLevel = "DEBUG"
	cfg.Users = []User{{AccountType: "YINGHUA", Account: "test@test.com", Password: "pw"}}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if loaded.Setting.BasicSetting.LogLevel != "DEBUG" {
		t.Error("LogLevel 未正确保存/加载")
	}
	if len(loaded.Users) != 1 || loaded.Users[0].Account != "test@test.com" {
		t.Error("Users 未正确保存/加载")
	}
}
