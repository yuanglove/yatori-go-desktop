# Yatori Go Desktop

Yatori Go Desktop 是基于 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造的 Windows 桌面版工具。

原项目提供命令行和网页模式的学习任务执行能力。本项目在其基础上增加了 Wails 桌面 GUI、账号管理、任务控制、日志中心、全局设置、主题切换和关于本项目页面，使主要操作可以通过窗口完成。

> 本项目由 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造而来。感谢原项目作者和相关开源依赖。

## 当前版本

v0.2.0

## 本项目做了什么

- 使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面。
- 复用 `yatori-go-console` 的核心 Go 逻辑。
- 增加账号管理、任务控制、日志中心、全局设置、关于本项目页面。
- 通过 worker 子进程执行任务，任务停止采用硬停止方式。
- 支持学习通、英华学堂及更多原项目平台的桌面任务入口。
- 支持学习通章节测验 AI / 题库答题配置。
- 支持多套主题，并在全局设置中持久化保存。
- 统一日志展示，并优化 Windows 中文乱码处理。
- 配置、数据库和日志统一保存到 `%APPDATA%\yatori-go-console`。

## 功能状态

| 功能 | 状态 | 说明 |
| --- | --- | --- |
| 桌面 GUI | 支持 | Wails v2 桌面窗口 |
| 账号管理 | 支持 | 增删改账号和平台配置 |
| 任务控制 | 支持 | 启动、硬停止、状态展示 |
| 日志中心 | 支持 | 历史日志 + 实时日志 |
| 全局设置 | 支持 | AI、题库、邮件、日志、主题 |
| 主题切换 | 支持 | 9 套主题，保存到 config.yaml |
| 学习通 | 支持 | 已做较完整桌面适配 |
| 英华学堂 | 支持 | worker 子进程入口 |
| 其他平台 | 支持入口 | 复用原项目 logic，具体效果需按账号实测 |

## 安装使用

下载或构建后，双击：

```text
build\bin\yatori-go-desktop.exe
```

首次启动会自动创建数据目录：

```text
%APPDATA%\yatori-go-console\
```

## 从源码构建

前置要求：

| 工具 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.22+ | 需要 CGO |
| GCC | Windows 可用 GCC | SQLite / Wails 构建需要 |
| Node.js | 18+ | 前端构建 |
| Wails v2 | 最新版 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

目录结构要求：

```text
学习/
├── yatori-go-console/   原项目
└── yatori-go-desktop/   本项目
```

构建命令：

```bat
cd yatori-go-desktop
go vet ./...
go test ./service/... -v
cd frontend && npm install && npm run build && cd ..
wails build -platform windows/amd64 -o yatori-go-desktop.exe
```

## 数据目录

```text
%APPDATA%\yatori-go-console\
├── config.yaml
├── yatori.db
└── assets\log\
```

## 已知限制

- 各平台的实际任务执行能力取决于原项目 `logic/{platform}` 的实现和平台接口状态。
- AI 答题依赖用户自行配置的 API Key 或题库接口。
- 不提供验证码破解、人脸识别绕过、考试作弊等能力。
- 本项目仅用于个人已授权账号的学习任务管理。

## 安全与合规声明

本项目仅用于个人已授权账号的学习任务管理。使用者应自行遵守相关平台用户协议、学校或机构规定，并承担使用风险。

## 更新日志

见 [CHANGELOG.md](./CHANGELOG.md) 或 [Releases](https://github.com/yuanglove/yatori-go-desktop/releases)。

## 致谢

- [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) - 核心学习逻辑来源
- [Wails](https://wails.io/) - Go + Web 桌面框架
- [React](https://react.dev/) + [TypeScript](https://www.typescriptlang.org/)

## License

本项目遵循 MIT License。原项目版权归原作者所有。

详见 [LICENSE](./LICENSE)。
