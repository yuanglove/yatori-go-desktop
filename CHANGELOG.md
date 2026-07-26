# 更新日志

## v0.4.0

- 同步上游 yatori-go-core 对应依赖版本并重新构建桌面端与 Android 移动核心。
- 统一桌面端、移动核心、前端、Docker 标签和发布脚本版本号为 v0.4.0。

## v0.3.9

- 建立桌面版与 Android 版的稳定核心同步流程。
- `mobilecore` 发布产物新增 `yatori-mobilecore-checksums.json`，并在 `yatori-core-version.json` 中记录 AAR SHA256、字节数、构建 target、Android API 和 core commit。
- Android 导入桌面核心时校验版本、schema、AAR 文件名和 SHA256，防止导入错误核心。
- 新增 Android 需要的 `mobilecore` API：配置文本导入、账号数据库导入、账号数据库导出。
- 修复 Android 仪表盘账号数为 0 的问题，改为读取真实账号列表统计。
- 修复 Android 导入账号功能只返回“不支持”的问题。
- 修复 Android 壳层多处中文乱码。
- 补充双仓库维护文档：桌面仓库维护核心，Android 仓库只做移动端适配和 APK 发布。
- 同步桌面端、Android 端、Docker 和前端版本号到 v0.3.9。

## v0.3.7

- 更新桌面、任务栏和侧边栏品牌图标，统一使用新版主题图片。
- 完善智慧职教（ICVE）测验/作业识别、题目解析、题库/AI 答题、保存/提交和诊断日志流程。
- 优化日志显示，修复部分 worker 输出乱码和历史日志展示问题。
- 优化 Docker 打包忽略规则，避免本地缓存、临时文件和发布产物进入镜像。
- 同步 Windows 安装包、Docker 镜像包和 GitHub Release 到 v0.3.7。

## v0.3.5

- 默认主题升级为灰白黑半透明磨砂风格。
- 优化设置页题库配置布局。
- 新增通用第三方题库适配能力。
- 支持粘贴 OCS 题库配置并自动解析。
- README 更新为中文说明。

## v0.3.4

- 课程进度页新增智慧职教（ICVE）支持。
- ICVE 课程进度按任务点完成状态统计。

## v0.3.3

- 智慧职教（ICVE）新增自动获取 Cookie 流程。
- 新增 Wails 后端接口 `StartICVECookieCapture` / `ReadICVECookie`。

## v0.3.2

- 智慧职教（ICVE）改为仅支持 Cookie 登录。
- 启动 ICVE 时跳过 OCR/Core Runtime 初始化。

## v0.3.1

- 优化账号管理中的视频模式显示。
- 修复 WeLearn 学时模式和课程进度刷新问题。
- 修复课程进度页和账号管理页历史编辑残片导致的构建问题。

## v0.3.0

- 重写 WeLearn 桌面安全 runner。
- 优化 WeLearn 课程进度、学时解析和错误处理。

