import { APP_VERSION, SOURCE_REPO_URL, PROJECT_REPO_URL, PROJECT_RELEASES_URL } from '../lib/version'
import { OpenURL, OpenDataDir } from '../../wailsjs/go/main/App'

function btn(label: string, onClick: () => void) {
  return (
    <button key={label} onClick={onClick} style={{
      padding: '6px 16px', marginRight: 8, marginBottom: 8,
      background: 'var(--accent)', color: '#fff', border: 'none',
      borderRadius: 4, cursor: 'pointer', fontSize: 13,
    }}>
      {label}
    </button>
  )
}

export default function About() {
  const open = (url: string) => OpenURL(url)
  const openData = () => OpenDataDir()

  return (
    <div style={{ padding: '28px 32px', maxWidth: 720, lineHeight: 1.7 }}>
      <h2 style={{ marginTop: 0 }}>Yatori Go Desktop</h2>
      <p style={{ color: 'var(--text2)', marginTop: -8 }}>
        基于 yatori-go-console 改造的 Windows 桌面版工具
      </p>

      <section style={{ marginBottom: 24 }}>
        <h3>项目来源</h3>
        <p>
          本项目基于 <strong>yatori-dev/yatori-go-console</strong> 改造。<br />
          原项目地址：<a href="#" onClick={e => { e.preventDefault(); open(SOURCE_REPO_URL) }}>{SOURCE_REPO_URL}</a>
        </p>
      </section>

      <section style={{ marginBottom: 24 }}>
        <h3>本项目做了什么</h3>
        <ul style={{ paddingLeft: 20 }}>
          <li>使用 Wails v2 + React + TypeScript 构建 Windows 桌面界面</li>
          <li>保留并复用 yatori-go-console 的核心 Go 逻辑</li>
          <li>增加账号管理页面</li>
          <li>增加任务控制页面</li>
          <li>支持学习通任务启动和硬停止</li>
          <li>增加日志中心</li>
          <li>统一日志格式：[时间] [平台][账号] 【课程】【当前任务点】【资源/章节测试标题】消息</li>
          <li>增加全局设置页面</li>
          <li>支持学习通多课程 / 多任务点模式配置</li>
          <li>支持章节测试 AI 答题配置</li>
          <li>配置、数据库、日志统一保存到 %APPDATA%\yatori-go-console</li>
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
      </section>

      <section>
        <h3>快速跳转</h3>
        {btn('原项目', () => open(SOURCE_REPO_URL))}
        {btn('本项目仓库', () => open(PROJECT_REPO_URL))}
        {btn('查看更新日志', () => open(PROJECT_RELEASES_URL))}
        {btn('打开数据目录', openData)}
      </section>
    </div>
  )
}
