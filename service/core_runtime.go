package service

import (
	"sync"

	coreUtils "github.com/yatori-dev/yatori-go-core/utils"
)

var coreRuntimeOnce sync.Once

func EnsureCoreRuntime() {
	coreRuntimeOnce.Do(func() {
		coreUtils.YatoriCoreInit()
	})
}
