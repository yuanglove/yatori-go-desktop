import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { AppConfig } from '../lib/api'
import { Section, FormGroup, Spinner } from '../components/shared'

const AI_TYPES = ['TONGYI', 'SILICON', 'DOUBAO', 'CHATGLM', 'XINGHUO', 'OPENAI', 'DEEPSEEK', 'METAAI', 'OTHER']

export default function SettingsPage() {
  // All hooks must be at the top, before any conditional return
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
    api.getConfig().then(r => {
      if (r.ok) setCfg(JSON.parse(JSON.stringify(r.data)))
    }).finally(() => setLoading(false))
  }, [])

  if (loading || !cfg) return <div className="page"><Spinner /></div>

  const setB = (k: keyof typeof cfg.setting.basicSetting, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, basicSetting: { ...cfg.setting.basicSetting, [k]: v } } })
  const setE = (k: keyof typeof cfg.setting.emailInform, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, emailInform: { ...cfg.setting.emailInform, [k]: v } } })
  const setAI = (k: keyof typeof cfg.setting.aiSetting, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, aiSetting: { ...cfg.setting.aiSetting, [k]: v } } })

  const save = async () => {
    setSaving(true); setMsgText('')
    const r = await api.saveConfig(cfg)
    setSaving(false)
    setMsgOk(r.ok)
    setMsgText(r.ok ? '保存成功' : r.error ?? '保存失败')
  }

  const importCfg = async () => {
    const r = await api.importConfig()
    if (r.ok) { setCfg(r.data); setMsgOk(true); setMsgText('导入成功') }
    else if (r.error !== '用户取消') { setMsgOk(false); setMsgText(r.error ?? '导入失败') }
  }

  const exportCfg = async () => {
    const r = await api.exportConfig(cfg)
    if (!r.ok && r.error !== '用户取消') { setMsgOk(false); setMsgText(r.error ?? '导出失败') }
  }

  const testAI = async () => {
    setTesting(true); setTestMsg('')
    const r = await api.testAIConfig()
    setTesting(false)
    setTestOk(r.ok)
    setTestMsg(r.ok ? String(r.data) : r.error ?? '测试失败')
  }

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 16 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>全局设置</div>
        <div className="flex-row">
          {msgText && <span style={{ fontSize: 13, color: msgOk ? 'var(--success)' : 'var(--danger)' }}>
            {msgOk ? '✓ ' : '✗ '}{msgText}
          </span>}
          <button className="btn btn-ghost btn-sm" onClick={importCfg}>导入 config.yaml</button>
          <button className="btn btn-ghost btn-sm" onClick={exportCfg}>导出 config.yaml</button>
          <button className="btn btn-ghost btn-sm" onClick={() => api.openDataDir()}>打开数据目录</button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>{saving ? '保存中…' : '保存'}</button>
        </div>
      </div>

      <Section title="基础设置">
        <div className="form-row form-row-3">
          <FormGroup label="完成提示音">
            <select className="form-select" value={cfg.setting.basicSetting.completionTone}
              onChange={e => setB('completionTone', Number(e.target.value))}>
              <option value={0}>关闭</option><option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="彩色日志（仅影响文件/控制台，桌面日志页自动清理颜色码）">
            <select className="form-select" value={cfg.setting.basicSetting.colorLog}
              onChange={e => setB('colorLog', Number(e.target.value))}>
              <option value={0}>关闭</option><option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="输出日志文件">
            <select className="form-select" value={cfg.setting.basicSetting.logOutFileSw}
              onChange={e => setB('logOutFileSw', Number(e.target.value))}>
              <option value={0}>关闭</option><option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="日志等级">
            <select className="form-select" value={cfg.setting.basicSetting.logLevel}
              onChange={e => setB('logLevel', e.target.value)}>
              {['INFO', 'DEBUG', 'WARN', 'ERROR'].map(l => <option key={l}>{l}</option>)}
            </select>
          </FormGroup>
          <FormGroup label="日志模式">
            <select className="form-select" value={cfg.setting.basicSetting.logModel}
              onChange={e => setB('logModel', Number(e.target.value))}>
              <option value={0}>以视频提交为基准</option>
              <option value={1}>以课程为基准</option>
            </select>
          </FormGroup>
        </div>
      </Section>

      <Section title="AI 设置">
        {testMsg && (
          <div className="alert" style={{marginBottom:8,
            background: testOk ? '#1a2a1a' : '#2a1a1a',
            border: '1px solid',
            borderColor: testOk ? 'var(--success)' : 'var(--danger)',
            color: testOk ? 'var(--success)' : 'var(--danger)'}}>
            {testOk ? '✓ ' : '✗ '}{testMsg}
          </div>
        )}
        <div className="form-row form-row-2">
          <FormGroup label="AI 类型">
            <select className="form-select" value={cfg.setting.aiSetting.aiType}
              onChange={e => setAI('aiType', e.target.value)}>
              {AI_TYPES.map(t => <option key={t}>{t}</option>)}
            </select>
          </FormGroup>
          <FormGroup label="模型名（如 Qwen/Qwen3-32B）">
            <input className="form-input" value={cfg.setting.aiSetting.model}
              placeholder="留空使用默认；豆包必填接入点 ID"
              onChange={e => setAI('model', e.target.value)} />
          </FormGroup>
          <FormGroup label="API 地址（OTHER 必填；SILICON 可留空）">
            <input className="form-input" value={cfg.setting.aiSetting.aiUrl}
              onChange={e => setAI('aiUrl', e.target.value)} />
            {cfg.setting.aiSetting.aiType === 'SILICON' && cfg.setting.aiSetting.aiUrl.includes('cloud.siliconflow') && (
              <div className="alert alert-warn" style={{marginTop:4}}>
                cloud.siliconflow.cn 是控制台地址，不是 API 地址。SILICON 留空即可，或填 https://api.siliconflow.cn/v1/chat/completions
              </div>
            )}
          </FormGroup>
          <FormGroup label="API Key（平台颁发，如 sk-xxx）">
            <div className="flex-row">
              <input className="form-input" type={showApiKey ? 'text' : 'password'}
                value={cfg.setting.aiSetting.apiKey}
                onChange={e => setAI('apiKey', e.target.value)} />
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }}
                onClick={() => setShowApiKey(p => !p)}>{showApiKey ? '隐藏' : '显示'}</button>
            </div>
            {cfg.setting.aiSetting.apiKey.includes('/') && (
              <div className="alert alert-warn" style={{marginTop:4}}>
                API Key 疑似填成了模型名（含"/"）。模型名填上方"模型名"字段，这里填 API Key（如 sk-xxx）。
              </div>
            )}
          </FormGroup>
        </div>
        <div style={{marginTop:8}}>
          <button className="btn btn-ghost btn-sm" onClick={testAI} disabled={testing}>
            {testing ? '测试中…' : '🔍 测试 AI 配置'}
          </button>
        </div>
      </Section>

      <Section title="外部题库接口">
        <FormGroup label="题库接口 URL">
          <input className="form-input" value={cfg.setting.apiQueSetting.url}
            onChange={e => setCfg({ ...cfg, setting: { ...cfg.setting, apiQueSetting: { url: e.target.value } } })} />
        </FormGroup>
      </Section>

      <Section title="邮箱通知">
        <div className="form-row form-row-2">
          <FormGroup label="开关">
            <select className="form-select" value={cfg.setting.emailInform.sw}
              onChange={e => setE('sw', Number(e.target.value))}>
              <option value={0}>关闭</option><option value={1}>开启</option>
            </select>
          </FormGroup>
          <FormGroup label="SMTP Host">
            <input className="form-input" value={cfg.setting.emailInform.smtpHost}
              onChange={e => setE('smtpHost', e.target.value)} />
          </FormGroup>
          <FormGroup label="SMTP Port">
            <input className="form-input" type="number" value={cfg.setting.emailInform.smtpPort || ''}
              onChange={e => setE('smtpPort', Number(e.target.value))} />
          </FormGroup>
          <FormGroup label="用户名">
            <input className="form-input" value={cfg.setting.emailInform.userName}
              onChange={e => setE('userName', e.target.value)} />
          </FormGroup>
          <FormGroup label="邮箱密码">
            <div className="flex-row">
              <input className="form-input" type={showEmailPwd ? 'text' : 'password'}
                value={cfg.setting.emailInform.password}
                onChange={e => setE('password', e.target.value)} />
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }}
                onClick={() => setShowEmailPwd(p => !p)}>{showEmailPwd ? '隐藏' : '显示'}</button>
            </div>
          </FormGroup>
        </div>
      </Section>
    </div>
  )
}
