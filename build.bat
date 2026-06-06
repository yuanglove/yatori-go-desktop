@echo off
REM ============================================================
REM  yatori-go-desktop  Windows 构建脚本
REM  前提：已安装 Go 1.21+、Wails v2、TDM-GCC 或 MSYS2 mingw64
REM ============================================================

echo [1/4] 检查环境...
where go >nul 2>&1 || (echo 错误: 未找到 go & exit /b 1)
where wails >nul 2>&1 || (echo 错误: 未找到 wails，请先执行 go install github.com/wailsapp/wails/v2/cmd/wails@latest & exit /b 1)
where gcc >nul 2>&1 || (echo 错误: 未找到 gcc，请安装 TDM-GCC 或 MSYS2 mingw64 & exit /b 1)

echo [2/4] 下载 Go 依赖...
go mod tidy || exit /b 1

echo [3/4] 安装前端依赖...
cd frontend && npm install && cd ..

echo [4/4] 构建 Windows exe...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
wails build -platform windows/amd64 -o yatori-go-desktop.exe

echo.
echo ✓ 构建完成，输出目录：build\bin\
