import { useState } from 'react'
import {
  ExternalLink, FolderOpen, RefreshCw, ArrowUpCircle,
  GitBranch, Shield
} from 'lucide-react'
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
        setUpdateText('发现新版本 v' + info.latestVersion + '（当前 v' + info.currentVersion + '）')
        setUpdateUrl(info.url || PROJECT_RELEASES_URL)
      } else {
        setUpdateText('当前已是最新版本 v' + info.currentVersion)
      }
    } catch (err) {
      setUpdateText('检测失败：' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setChecking(false)
    }
  }

  const shortRepo = PROJECT_REPO_URL.replace('https://github.com/', '')

  return (
    <div className="page about-page">

      <div className="about-header card">
        <div className="about-header-top">
          <span className="about-project-name">{'Yatori Go Desktop'}</span>
          <span className="badge badge-running about-version-badge">{'v' + APP_VERSION}</span>
        </div>
        <p className="about-desc">
          {'基于 yatori-go-console 改造的 Windows 桌面版学习管理工具'}
        </p>
        <div className="about-actions">
          <button className="btn btn-ghost btn-sm" onClick={() => open(SOURCE_REPO_URL)}>
            <GitBranch size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {'原项目'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => open(PROJECT_REPO_URL)}>
            <GitBranch size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {'本项目仓库'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => open(PROJECT_RELEASES_URL)}>
            <ExternalLink size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {'更新日志'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={openData}>
            <FolderOpen size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {'打开数据目录'}
          </button>
        </div>
      </div>

      <div className="card about-version-card">
        <div className="about-version-row">
          <div>
            <div className="about-section-label">{'当前版本'}</div>
            <div className="about-version-num">{'v' + APP_VERSION}</div>
          </div>
          <div className="about-version-actions">
            <button className="btn btn-primary btn-sm" onClick={checkUpdate} disabled={checking}>
              <RefreshCw size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
              {checking ? '检测中…' : '检测新版本'}
            </button>
            {updateUrl && (
              <button className="btn btn-primary btn-sm" onClick={() => open(updateUrl)}>
                <ArrowUpCircle size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
                {'去更新'}
              </button>
            )}
          </div>
        </div>
        {updateText && (
          <div className={updateUrl ? 'alert alert-info' : 'alert alert-warn'} style={{ marginTop: 10 }}>
            {updateText}
          </div>
        )}
      </div>

      <div className="card">
        <div className="card-title">{'本项目做了什么'}</div>
        <p className="about-text-block">
          {'使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面，复用 yatori-go-console 的核心 Go 逻辑。增加账号管理、任务控制、日志中心、全局设置和课程进度页面。支持 worker 子进程运行任务并支持硬停止，支持多套主题保存到 config.yaml，增加 GitHub 自动版本检测。配置、数据库和日志统一保存到 '}
          <code className="about-inline-path">{'%APPDATA%\\yatori-go-console'}</code>
          {'。'}
        </p>
      </div>


      <div className="card">
        <div className="card-title">{'项目来源'}</div>
        <div className="about-source-list">
          <div className="about-source-item">
            <span className="about-source-label">{'原项目'}</span>
            <a href="#" className="about-source-link" onClick={e => { e.preventDefault(); open(SOURCE_REPO_URL) }}>
              {'yatori-dev/yatori-go-console'}
              <ExternalLink size={12} strokeWidth={2} style={{ marginLeft: 4, verticalAlign: 'middle', opacity: 0.7 }} />
            </a>
          </div>
          <div className="about-source-item">
            <span className="about-source-label">{'本项目'}</span>
            <a href="#" className="about-source-link" onClick={e => { e.preventDefault(); open(PROJECT_REPO_URL) }}>
              {shortRepo}
              <ExternalLink size={12} strokeWidth={2} style={{ marginLeft: 4, verticalAlign: 'middle', opacity: 0.7 }} />
            </a>
          </div>
        </div>
      </div>

      <div className="card about-data-card">
        <div className="card-title" style={{ marginBottom: 8 }}>{'数据目录'}</div>
        <div className="about-data-row">
          <code className="about-path-chip">{'%APPDATA%\\yatori-go-console'}</code>
          <button className="btn btn-ghost btn-sm" onClick={openData}>
            <FolderOpen size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            {'打开'}
          </button>
        </div>
        <div className="text-muted text-sm" style={{ marginTop: 6 }}>
          {'配置文件、数据库和日志均保存于此目录'}
        </div>
      </div>

      <div className="alert alert-info about-security">
        <Shield size={14} strokeWidth={2} style={{ marginRight: 6, verticalAlign: 'middle', flexShrink: 0 }} />
        <span>{'本项目仅用于个人已授权账号的学习任务管理，不提供验证码破解、人脸绕过或考试作弊能力。'}</span>
      </div>

    </div>
  )
}
