import { useState } from 'react'
import { OpenDataDir, OpenURL } from '../../wailsjs/go/main/App'
import { APP_VERSION, PROJECT_RELEASES_URL, PROJECT_REPO_URL, SOURCE_REPO_URL } from '../lib/version'
import { checkForUpdates } from '../lib/update'

export default function About() {
  const [checking, setChecking] = useState(false)
  const [updateText, setUpdateText] = useState('')
  const [updateUrl, setUpdateUrl] = useState('')
  const open = (url: string) => OpenURL(url)
  const openData = () => OpenDataDir()

  const checkUpdate = async () => {
    setChecking(true); setUpdateText(''); setUpdateUrl('')
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
    <div className="page" style={{ maxWidth: 720 }}>
      <div className="page-title">关于本项目</div>

      <div className="card">
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 4 }}>
          <span style={{ fontSize: 20, fontWeight: 800, letterSpacing: '-0.03em', color: 'var(--accent)' }}>
            Yatori Go Desktop
          </span>
          <span className="badge badge-full">v{APP_VERSION}</span>
        </div>
        <div style={{ fontSize: 13, color: 'var(--text2)', marginBottom: 16 }}>
          基于 yatori-go-console 改造的 Windows 桌面版学习管理工具
        </div>

        {updateText && (
          <div className={updateUrl ? 'alert alert-info' : 'alert alert-warn'} style={{ marginBottom: 12 }}>
            {updateText}
            {updateUrl && (
              <button className="btn btn-primary btn-sm" style={{ marginLeft: 12 }} onClick={() => open(updateUrl)}>
                去更新
              </button>
            )}
          </div>
        )}

        <div className="flex-row" style={{ flexWrap: 'wrap', gap: 8 }}>
          <button className="btn btn-ghost btn-sm" onClick={() => open(SOURCE_REPO_URL)}>原项目</button>
          <button className="btn btn-ghost btn-sm" onClick={() => open(PROJECT_REPO_URL)}>本项目仓库</button>
          <button className="btn btn-ghost btn-sm" onClick={() => open(PROJECT_RELEASES_URL)}>更新日志</button>
          <button className="btn btn-primary btn-sm" onClick={checkUpdate} disabled={checking}>
            {checking ? '检测中…' : '检测新版本'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={openData}>打开数据目录</button>
        </div>
      </div>

      <div className="card">
        <div className="card-title">项目来源</div>
        <div style={{ fontSize: 13, color: 'var(--text2)', lineHeight: 1.7 }}>
          本项目基于 <strong style={{ color: 'var(--text)' }}>yatori-dev/yatori-go-console</strong> 改造。<br />
          原项目：<a href="#" style={{ color: 'var(--accent)' }} onClick={e => { e.preventDefault(); open(SOURCE_REPO_URL) }}>{SOURCE_REPO_URL}</a><br />
          本项目：<a href="#" style={{ color: 'var(--accent)' }} onClick={e => { e.preventDefault(); open(PROJECT_REPO_URL) }}>{PROJECT_REPO_URL}</a>
        </div>
      </div>

      <div className="card">
        <div className="card-title">本项目做了什么</div>
        <div style={{ fontSize: 13, color: 'var(--text2)', lineHeight: 1.8 }}>
          使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面，复用 yatori-go-console 的核心 Go 逻辑。
          增加账号管理、任务控制、日志中心、全局设置和课程进度页面。
          支持 worker 子进程运行任务并支持硬停止，支持多套主题保存到 config.yaml，
          增加 GitHub 自动版本检测。配置、数据库和日志统一保存到
          <code style={{ background: 'var(--bg3)', padding: '1px 6px', borderRadius: 4, margin: '0 4px', fontSize: 12 }}>
            %APPDATA%\yatori-go-console
          </code>
        </div>
      </div>

      <div className="card">
        <div className="card-title">安全声明</div>
        <div style={{ fontSize: 13, color: 'var(--text2)', lineHeight: 1.7 }}>
          本项目仅用于个人已授权账号的学习任务管理。<br />
          不提供验证码破解、人脸绕过、考试作弊等能力。
        </div>
      </div>
    </div>
  )
}
