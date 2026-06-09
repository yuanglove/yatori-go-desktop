import { useRef, useState, KeyboardEvent, ClipboardEvent } from 'react'
import { X } from 'lucide-react'

interface Props {
  value: string[]
  onChange(tags: string[]): void
  placeholder?: string
  variant?: 'include' | 'exclude'
}

const SPLIT_RE = /[,，\n]+/

function splitInput(raw: string): string[] {
  return raw.split(SPLIT_RE).map(s => s.trim()).filter(Boolean)
}

export default function TagInput({ value, onChange, placeholder = '输入后按 Enter 添加', variant = 'include' }: Props) {
  const [draft, setDraft] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const addTags = (items: string[]) => {
    const next = [...value]
    for (const item of items) {
      if (item && !next.includes(item)) next.push(item)
    }
    onChange(next)
  }

  const removeTag = (idx: number) => {
    const next = [...value]
    next.splice(idx, 1)
    onChange(next)
  }

  const handleKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      const parts = splitInput(draft)
      if (parts.length) { addTags(parts); setDraft('') }
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      removeTag(value.length - 1)
    }
  }

  const handleChange = (raw: string) => {
    // auto-split on comma or Chinese comma typed mid-string
    if (SPLIT_RE.test(raw)) {
      const parts = splitInput(raw)
      if (parts.length) { addTags(parts); setDraft(''); return }
    }
    setDraft(raw)
  }

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault()
    const text = e.clipboardData.getData('text')
    const parts = splitInput(text)
    if (parts.length) { addTags(parts); setDraft('') }
  }

  return (
    <div
      className={`tag-input-wrap tag-${variant}`}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((tag, i) => (
        <span key={i} className="tag-chip">
          {tag}
          <button
            type="button"
            className="tag-chip-del"
            onClick={e => { e.stopPropagation(); removeTag(i) }}
            tabIndex={-1}
            aria-label={'删除 ' + tag}
          >
            <X size={10} strokeWidth={2.5} />
          </button>
        </span>
      ))}
      <input
        ref={inputRef}
        className="tag-input-field"
        value={draft}
        placeholder={value.length === 0 ? placeholder : ''}
        onChange={e => handleChange(e.target.value)}
        onKeyDown={handleKey}
        onPaste={handlePaste}
      />
    </div>
  )
}
