@echo off
setlocal EnableDelayedExpansion
set PASS=0
set FAIL=0

echo ============================================================
echo  yatori-go-desktop 构建验证
echo ============================================================

:: [1] Go
echo.
echo [1/6] 检查 Go ^>= 1.22 ...
go version 2>nul | findstr /r "go1\.[2-9][2-9]\|go1\.[3-9]" >nul
if errorlevel 1 (
    go version 2>nul
    echo  WARN: 建议 Go 1.22+ 以匹配原项目依赖
) else (
    echo  OK
)

:: [2] GCC
echo.
echo [2/6] 检查 GCC (CGO / SQLite 必须) ...
gcc --version >nul 2>&1
if errorlevel 1 (
    echo  FAIL: 未找到 gcc
    echo        请安装 TDM-GCC: https://jmeubank.github.io/tdm-gcc/
    set /a FAIL+=1
) else (
    echo  OK: !
    gcc --version 2>nul | findstr /i "gcc"
)

:: [3] Wails
echo.
echo [3/6] 检查 Wails v2 ...
wails version 2>nul | findstr /i "wails"
if errorlevel 1 (
    echo  FAIL: 未找到 wails
    echo        请执行: go install github.com/wailsapp/wails/v2/cmd/wails@latest
    set /a FAIL+=1
) else (
    echo  OK
)

:: [4] 原项目目录
echo.
echo [4/6] 检查 ../yatori-go-console 目录 ...
if not exist "..\yatori-go-console\go.mod" (
    echo  FAIL: 找不到 ..\yatori-go-console\go.mod
    echo        请执行: cd .. ^&^& git clone https://github.com/yatori-dev/yatori-go-console.git
    set /a FAIL+=1
) else (
    echo  OK: 原项目目录存在
)

if !FAIL! GTR 0 (
    echo.
    echo 有 !FAIL! 项依赖缺失，请修复后重试
    exit /b 1
)

:: [5] go mod tidy
echo.
echo [5/6] go mod tidy ...
set CGO_ENABLED=1
go mod tidy
if errorlevel 1 (
    echo  FAIL: go mod tidy 失败
    exit /b 1
)
echo  OK

:: [6] go vet + 单元测试
echo.
echo [6/6] go vet ^& go test ./service/...
go vet ./...
if errorlevel 1 (
    echo  WARN: go vet 报告了问题（见上）
)
go test ./service/... -run "TestTaskManager|TestDecodePassword|TestValidateConfig|TestSaveAndLoad" -v -timeout 30s
if errorlevel 1 (
    echo  FAIL: 单元测试失败
    exit /b 1
)
echo  OK

echo.
echo ============================================================
echo  验证通过！下一步：
echo    wails dev          开发模式（热重载）
echo    build.bat          构建 release exe
echo ============================================================
