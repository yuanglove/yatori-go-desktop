# Yatori Go Desktop

Yatori Go Desktop 是基于 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造的 Windows 桌面版工具。

原项目提供命令行和网页模式的学习任务执行能力。本项目在其基础上增加了 Wails 桌面 GUI、账号管理、任务控制、日志中心、全局设置、主题切换、关于本项目页面和版本检测能力，使主要操作可以通过窗口完成。

> 本项目由 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造而来。感谢原项目作者和相关开源依赖。

## 当前版本

v0.2.91

## 本项目做了什么

- 使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面。
- 复用 `yatori-go-console` 的核心 Go 逻辑。
- 增加账号管理、任务控制、日志中心、全局设置、关于本项目页面。
- 通过 worker 子进程执行任务，任务停止采用硬停止方式。
- 支持学习通、英华学堂、海旗科技、WeLearn 随行课堂及更多原项目平台的桌面任务入口。
- 新增课程进度页面，支持学习通、英华学堂、海旗科技、WeLearn 随行课堂课程列表和进度展示。
- 支持学习通章节测验 AI / 题库答题配置。
- 支持 9 套主题，并在全局设置中持久化保存。
- 增加 GitHub 自动检测新版本和手动检测入口。
- 统一日志展示，并优化 Windows 中文乱码处理。
- 配置、数据库和日志统一保存到 `%APPDATA%\yatori-go-console`。

## 免责声明

本项目代码已开源，仅供学习与技术交流使用，严禁任何形式的倒卖、贩卖或商业牟利。

请在遵守法律法规、平台规则和账号授权范围的前提下使用本软件。任何个人或组织使用本项目代码或软件进行的违法违规行为，均与项目作者无关，相关责任由使用者自行承担。

如本项目内容对相关公司或平台造成影响，请通过 GitHub 仓库联系，我会及时处理。

## 本项目架构

```text
yatori-go-desktop/
├── main.go                         Wails 入口，支持普通桌面模式与 --worker 子进程模式
├── app.go                          Wails 绑定层，暴露给前端调用的 Go 方法
├── worker.go                       任务 worker 入口，按平台分发执行逻辑
├── worker_utf8_windows.go          Windows worker 输出编码处理
├── service/
│   ├── account_service.go          账号 CRUD、仪表盘数据
│   ├── config_service.go           config.yaml 读写、默认值、校验
│   ├── log_service.go              日志缓冲、文件 tail、乱码修复
│   ├── task_manager.go             任务生命周期、worker 子进程启动与硬停止
│   ├── course_service.go           课程进度平台分发入口
│   ├── course_xxt.go               学习通课程进度适配
│   ├── course_yinghua.go           英华学堂课程进度适配
│   ├── course_hqkj.go              海旗科技课程进度适配
│   ├── course_welearn.go           WeLearn 随行课堂课程进度适配
│   ├── platform.go                 平台支持状态表
│   ├── worker_helpers.go           worker 构建用户和平台对象的辅助方法
│   ├── xxt_runner.go               学习通安全 runner
│   ├── yinghua_runner.go           英华学堂 runner
│   └── other_runners.go            其他平台 worker 入口
├── frontend/
│   ├── src/components/Layout.tsx    主布局、导航、启动时主题和版本检测
│   ├── src/pages/                  仪表盘、账号、任务、课程进度、日志、设置、关于页面
│   ├── src/lib/api.ts              前端调用 Wails Go 方法的封装
│   ├── src/lib/theme.ts            主题列表与主题应用
│   ├── src/lib/update.ts           GitHub 版本检测
│   └── src/lib/version.ts          当前版本和仓库地址常量
└── wails.json                      Wails 项目配置和 exe 元信息
```

运行流程：

```text
React 前端页面
  ↓ Wails binding
app.go
  ↓
service.TaskManager
  ↓ 启动独立 worker 子进程
worker.go
  ↓ 按账号平台分发
service/*_runner.go
  ↓ 复用原项目逻辑
yatori-go-console/logic/{platform}
```

任务停止流程：

```text
前端点击停止
  ↓
TaskManager.Stop(uid)
  ↓
taskkill /T /F 结束 worker 子进程树
  ↓
主窗口保持运行
```

数据路径：

```text
%APPDATA%\yatori-go-console\
├── config.yaml
├── yatori.db
└── assets\log\
```

## 功能状态

| 功能 | 状态 | 说明 |
| --- | --- | --- |
| 桌面 GUI | 支持 | Wails v2 桌面窗口 |
| 账号管理 | 支持 | 增删改账号和平台配置 |
| 任务控制 | 支持 | 启动、硬停止、状态展示 |
| 日志中心 | 支持 | 历史日志 + 实时日志 |
| 课程进度 | 支持 | 学习通、英华学堂、海旗科技、WeLearn 随行课堂已接入 |
| 全局设置 | 支持 | AI、题库、邮件、日志、主题 |
| 主题切换 | 支持 | 9 套主题，保存到 config.yaml |
| 版本检测 | 支持 | 自动从 GitHub Releases/Tags 检测新版本 |
| 学习通 | 已实测可用 | 支持任务执行、日志输出、硬停止、章节测验答题等桌面流程 |
| 英华学堂 | 已实测可用 | 已修复 worker runtime 初始化问题，可正常登录并进入课程流程 |
| 海旗科技 | 已实测可用 | worker 子进程运行正常 |
| WeLearn 随行课堂 | 已实测可用 | worker 子进程运行正常 |
| 其他原项目平台 | 支持入口 | 复用原项目 logic，具体效果需按账号继续实测 |

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
