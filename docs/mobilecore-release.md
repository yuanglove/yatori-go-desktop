# Mobilecore 发布流程

`yatori-go-desktop` 是桌面版和 Android 版共用的唯一核心仓库。Android 仓库不维护平台业务逻辑，只消费本仓库生成的 `mobilecore` AAR。

## 仓库职责

本仓库维护：

- 平台接口：ICVE、学习通等
- 题库协议和答案解析
- AI 请求
- 答题、保存、提交逻辑
- 任务调度
- 日志格式
- `mobilecore` API、schema、AAR

Android 仓库维护：

- Android WebView / 原生桥接
- 移动端 UI 和权限
- 前台服务、通知栏
- APK 构建和发布
- 导入本仓库发布的 AAR

## API 约定

`mobilecore` 所有对外接口必须返回统一 JSON：

```json
{
  "ok": true,
  "data": {},
  "error": "",
  "code": ""
}
```

失败时：

```json
{
  "ok": false,
  "data": null,
  "error": "错误信息",
  "code": "ERROR_CODE"
}
```

新增、删除或修改 `mobilecore` API 时，必须同步更新：

- `mobilecore/api-schema.json`
- `mobilecore` 测试
- Android 仓库的桥接调用（如需要）

## 构建产物

运行：

```powershell
.\scripts\build-mobilecore.ps1 -Version 0.3.9 -Target android/arm64 -ApiSchemaVersion 1
```

会生成：

```text
release/
├── yatori-mobile-v0.3.9.aar
├── api-schema.json
├── yatori-core-version.json
└── yatori-mobilecore-checksums.json
```

`yatori-core-version.json` 记录 Android 版本占位、桌面核心版本、core commit、schema version、AAR 文件名、AAR SHA256、AAR 字节数、gomobile target、Android API 和构建时间。

`yatori-mobilecore-checksums.json` 记录 AAR、schema、version 文件的 SHA256，用于 Android 导入校验。

## 推荐发布顺序

1. 修改桌面核心逻辑。
2. 更新 `mobilecore/api-schema.json`。
3. 运行 `go test ./mobilecore ./service -count=1`。
4. 运行 `scripts/build-mobilecore.ps1`。
5. 将 `release/` 下四个文件上传到桌面版 Release。
6. Android 仓库导入这四个产物并构建 APK。

## 体积说明

当前 Android AAR 使用 `android/arm64`，只包含 `arm64-v8a` 原生库。不要默认使用 universal target，否则 APK 会包含多套 `libgojni.so`。

继续大幅减小 APK 体积需要做移动端裁剪构建，例如用 build tags 排除 Android 不需要的平台、OCR/ONNX 或桌面专用依赖。

