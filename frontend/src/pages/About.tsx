import { useState } from 'react'
import { OpenDataDir, OpenURL } from '../../wailsjs/go/main/App'
import { APP_VERSION, PROJECT_RELEASES_URL, PROJECT_REPO_URL, SOURCE_REPO_URL } from '../lib/version'
import { checkForUpdates } from '../lib/update'

function btn(label: string, onClick: () => void, disabled = false) {
  return (
    <button
      key={label}
      className="btn btn-primary"
      disabled={disabled}
      onClick={onClick}
      style={{ marginRight: 8, marginBottom: 8 }}
    >
      {label}
    </button>
  )
}

export default function About() {
  const [checking, setChecking] = useState(false)
  const [updateText, setUpdateText] = useState('')
  const [updateUrl, setUpdateUrl] = useState('')
  const open = (url: string) => OpenURL(url)
  const openData = () => OpenDataDir()

  const checkUpdate = async () => {
    setChecking(true)
    setUpdateText('')
    setUpdateUrl('')
    try {
      const info = await checkForUpdates()
      if (info.hasUpdate) {
        setUpdateText(`发现新版本 v${info.latestVersion}，当前版本 v${info.currentVersion}`)
        setUpdateUrl(info.url || PROJECT_RELEASES_URL)
      } else {
        setUpdateText(`当前已是最新版本 v${info.currentVersion}`)
      }
    } catch (err) {
      setUpdateText(`检测失败：${err instanceof Error ? err.message : String(err)}`)
    } finally {
      setChecking(false)
    }
  }

  return (
    <div style={{ padding: '28px 32px', maxWidth: 760, lineHeight: 1.7 }}>
      <h2 style={{ marginTop: 0 }}>Yatori Go Desktop</h2>
      <p style={{ color: 'var(--text2)', marginTop: -8 }}>
        基于 yatori-go-console 改造的 Windows 桌面版工具
      </p>

      <section style={{ marginBottom: 24 }}>
        <h3>项目来源</h3>
        <p>
          本项目基于 <strong>yatori-dev/yatori-go-console</strong> 改造。<br />
          原项目地址：<a href="#" onClick={e => { e.preventDefault(); open(SOURCE_REPO_URL) }}>{SOURCE_REPO_URL}</a><br />
          本项目地址：<a href="#" onClick={e => { e.preventDefault(); open(PROJECT_REPO_URL) }}>{PROJECT_REPO_URL}</a>
        </p>
      </section>

      <section style={{ marginBottom: 24 }}>
        <h3>本项目做了什么</h3>
        <ul style={{ paddingLeft: 20 }}>
          <li>使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面</li>
          <li>复用 yatori-go-console 的核心 Go 逻辑</li>
          <li>增加账号管理、任务控制、日志中心、全局设置和关于本项目页面</li>
          <li>支持 worker 子进程运行任务，并支持硬停止</li>
          <li>支持多套主题并保存到 config.yaml</li>
          <li>增加 GitHub 自动版本检测和手动检测入口</li>
          <li>配置、数据库和日志统一保存到 %APPDATA%\yatori-go-console</li>
        </ul>
      </section>

      <section style={{ marginBottom: 24 }}>
        <h3>安全声明</h3>
        <p>
          本项目仅用于个人已授权账号的学习任务管理。<br />
          不提供验证码破解、人脸绕过、考试作弊等能力。
        </p>
      </section>

      <section style={{ marginBottom: 24 }}>
        <h3>数据目录</h3>
        <code>%APPDATA%\yatori-go-console</code>
      </section>

      <section style={{ marginBottom: 24 }}>
        <h3>版本信息</h3>
        <p>当前版本：<strong>v{APP_VERSION}</strong></p>
        {updateText && <p style={{ color: 'var(--text2)' }}>{updateText}</p>}
        {updateUrl && (
          <button className="btn btn-primary btn-sm" onClick={() => open(updateUrl)}>
            去更新
          </button>
        )}
      </section>

      <section>
        <h3>快速跳转</h3>
        {btn('原项目', () => open(SOURCE_REPO_URL))}
        {btn('本项目仓库', () => open(PROJECT_REPO_URL))}
        {btn('查看更新日志', () => open(PROJECT_RELEASES_URL))}
        {btn(checking ? '检测中...' : '检测新版本', checkUpdate, checking)}
        {btn('打开数据目录', openData)}
      </section>
    </div>
  )
}
