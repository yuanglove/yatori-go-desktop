package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	coreUtils "github.com/yatori-dev/yatori-go-core/utils"
)

var coreRuntimeOnce sync.Once
var coreRuntimeErr error

func EnsureCoreRuntime() error {
	coreRuntimeOnce.Do(func() {
		coreRuntimeErr = runCoreRuntimeInit()
	})
	return coreRuntimeErr
}

func runCoreRuntimeInit() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	dataDir, dataErr := DataDir()
	if dataErr != nil {
		return fmt.Errorf("获取数据目录失败: %w", dataErr)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "assets"), 0755); err != nil {
		return fmt.Errorf("创建 OCR 资源目录失败: %w", err)
	}
	if err := os.Chdir(dataDir); err != nil {
		return fmt.Errorf("切换 OCR 工作目录失败: %w", err)
	}
	if wd, wdErr := os.Getwd(); wdErr == nil {
		if !filepath.IsAbs(wd) {
			return fmt.Errorf("OCR 工作目录异常: %s", wd)
		}
	}

	coreUtils.YatoriCoreInit()
	return nil
}
