# Yatori Go Desktop

Yatori Go Desktop 是基于 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造的学习任务管理桌面应用。项目在原 Go 核心能力基础上提供 Wails 桌面 GUI、账号管理、任务控制、日志中心、课程进度、全局设置、题库/AI 配置、公告、版本检测，以及面向 Android 的 `mobilecore` 产物。

> 本项目仅用于个人已授权账号的学习任务管理和技术交流。请遵守法律法规、平台规则和账号授权范围。

> 本项目由 [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console) 改造而来。感谢原项目作者和相关开源依赖。
> <img width="1782" height="1161" alt="image" src="https://github.com/user-attachments/assets/f33e531f-581c-49d1-91d4-9164d1be8b28" />
> <img width="1782" height="1161" alt="image" src="https://github.com/user-attachments/assets/648fbdff-c5da-456f-961f-81510bd47806" />

## 当前版本

v0.3.8

## v0.3.8 更新重点

- 建立桌面版与 Android 版的稳定同步流程：桌面仓库作为唯一核心仓库，Android 仓库只消费 `mobilecore` AAR。
- 新增 `mobilecore` 发布元数据和校验文件，包含 AAR SHA256、schema 版本、core commit、构建目标和 Android API。
- Android 导入核心产物时会校验版本、schema 和 hash，避免误导入旧核心或损坏产物。
- 补齐 Android 需要的配置导入、账号数据库导入/导出、仪表盘账号统计等核心 API。
- 修复 Android 壳层多处中文乱码，并补充双仓库维护文档。
- 同步桌面应用、Android 应用、Docker label、前端版本号到 v0.3.8。

## 仓库职责

`yatori-go-desktop` 是唯一核心仓库，负责维护：

- ICVE、学习通等平台接口
- 题库协议和答案解析
- AI 请求
- 答题、保存、提交逻辑
- 任务调度
- 日志格式
- 桌面 GUI
- `mobilecore` AAR、schema 和版本元数据

`yatori-go-android` 只负责 Android 适配：

- WebView/原生桥接
- Android 权限
- 前台服务和通知栏
- 文件导入、导出和分享
- APK 构建与发布

Android 仓库不应复制或单独修改平台业务逻辑。

## Mobilecore 发布

```powershell
cd "D:\AI\智能体工作目录\claude工作目录\学习\yatori-go-desktop-V0.3.4-clean"

$env:PATH='D:\AI\智能体工作目录\claude工作目录\学习\YatoriAndroidWork\tools\bin;C:\msys64\ucrt64\bin;C:\Users\35862\go\bin;' + $env:PATH
$env:CGO_ENABLED='1'

.\scripts\build-mobilecore.ps1 -Version 0.3.8 -Target android/arm64 -ApiSchemaVersion 1
```

产物：

```text
release/
├── yatori-mobile-v0.3.8.aar
├── api-schema.json
├── yatori-core-version.json
└── yatori-mobilecore-checksums.json
```

详细流程见 [docs/mobilecore-release.md](docs/mobilecore-release.md)。

## 从源码构建桌面版

前置要求：Go、Node.js、Windows GCC、Wails v2。

```powershell
cd "D:\AI\智能体工作目录\claude工作目录\学习\yatori-go-desktop-V0.3.4-clean"

$env:PATH='C:\msys64\ucrt64\bin;C:\Users\35862\go\bin;' + $env:PATH
$env:CGO_ENABLED='1'

go test ./service ./mobilecore -count=1
npm --prefix frontend install
npm --prefix frontend run build
wails build -platform windows/amd64 -o yatori-go-desktop.exe
```

## 数据路径

桌面默认数据目录：

```text
%APPDATA%\yatori-go-console\
├── config.yaml
├── yatori.db
└── assets\log\
```

Android 默认数据目录：

```text
/data/user/0/app.yatori.android/files/yatori/
```

## 免责声明

本项目代码已开源，仅供学习与技术交流使用，严禁任何形式的倒卖、贩卖或商业牟利。任何个人或组织使用本项目代码或软件进行违法违规行为，均与项目作者无关，相关责任由使用者自行承担。

## 致谢

- [yatori-dev/yatori-go-console](https://github.com/yatori-dev/yatori-go-console)
- [Wails](https://wails.io/)
- [React](https://react.dev/)

## License

MIT License。原项目版权归原作者所有。
