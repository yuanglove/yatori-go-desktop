import { useEffect, useState } from 'react'
import { Eye, EyeOff, Upload, Download, FolderOpen, Save, Zap } from 'lucide-react'
import { api } from '../lib/api'
import type { AppConfig } from '../lib/api'
import { FormGroup, Section, Spinner } from '../components/shared'
import { applyTheme, THEMES } from '../lib/theme'
import AnimatedSelect from '../components/AnimatedSelect'

const AI_PRESETS = [
  { value: 'TONGYI', label: '通义千问（TONGYI）', url: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions' },
  { value: 'SILICON', label: '硅基流动（SILICON）', url: 'https://api.siliconflow.cn/v1/chat/completions' },
  { value: 'DOUBAO', label: '豆包（DOUBAO）', url: 'https://ark.cn-beijing.volces.com/api/v3/chat/completions' },
  { value: 'CHATGLM', label: '智谱清言（CHATGLM）', url: 'https://open.bigmodel.cn/api/paas/v4/chat/completions' },
  { value: 'XINGHUO', label: '讯飞星火（XINGHUO）', url: 'https://spark-api-open.xf-yun.com/v1/chat/completions' },
  { value: 'OPENAI', label: 'OpenAI（OPENAI）', url: 'https://api.openai.com/v1/chat/completions' },
  { value: 'DEEPSEEK', label: 'DeepSeek（DEEPSEEK）', url: 'https://api.deepseek.com/v1/chat/completions' },
  { value: 'METAAI', label: '秘塔 AI（METAAI）', url: 'https://metaso.cn/api/openai/v1/chat/completions' },
  { value: 'OTHER', label: '其他（OTHER）', url: '' },
]

const AI_PRESET_URLS = AI_PRESETS.map(p => p.url).filter(Boolean)

function isPresetAIUrl(url: string) {
  const value = url.trim()
  return value === '' || AI_PRESET_URLS.includes(value)
}

const QUESTION_BANK_PRESETS = [
  { value: 'yatori', label: 'Yatori 原协议（Yatori）', url: 'http://localhost:8083', method: 'POST', contentType: 'json', tokenParam: 'token', authType: 'none', questionField: 'content', typeField: 'type', optionsField: 'options', courseNameField: 'courseName', optionsFormat: 'array', typeMap: '', answerPath: 'question.answers', answerSplit: '#' },
  { value: 'wanjuan', label: '万卷题库（AXE）', url: 'https://tk.wanjuantiku.com/api/query', method: 'POST', contentType: 'form', tokenParam: 'token', authType: 'form', questionField: 'tm', typeField: 'type', optionsField: 'options', courseNameField: 'coursename', optionsFormat: 'text', typeMap: 'single=single,multiple=multiple,judge=judge,fill=completion,short=short', answerPath: 'data.questions.0.answer', answerSplit: '#' },
  { value: 'zerror', label: 'ZE 题库（ZError）', url: 'https://api.zaizhexue.top/api/query', method: 'GET', contentType: 'json', tokenParam: 'token', authType: 'query', questionField: 'title', typeField: 'type', optionsField: 'options', courseNameField: 'courseName', optionsFormat: 'text', typeMap: '', answerPath: 'data.data', answerSplit: '#' },
  { value: 'xhwlgzs', label: '小杭题库（XHWL）', url: 'https://api.tiku.xhwlgzs.cn/v1/questions/search', method: 'POST', contentType: 'json', tokenParam: 'Authorization', authType: 'bearer', questionField: 'value', typeField: 'type', optionsField: 'options', courseNameField: '', optionsFormat: 'object', typeMap: 'single=0,multiple=1,fill=2,judge=3,short=4', answerPath: 'data.answer', answerSplit: '#' },
  { value: 'ocs', label: 'OCS 兼容接口（OCS Compatible）', url: '', method: 'POST', contentType: 'json', tokenParam: 'token', authType: 'query', questionField: 'title', typeField: 'type', optionsField: 'options', courseNameField: 'courseName', optionsFormat: 'array', typeMap: '', answerPath: 'data.answer', answerSplit: '#' },
  { value: 'custom', label: '自定义接口（Custom）', url: '', method: 'POST', contentType: 'json', tokenParam: 'token', authType: 'none', questionField: 'title', typeField: 'type', optionsField: 'options', courseNameField: 'courseName', optionsFormat: 'array', typeMap: '', answerPath: 'answers', answerSplit: '#' },
]
const QUESTION_BANK_PRESET_URLS = QUESTION_BANK_PRESETS.map(p => p.url).filter(Boolean)

function isPresetQuestionBankUrl(url: string) {
  const value = url.trim()
  return value === '' || QUESTION_BANK_PRESET_URLS.includes(value)
}

function presetForQuestionBankUrl(raw: string) {
  try {
    const host = new URL(raw.trim()).host.toLowerCase()
    if (host === 'api.tiku.xhwlgzs.cn') return QUESTION_BANK_PRESETS.find(p => p.value === 'xhwlgzs')
    if (host === 'tk.wanjuantiku.com') return QUESTION_BANK_PRESETS.find(p => p.value === 'wanjuan')
    if (host === 'api.zaizhexue.top') return QUESTION_BANK_PRESETS.find(p => p.value === 'zerror')
  } catch {
    return undefined
  }
  return undefined
}

type OCSAnswererConfig = {
  name?: string
  url?: string
  method?: string
  contentType?: string
  data?: Record<string, unknown>
  headers?: Record<string, string>
  handler?: string
}

function findOCSPlaceholder(data: Record<string, unknown> | undefined, placeholder: string) {
  if (!data) return ''
  for (const [key, value] of Object.entries(data)) {
    if (String(value).includes('${' + placeholder + '}')) return key
  }
  return ''
}

function inferAnswerPathFromHandler(handler?: string) {
  if (!handler) return 'data'
  if (handler.includes('res.data.data')) return 'data'
  if (handler.includes('res.data.answer')) return 'answer'
  if (handler.includes('res.data.answers')) return 'answers'
  if (handler.includes('res.data.msg')) return 'msg'
  return 'data'
}

function parseOCSAnswererConfig(text: string): OCSAnswererConfig {
  const parsed = JSON.parse(text.trim())
  const item = Array.isArray(parsed) ? parsed[0] : parsed
  if (!item || typeof item !== 'object') throw new Error('OCS 配置必须是对象或对象数组')
  return item as OCSAnswererConfig
}

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
  const [qbTestMsg, setQbTestMsg] = useState('')
  const [qbTestOk, setQbTestOk] = useState(true)
  const [qbTesting, setQbTesting] = useState(false)
  const [ocsText, setOcsText] = useState('')
  const [ocsMsg, setOcsMsg] = useState('')
  const [ocsOk, setOcsOk] = useState(true)

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
  const setQuestionBank = (k: keyof typeof cfg.setting.apiQueSetting, v: unknown) =>
    setCfg({ ...cfg, setting: { ...cfg.setting, apiQueSetting: { ...cfg.setting.apiQueSetting, [k]: v } } })

  const setTheme = (theme: string) => {
    applyTheme(theme)
    setBasic('theme', theme)
  }

  const selectAIType = (aiType: string) => {
    const preset = AI_PRESETS.find(p => p.value === aiType)
    const nextUrl = preset?.url || ''
    const currentUrl = cfg.setting.aiSetting.aiUrl || ''
    setCfg({
      ...cfg,
      setting: {
        ...cfg.setting,
        aiSetting: {
          ...cfg.setting.aiSetting,
          aiType,
          aiUrl: isPresetAIUrl(currentUrl) ? nextUrl : currentUrl,
        },
      },
    })
  }

  const selectQuestionBankProtocol = (protocol: string) => {
    const preset = QUESTION_BANK_PRESETS.find(p => p.value === protocol)
    if (!preset) return
    const currentUrl = cfg.setting.apiQueSetting.url || ''
    setCfg({
      ...cfg,
      setting: {
        ...cfg.setting,
        apiQueSetting: {
          ...cfg.setting.apiQueSetting,
          protocol,
          url: isPresetQuestionBankUrl(currentUrl) ? preset.url : currentUrl,
          method: preset.method,
          contentType: preset.contentType,
          tokenParam: preset.tokenParam,
          authType: preset.authType,
          questionField: preset.questionField,
          typeField: preset.typeField,
          optionsField: preset.optionsField,
          courseNameField: preset.courseNameField,
          optionsFormat: preset.optionsFormat,
          typeMap: preset.typeMap,
          answerPath: preset.answerPath,
          answerSplit: preset.answerSplit,
        },
      },
    })
  }

  const setQuestionBankURL = (url: string) => {
    const preset = presetForQuestionBankUrl(url)
    if (!preset) {
      setQuestionBank('url', url)
      return
    }
    setCfg({
      ...cfg,
      setting: {
        ...cfg.setting,
        apiQueSetting: {
          ...cfg.setting.apiQueSetting,
          protocol: preset.value,
          url: preset.url,
          method: preset.method,
          contentType: preset.contentType,
          tokenParam: preset.tokenParam,
          authType: preset.authType,
          questionField: preset.questionField,
          typeField: preset.typeField,
          optionsField: preset.optionsField,
          courseNameField: preset.courseNameField,
          optionsFormat: preset.optionsFormat,
          typeMap: preset.typeMap,
          answerPath: preset.answerPath,
          answerSplit: preset.answerSplit,
        },
      },
    })
  }

  const save = async () => {
    setSaving(true); setMsgText('')
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
      setCfg(plain); setMsgOk(true); setMsgText('导入成功')
      applyTheme(plain.setting.basicSetting.theme || 'dark')
    } else if (r.error !== '用户取消') {
      setMsgOk(false); setMsgText(r.error ?? '导入失败')
    }
  }

  const exportCfg = async () => {
    const r = await api.exportConfig(cfg)
    if (!r.ok && r.error !== '用户取消') { setMsgOk(false); setMsgText(r.error ?? '导出失败') }
  }

  const testAI = async () => {
    setTesting(true); setTestMsg('')
    const r = await api.testAIConfig()
    setTesting(false); setTestOk(r.ok)
    setTestMsg(r.ok ? String(r.data) : r.error ?? '测试失败')
  }

  const testQuestionBank = async () => {
    await api.saveConfig(cfg)
    setQbTesting(true); setQbTestMsg('')
    const r = await api.testQuestionBankConfig()
    setQbTesting(false); setQbTestOk(r.ok)
    setQbTestMsg(r.ok ? String(r.data) : r.error ?? '测试失败')
  }

  const applyOCSConfig = () => {
    try {
      const item = parseOCSAnswererConfig(ocsText)
      const headers = item.headers || {}
      const auth = headers.Authorization || headers.authorization || ''
      let token = ''
      let tokenParam = 'token'
      let authType = 'none'
      if (auth.toLowerCase().startsWith('bearer ')) {
        token = auth.slice(7).trim()
        tokenParam = 'Authorization'
        authType = 'bearer'
      } else if (auth) {
        token = auth.trim()
        tokenParam = 'Authorization'
        authType = 'header'
      }
      const contentType = String(item.contentType || headers['Content-Type'] || headers['content-type'] || 'application/json').toLowerCase().includes('form') ? 'form' : 'json'
      const next = {
        ...cfg.setting.apiQueSetting,
        protocol: 'custom',
        url: item.url || cfg.setting.apiQueSetting.url,
        method: String(item.method || 'POST').toUpperCase(),
        contentType,
        token,
        tokenParam,
        authType,
        questionField: findOCSPlaceholder(item.data, 'title') || findOCSPlaceholder(item.data, 'question') || 'title',
        typeField: findOCSPlaceholder(item.data, 'type') || 'type',
        optionsField: findOCSPlaceholder(item.data, 'options') || 'options',
        courseNameField: findOCSPlaceholder(item.data, 'courseName') || findOCSPlaceholder(item.data, 'course') || '',
        optionsFormat: 'text',
        answerPath: inferAnswerPathFromHandler(item.handler),
        answerSplit: cfg.setting.apiQueSetting.answerSplit || '#',
      }
      setCfg({ ...cfg, setting: { ...cfg.setting, apiQueSetting: next } })
      setOcsOk(true)
      setOcsMsg(`已应用 OCS 配置：${item.name || item.url || '未命名题库'}`)
    } catch (err) {
      setOcsOk(false)
      setOcsMsg(err instanceof Error ? err.message : 'OCS 配置解析失败')
    }
  }

  const apiKeyLooksLikeModel = cfg.setting.aiSetting.apiKey.includes('/')
  const siliconUrlLooksWrong =
    cfg.setting.aiSetting.aiType === 'SILICON' &&
    cfg.setting.aiSetting.aiUrl.includes('cloud.siliconflow')
  const questionBankNeedsToken =
    ['wanjuan', 'zerror', 'xhwlgzs', 'ocs'].includes(cfg.setting.apiQueSetting.protocol || '') &&
    (cfg.setting.apiQueSetting.authType || 'none') !== 'none' &&
    !(cfg.setting.apiQueSetting.token || '').trim()

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 18 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>{'全局设置'}</div>
        <div className="flex-row">
          {msgText && (
            <span style={{ fontSize: 12, color: msgOk ? 'var(--success)' : 'var(--danger)' }}>
              {msgText}
            </span>
          )}
          <button className="btn btn-ghost btn-sm" onClick={importCfg}>
            <Upload size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            {'导入配置'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={exportCfg}>
            <Download size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            {'导出配置'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => api.openDataDir()}>
            <FolderOpen size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            {'数据目录'}
          </button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>
            <Save size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>

      <Section title="基础设置">
        <div className="form-row form-row-3">
          <FormGroup label="界面主题">
            <AnimatedSelect
              value={cfg.setting.basicSetting.theme || 'dark'}
              options={THEMES.map(t => ({ value: t.value, label: t.label }))}
              onChange={v => setTheme(String(v))}
            />
          </FormGroup>
          <FormGroup label="完成提示音">
            <AnimatedSelect
              value={cfg.setting.basicSetting.completionTone}
              options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
              onChange={v => setBasic('completionTone', Number(v))}
            />
          </FormGroup>
          <FormGroup label="彩色日志">
            <AnimatedSelect
              value={cfg.setting.basicSetting.colorLog}
              options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
              onChange={v => setBasic('colorLog', Number(v))}
            />
          </FormGroup>
          <FormGroup label="输出日志文件">
            <AnimatedSelect
              value={cfg.setting.basicSetting.logOutFileSw}
              options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
              onChange={v => setBasic('logOutFileSw', Number(v))}
            />
          </FormGroup>
          <FormGroup label="日志等级">
            <AnimatedSelect
              value={cfg.setting.basicSetting.logLevel}
              options={['INFO', 'DEBUG', 'WARN', 'ERROR'].map(l => ({ value: l, label: l }))}
              onChange={v => setBasic('logLevel', String(v))}
            />
          </FormGroup>
          <FormGroup label="日志模式">
            <AnimatedSelect
              value={cfg.setting.basicSetting.logModel}
              options={[{ value: 0, label: '以视频提交为基准' }, { value: 1, label: '以课程为基准' }]}
              onChange={v => setBasic('logModel', Number(v))}
            />
          </FormGroup>
        </div>
      </Section>

      <Section title="任务并发">
        <div className="alert alert-info" style={{ marginBottom: 12 }}>
          {'控制桌面端最多同时运行的账号任务数量。超过上限时新任务会被拒绝启动。'}
        </div>
        <div style={{ maxWidth: 240 }}>
          <FormGroup label="最大同时运行任务数 (1-10)">
            <input
              className="form-input"
              type="number" min={1} max={10} step={1}
              value={cfg.setting.basicSetting.maxWorkers ?? 3}
              onChange={e => setBasic('maxWorkers', Math.min(10, Math.max(1, Number(e.target.value || 3))))}
            />
          </FormGroup>
        </div>
      </Section>

      <Section title="AI 设置" action={
        <button className="btn btn-ghost btn-sm" onClick={testAI} disabled={testing}>
          <Zap size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
          {testing ? '测试中…' : '测试连接'}
        </button>
      }>
        {testMsg && (
          <div className={testOk ? 'alert alert-info' : 'alert alert-warn'} style={{ marginBottom: 12 }}>
            {testMsg}
          </div>
        )}
        <div className="form-row form-row-2">
          <FormGroup label="AI 类型">
            <AnimatedSelect
              value={cfg.setting.aiSetting.aiType}
              options={AI_PRESETS.map(p => ({ value: p.value, label: p.label }))}
              onChange={v => selectAIType(String(v))}
            />
          </FormGroup>
          <FormGroup label="模型名">
            <input className="form-input" value={cfg.setting.aiSetting.model} placeholder="例如 Qwen/Qwen3-32B" onChange={e => setAI('model', e.target.value)} />
          </FormGroup>
          <FormGroup label="API 地址">
            <input className="form-input" value={cfg.setting.aiSetting.aiUrl} onChange={e => setAI('aiUrl', e.target.value)} />
            {siliconUrlLooksWrong && (
              <div className="alert alert-warn" style={{ marginTop: 6, fontSize: 12 }}>
                {'cloud.siliconflow.cn 是控制台，不是 API 地址。可填 https://api.siliconflow.cn/v1/chat/completions'}
              </div>
            )}
          </FormGroup>
          <FormGroup label="API Key">
            <div className="flex-row">
              <input className="form-input" type={showApiKey ? 'text' : 'password'} value={cfg.setting.aiSetting.apiKey} onChange={e => setAI('apiKey', e.target.value)} />
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }} onClick={() => setShowApiKey(v => !v)}>
                {showApiKey ? <EyeOff size={14} strokeWidth={2} /> : <Eye size={14} strokeWidth={2} />}
              </button>
            </div>
            {apiKeyLooksLikeModel && (
              <div className="alert alert-warn" style={{ marginTop: 6, fontSize: 12 }}>
                {'API Key 看起来像模型名，请检查填写位置。'}
              </div>
            )}
          </FormGroup>
        </div>
      </Section>

      <Section title="外部题库接口" action={
        <button className="btn btn-ghost btn-sm" onClick={testQuestionBank} disabled={qbTesting}>
          <Zap size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
          {qbTesting ? '测试中...' : '测试题库'}
        </button>
      }>
        {qbTestMsg && (
          <div className={qbTestOk ? 'alert alert-info' : 'alert alert-warn'} style={{ marginBottom: 12 }}>
            {qbTestMsg}
          </div>
        )}
        <div className="alert alert-info" style={{ marginBottom: 12 }}>
          {'学习通选择“外部题库”时使用。Yatori 原协议保持兼容；万卷/ZE 会自动按各自协议转换请求和答案。'}
        </div>
        <details className="alert alert-info" style={{ marginBottom: 12, lineHeight: 1.7 }}>
          <summary style={{ cursor: 'pointer', fontWeight: 700 }}>{'自定义第三方题库填写说明'}</summary>
          <div style={{ marginTop: 8 }}>{'请打开题库平台的 API 文档，对照“请求地址、认证方式、请求参数、响应示例”填写。URL 填查题/搜题接口，不要填登录页、用户中心、AI 出题或文档页面。'}</div>
          <div>{'Token / Key：填平台给你的密钥；Key 参数名：按文档填写，如 token、key、Authorization、X-API-Key。认证方式选 Bearer Header 时会发送 Authorization: Bearer <Key>。'}</div>
          <div>{'字段名：题目字段对应文档里的 title/question/content/value/tm；题型字段一般是 type/question_type；选项字段一般是 options；答案路径按响应 JSON 写，例如 answers、data.answer、data.data、data.questions.0.answer。'}</div>
          <div>{'题型映射用于把学习通题型转成接口要求的值，例如 single=0,multiple=1,fill=2,judge=3,short=4 或 single=single_choice,multiple=multiple_choice。'}</div>
          <div>{'OCS 题库配置：很多题库会提供 name/url/method/data/headers/handler 的 JSON，可直接粘贴到下方导入。为安全起见，程序只解析字段和答案路径，不执行 handler 脚本。'}</div>
          <div>{'示例：小杭题库（XHWL）应使用 /v1/questions/search，认证方式 Bearer Header，Key 参数名 Authorization，题目字段 value，选项格式对象，答案路径通常填 data.answer。'}</div>
        </details>
        <details className="alert alert-info" style={{ marginBottom: 12 }}>
          <summary style={{ cursor: 'pointer', fontWeight: 700 }}>{'粘贴 OCS 题库配置自动填充'}</summary>
          <div style={{ marginTop: 10 }}>
            <textarea
              className="form-input"
              style={{ minHeight: 110, resize: 'vertical', fontFamily: 'var(--font-mono)' }}
              placeholder={'粘贴题库网站提供的 OCS JSON 配置，例如 [{ "name": "...", "url": "...", "data": { "title": "${title}" }, "headers": { "Authorization": "Bearer ..." } }]'}
              value={ocsText}
              onChange={e => setOcsText(e.target.value)}
            />
            <div className="flex-row" style={{ marginTop: 8 }}>
              <button className="btn btn-ghost btn-sm" onClick={applyOCSConfig}>{'应用 OCS 配置'}</button>
              {ocsMsg && <span style={{ fontSize: 12, color: ocsOk ? 'var(--success)' : 'var(--danger)' }}>{ocsMsg}</span>}
            </div>
          </div>
        </details>
        <div className="form-row form-row-2">
          <FormGroup label="题库平台">
            <AnimatedSelect
              value={cfg.setting.apiQueSetting.protocol || 'yatori'}
              options={QUESTION_BANK_PRESETS.map(p => ({ value: p.value, label: p.label }))}
              onChange={v => selectQuestionBankProtocol(String(v))}
            />
          </FormGroup>
          <FormGroup label="题库接口 URL">
            <input className="form-input" value={cfg.setting.apiQueSetting.url}
              onChange={e => setQuestionBankURL(e.target.value)} />
          </FormGroup>
          <FormGroup label="Token / Key">
            <input className="form-input" type="password" value={cfg.setting.apiQueSetting.token || ''}
              onChange={e => setQuestionBank('token', e.target.value)} />
            {questionBankNeedsToken && (
              <div className="alert alert-warn" style={{ marginTop: 6, fontSize: 12 }}>
                {'当前题库需要 Token / Key。未填写时接口会返回 401，外部题库不会生效。'}
              </div>
            )}
          </FormGroup>
        </div>

        <details className="alert alert-info" style={{ marginTop: 12 }}>
          <summary style={{ cursor: 'pointer', fontWeight: 700 }}>{'高级配置：认证、请求与字段映射'}</summary>
          <div className="form-row form-row-2" style={{ marginTop: 12 }}>
            <FormGroup label="Key 参数名">
              <input className="form-input" value={cfg.setting.apiQueSetting.tokenParam || 'token'}
                onChange={e => setQuestionBank('tokenParam', e.target.value)} />
            </FormGroup>
            <FormGroup label="认证方式">
              <AnimatedSelect
                value={cfg.setting.apiQueSetting.authType || 'none'}
                options={[
                  { value: 'none', label: '无认证' },
                  { value: 'query', label: 'URL Query' },
                  { value: 'form', label: 'Form 参数' },
                  { value: 'body', label: 'JSON Body' },
                  { value: 'bearer', label: 'Bearer Header' },
                  { value: 'header', label: '自定义 Header' },
                ]}
                onChange={v => setQuestionBank('authType', String(v))}
              />
            </FormGroup>
            <FormGroup label="请求方式">
              <AnimatedSelect
                value={cfg.setting.apiQueSetting.method || 'POST'}
                options={['GET', 'POST'].map(v => ({ value: v, label: v }))}
                onChange={v => setQuestionBank('method', String(v))}
              />
            </FormGroup>
            <FormGroup label="请求格式">
              <AnimatedSelect
                value={cfg.setting.apiQueSetting.contentType || 'json'}
                options={[{ value: 'json', label: 'JSON' }, { value: 'form', label: 'Form 表单' }]}
                onChange={v => setQuestionBank('contentType', String(v))}
              />
            </FormGroup>
            <FormGroup label="题目字段名">
              <input className="form-input" value={cfg.setting.apiQueSetting.questionField || 'title'}
                onChange={e => setQuestionBank('questionField', e.target.value)} />
            </FormGroup>
            <FormGroup label="题型字段名">
              <input className="form-input" value={cfg.setting.apiQueSetting.typeField || 'type'}
                onChange={e => setQuestionBank('typeField', e.target.value)} />
            </FormGroup>
            <FormGroup label="选项字段名">
              <input className="form-input" value={cfg.setting.apiQueSetting.optionsField || 'options'}
                onChange={e => setQuestionBank('optionsField', e.target.value)} />
            </FormGroup>
            <FormGroup label="课程字段名">
              <input className="form-input" value={cfg.setting.apiQueSetting.courseNameField || ''}
                onChange={e => setQuestionBank('courseNameField', e.target.value)} />
            </FormGroup>
            <FormGroup label="选项格式">
              <AnimatedSelect
                value={cfg.setting.apiQueSetting.optionsFormat || 'array'}
                options={[
                  { value: 'array', label: '数组 A.xxx' },
                  { value: 'object', label: '对象 {A: xxx}' },
                  { value: 'text', label: '换行文本' },
                  { value: 'values', label: '仅选项文本数组' },
                ]}
                onChange={v => setQuestionBank('optionsFormat', String(v))}
              />
            </FormGroup>
            <FormGroup label="题型映射">
              <input className="form-input" value={cfg.setting.apiQueSetting.typeMap || ''} placeholder="single=0,multiple=1,fill=2,judge=3,short=4"
                onChange={e => setQuestionBank('typeMap', e.target.value)} />
            </FormGroup>
            <FormGroup label="答案路径">
              <input className="form-input" value={cfg.setting.apiQueSetting.answerPath || 'answers'}
                onChange={e => setQuestionBank('answerPath', e.target.value)} />
            </FormGroup>
            <FormGroup label="多答案分隔符">
              <input className="form-input" value={cfg.setting.apiQueSetting.answerSplit || '#'}
                onChange={e => setQuestionBank('answerSplit', e.target.value)} />
            </FormGroup>
          </div>
        </details>      </Section>
      <Section title="邮件通知">
        <div className="form-row form-row-3">
          <FormGroup label="开关">
            <AnimatedSelect
              value={cfg.setting.emailInform.sw}
              options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
              onChange={v => setEmail('sw', Number(v))}
            />
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
              <button className="btn btn-ghost btn-sm" style={{ flexShrink: 0 }} onClick={() => setShowEmailPwd(v => !v)}>
                {showEmailPwd ? <EyeOff size={14} strokeWidth={2} /> : <Eye size={14} strokeWidth={2} />}
              </button>
            </div>
          </FormGroup>
        </div>
      </Section>
    </div>
  )
}
