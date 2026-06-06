# Yatori Go Desktop

Yatori Go Desktop 是基于 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造的 Windows 桌面版工具。

原项目提供了命令行/网页模式的学习任务执行能力，本项目在其基础上增加了 Wails 桌面 GUI、账号管理、任务控制、日志中心、全局设置和关于页面，使主要操作可以通过窗口完成。

> 本项目由 yatori-dev/yatori-go-console 改造而来，原项目地址：https://github.com/yatori-dev/yatori-go-console

> 感谢原项目作者和相关开源依赖。本项目保留原项目核心逻辑，在外层增加桌面 GUI、任务管理、日志中心和配置界面。

---

## 项目简介

学习通等课程平台的任务自动化工具，具备 Windows 桌面 GUI，支持多账号管理、任务启停、日志查看和全局配置。

---

## 本项目做了什么

- 使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面
- 保留并复用 yatori-go-console 的核心 Go 逻辑
- 增加账号管理、任务控制、日志中心、全局设置、关于页面
- 支持学习通任务启动与硬停止
- 支持学习通多课程 / 多任务点配置
- 支持章节测试 AI 答题配置
- 统一日志格式为：`[时间] [平台][账号] 【课程】【当前任务点】【资源/章节测试标题】消息`
- 配置、数据库和日志统一保存到 `%APPDATA%\yatori-go-console`

---

## 功能特性

| 功能 | 状态 | 说明 |
|------|------|------|
| 学习通单账号 GUI 控制 | ✅ 支持 | 登录 / 启动 / 停止 |
| 其他平台 GUI 控制 | ⚠️ 仅配置 | 用 `-cli` 运行 |
| 学习通 CxNode 并发数 | ✅ 支持 | 账号编辑页可配置 |
| 章节测试 AI 答题 | ✅ 支持 | 需配置 API Key |
| 验证码 / 人脸识别绕过 | ❌ 不支持 | 不提供此能力 |

---

## 安装使用

双击 `build\bin\yatori-go-desktop.exe`，无需安装。

启动后自动创建数据目录，点击"关于"页的"打开数据目录"可快速访问。

---

## 从源码构建

**前置要求：**

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.22+ | https://go.dev/dl/ |
| TDM-GCC | 最新 | SQLite CGO 必须，https://jmeubank.github.io/tdm-gcc/ |
| Node.js | 18+ | https://nodejs.org/ |
| Wails v2 | 最新 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

**目录结构：**

当前 `go.mod` 使用 `replace yatori-go-console => ../yatori-go-console`，所以源码构建需要两个项目并列：

```
学习/
├── yatori-go-console/    ← git clone https://github.com/yatori-dev/yatori-go-console.git
└── yatori-go-desktop/    ← 本项目
```

**构建命令：**

```bat
cd yatori-go-desktop
go vet ./...
go test ./service/... -v
cd frontend && npm install && npm run build && cd ..
wails build -platform windows/amd64 -o yatori-go-desktop.exe
```

**开发模式：**

```bat
wails dev
```

---

## 目录结构要求

```
学习/
├── yatori-go-console/   ← 原项目（核心逻辑，go.mod replace 依赖）
└── yatori-go-desktop/   ← 本项目（桌面 GUI）
```

两个目录必须并列，否则 `go mod tidy` 会失败。

---

## 数据目录说明

```
%APPDATA%\yatori-go-console\
├── config.yaml       ← 全局配置（首次启动自动创建默认值）
├── yatori.db         ← 账号数据库
└── assets\log\       ← 日志文件
```

---

## 已知限制

- GUI 主要支持学习通，其他平台可能仅支持配置或 CLI
- AI 答题依赖用户自己的 API Key
- 不提供验证码破解、人脸绕过、考试作弊等能力
- 本项目仅用于个人已授权账号的学习任务管理

---

## 安全与合规声明

> 本项目仅用于个人已授权账号的学习任务管理。不提供验证码破解、人脸识别绕过、考试作弊等能力。使用者须自行承担使用风险，遵守相关平台用户协议。

---

## 更新日志

见 [CHANGELOG.md](./CHANGELOG.md) 或 [Releases](https://github.com/yuanglove/yatori-go-desktop/releases)。

---

## 致谢

- [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) — 核心学习逻辑来源
- [Wails](https://wails.io/) — Go + Web 桌面框架
- [React](https://react.dev/) + [TypeScript](https://www.typescriptlang.org/)
- 相关 Go 开源依赖（见 go.mod）

---

## License

原项目 yatori-go-console 使用 MIT License，本项目作为改造版本同样遵循 MIT License。原项目版权归原作者所有。

详见 [LICENSE](./LICENSE)。
