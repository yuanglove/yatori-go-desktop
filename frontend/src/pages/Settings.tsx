import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { AppConfig } from '../lib/api'
import { FormGroup, Section, Spinner } from '../components/shared'
import { applyTheme, THEMES } from '../lib/theme'

const AI_TYPES = ['TONGYI', 'SILICON', 'DOUBAO', 'CHATGLM', 'XINGHUO', 'OPENAI', 'DEEPSEEK', 'METAAI', 'OTHER']

export default function SettingsPage() {
  const [cfg, setCfg] = useState<AppConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [msgText, setMsgText] = useState('')
  const [msgOk, setMsgOk] = useState(true)
  const [showApiKey, setShowApiKey] = useState(false)
  const [showEmailPwd, setShowEmailPwd] = useState(false)
  const [testMsg, setTestMsg] = useState('')
  const [testOk, setTestOk] = useState(true)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    api.getConfig()
      .then(r => {
        if (r.ok) {
          const plain = JSON.parse(JSON.stringify(r.data)) as AppConfig
          setCfg(plain)
          applyTheme(plain.setting.basicSetting.theme || 'dark')
        }
      })
      .finally(() => setLoading(false))
  }, [])

  if (loading || !cfg) return <div className="page"><Spinner /></div>

  const setBasic = (k: keyof typeof cfg.setting.basicSetting, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, basicSetting: { ...cfg.setting.basicSetting, [k]: v } } })
  const setEmail = (k: keyof typeof cfg.setting.emailInform, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, emailInform: { ...cfg.setting.emailInform, [k]: v } } })
  const setAI = (k: keyof typeof cfg.setting.aiSetting, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, aiSetting: { ...cfg.setting.aiSetting, [k]: v } } })

  const setTheme = (theme: string) => {
    applyTheme(theme)
    setBasic('theme', theme)
  }

  const save = async () => {
    setSaving(true)
    setMsgText('')
    const r = await api.saveConfig(cfg)
    setSaving(false)
    setMsgOk(r.ok)
    setMsgText(r.ok ? '保存成功' : r.error ?? '保存失败')
    if (r.ok) applyTheme(cfg.setting.basicSetting.theme || 'dark')
  }

  const importCfg = async () => {
    const r = await api.importConfig()
    if (r.ok) {
      const plain = JSON.parse(JSON.stringify(r.data)) as AppConfig
      setCfg(plain)
      setMsgOk(true)
      setMsgText('导入成功')
      applyTheme(plain.setting.basicSetting.theme || 'dark')
    } else if (r.error !== '用户取消') {
      setMsgOk(false)
      setMsgText(r.error ?? '导入失败')
    }
  }

  const exportCfg = async () => {
    const r = await api.exportConfig(cfg)
    if (!r.ok && r.error !== '用户取消') {
      setMsgOk(false)
      setMsgText(r.error ?? '导出失败')
    }
  }

  const testAI = async () => {
    setTesting(true)
    setTestMsg('')
    const r = await api.testAIConfig()
    setTesting(false)
    setTestOk(r.ok)
    setTestMsg(r.ok ? String(r.data) : r.error ?? '测试失败')
  }

  const apiKeyLooksLikeModel = cfg.setting.aiSetting.apiKey.includes('/')
  const siliconUrlLooksWrong =
    cfg.setting.aiSetting.aiType === 'SILICON' &&
    cfg.setting.aiSetting.aiUrl.includes('cloud.siliconflow')

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 16 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>全局设置</div>
        <div className="flex-row">
          {msgText && (
            <span style={{ fontSize: 13, color: msgOk ? 'var(--success)' : 'var(--danger)' }}>
              {msgOk ? '通过 ' : '失败 '}{msgText}
            </span>
          )}
          <button className="btn btn-ghost btn-sm" onClick={importCfg}>导入 config.yaml</button>
          <button className="btn btn-ghost btn-sm" onClick={exportCfg}>导出 config.yaml</button>
          <button className="btn btn-ghost btn-sm" onClick={() => api.openDataDir()}>打开数据目录</button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>{saving ? '保存中...' : '保存'}</button>
        </div>
      </div>

      <Section title="基础设置">
        <div className="form-row form-row-2">
          <FormGroup label="界面主题">
            <select className="form-select" value={cfg.setting.basicSetting.theme || 'dark'} onChange={e => setTheme(e.target.value)}>
              {THEMES.map(theme => <option key={theme.value} value={theme.value}>{theme.label}</option>)}
            </select>
          </FormGroup>
          <FormGroup label="完成提示音">
            <select className="form-select" value={cfg.setting.basicSetting.completionTone} onChange={e => setBasic('completionTone', Number(e.target.value))}>
              <option value={0}>关闭</option>
              <option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="彩色日志">
            <select className="form-select" value={cfg.setting.basicSetting.colorLog} onChange={e => setBasic('colorLog', Number(e.target.value))}>
              <option value={0}>关闭</option>
              <option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="输出日志文件">
            <select className="form-select" value={cfg.setting.basicSetting.logOutFileSw} onChange={e => setBasic('logOutFileSw', Number(e.target.value))}>
              <option value={0}>关闭</option>
              <option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="日志等级">
            <select className="form-select" value={cfg.setting.basicSetting.logLevel} onChange={e => setBasic('logLevel', e.target.value)}>
              {['INFO', 'DEBUG', 'WARN', 'ERROR'].map(level => <option key={level}>{level}</option>)}
            </select>
          </FormGroup>
          <FormGroup label="日志模式">
            <select className="form-select" value={cfg.setting.basicSetting.logModel} onChange={e => setBasic('logModel', Number(e.target.value))}>
              <option value={0}>以视频提交为基准</option>
              <option value={1}>以课程为基准</option>
            </select>
          </FormGroup>
        </div>
      </Section>

      <Section title="任务并发">
        <div className="alert alert-info" style={{ marginBottom: 12 }}>
          控制桌面端最多同时运行几个账号任务。超过上限时，新任务会被拒绝启动；停止或任务结束后会释放名额。
        </div>
        <FormGroup label="最大同时运行任务数 (1-10)">
          <input
            className="form-input"
            type="number"
            min={1}
            max={10}
            step={1}
            value={cfg.setting.basicSetting.maxWorkers ?? 3}
            onChange={e => {
              const next = Number(e.target.value || 3)
              setBasic('maxWorkers', Math.min(10, Math.max(1, next)))
            }}
          />
        </FormGroup>
      </Section>

      <Section title="AI 设置">
        {testMsg && <div className={testOk ? 'alert alert-info' : 'alert alert-warn'}>{testOk ? '通过 ' : '失败 '}{testMsg}</div>}
        <div className="form-row form-row-2">
          <FormGroup label="AI 类型">
            <select className="form-select" value={cfg.setting.aiSetting.aiType} onChange={e => setAI('aiType', e.target.value)}>
              {AI_TYPES.map(type => <option key={type}>{type}</option>)}
            </select>
          </FormGroup>
          <FormGroup label="模型名">
            <input className="form-input" value={cfg.setting.aiSetting.model} placeholder="例如 Qwen/Qwen3-32B" onChange={e => setAI('model', e.target.value)} />
          </FormGroup>
          <FormGroup label="API 地址">
            <input className="form-input" value={cfg.setting.aiSetting.aiUrl} onChange={e => setAI('aiUrl', e.target.value)} />
            {siliconUrlLooksWrong && (
              <div className="alert alert-warn" style={{ marginTop: 6 }}>
                cloud.siliconflow.cn 是控制台地址，不是 API 地址。SILICON 可留空，或填写 https://api.siliconflow.cn/v1/chat/completions
              </div>
            )}
          </FormGroup>
          <FormGroup label="API Key">
            <div className="flex-row">
              <input className="form-input" type={showApiKey ? 'text' : 'password'} value={cfg.setting.aiSetting.apiKey} onChange={e => setAI('apiKey', e.target.value)} />
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }} onClick={() => setShowApiKey(v => !v)}>
                {showApiKey ? '隐藏' : '显示'}
              </button>
            </div>
            {apiKeyLooksLikeModel && (
              <div className="alert alert-warn" style={{ marginTop: 6 }}>API Key 看起来像模型名。模型名填在“模型名”，这里填写平台发放的 API Key。</div>
            )}
          </FormGroup>
        </div>
        <div style={{ marginTop: 8 }}>
          <button className="btn btn-ghost btn-sm" onClick={testAI} disabled={testing}>{testing ? '测试中...' : '测试 AI 配置'}</button>
        </div>
      </Section>

      <Section title="外部题库接口">
        <FormGroup label="题库接口 URL">
          <input className="form-input" value={cfg.setting.apiQueSetting.url} onChange={e => setCfg({ ...cfg, setting: { ...cfg.setting, apiQueSetting: { url: e.target.value } } })} />
        </FormGroup>
      </Section>

      <Section title="邮件通知">
        <div className="form-row form-row-2">
          <FormGroup label="开关">
            <select className="form-select" value={cfg.setting.emailInform.sw} onChange={e => setEmail('sw', Number(e.target.value))}>
              <option value={0}>关闭</option>
              <option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="SMTP Host">
            <input className="form-input" value={cfg.setting.emailInform.smtpHost} onChange={e => setEmail('smtpHost', e.target.value)} />
          </FormGroup>
          <FormGroup label="SMTP Port">
            <input className="form-input" type="number" value={cfg.setting.emailInform.smtpPort || ''} onChange={e => setEmail('smtpPort', Number(e.target.value))} />
          </FormGroup>
          <FormGroup label="用户名">
            <input className="form-input" value={cfg.setting.emailInform.userName} onChange={e => setEmail('userName', e.target.value)} />
          </FormGroup>
          <FormGroup label="邮箱密码">
            <div className="flex-row">
              <input className="form-input" type={showEmailPwd ? 'text' : 'password'} value={cfg.setting.emailInform.password} onChange={e => setEmail('password', e.target.value)} />
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }} onClick={() => setShowEmailPwd(v => !v)}>{showEmailPwd ? '隐藏' : '显示'}</button>
            </div>
          </FormGroup>
        </div>
      </Section>
    </div>
  )
}
