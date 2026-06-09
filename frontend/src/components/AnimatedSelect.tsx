import { CSSProperties, useRef, useState, useEffect, KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown, Check } from 'lucide-react'

export interface SelectOption {
  value: string | number
  label: string
  disabled?: boolean
}

interface Props {
  value: string | number
  options: SelectOption[]
  onChange(value: string | number): void
  placeholder?: string
  disabled?: boolean
  className?: string
  ariaLabel?: string
}

export default function AnimatedSelect({
  value, options, onChange, placeholder, disabled, className, ariaLabel,
}: Props) {
  const [open, setOpen] = useState(false)
  const [closing, setClosing] = useState(false)
  const [highlighted, setHighlighted] = useState<number>(-1)
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({})
  const wrapRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLUListElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  const selected = options.find(o => o.value === value)
  const label = selected?.label ?? placeholder ?? ''

  // close with animation
  const closeMenu = () => {
    if (!open) return
    setClosing(true)
    setTimeout(() => { setOpen(false); setClosing(false) }, 130)
  }

  const updateMenuPosition = () => {
    const trigger = triggerRef.current
    if (!trigger) return
    const rect = trigger.getBoundingClientRect()
    const gap = 6
    const padding = 12
    const maxMenuHeight = 280
    const spaceBelow = window.innerHeight - rect.bottom - padding
    const spaceAbove = rect.top - padding
    const openUp = spaceBelow < 160 && spaceAbove > spaceBelow
    const maxHeight = Math.max(120, Math.min(maxMenuHeight, openUp ? spaceAbove - gap : spaceBelow - gap))

    setMenuStyle({
      position: 'fixed',
      left: rect.left,
      right: 'auto',
      top: openUp ? 'auto' : rect.bottom + gap,
      bottom: openUp ? window.innerHeight - rect.top + gap : 'auto',
      width: rect.width,
      maxHeight,
      zIndex: 10000,
      transformOrigin: openUp ? 'bottom center' : 'top center',
    })
  }

  // click outside
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const target = e.target as Node
      if (wrapRef.current?.contains(target) || menuRef.current?.contains(target)) return
      closeMenu()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  useEffect(() => {
    if (!open) return
    updateMenuPosition()
    const handler = () => updateMenuPosition()
    window.addEventListener('resize', handler)
    window.addEventListener('scroll', handler, true)
    return () => {
      window.removeEventListener('resize', handler)
      window.removeEventListener('scroll', handler, true)
    }
  }, [open, options.length])

  // scroll highlighted item into view
  useEffect(() => {
    if (highlighted < 0 || !menuRef.current) return
    const item = menuRef.current.children[highlighted] as HTMLElement | undefined
    item?.scrollIntoView({ block: 'nearest' })
  }, [highlighted])

  const openMenu = () => {
    if (disabled) return
    const idx = options.findIndex(o => o.value === value)
    setHighlighted(idx >= 0 ? idx : 0)
    updateMenuPosition()
    setOpen(true)
    setClosing(false)
  }

  const selectOption = (opt: SelectOption) => {
    if (opt.disabled) return
    onChange(opt.value)
    closeMenu()
  }

  const handleTriggerKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open ? closeMenu() : openMenu() }
    else if (e.key === 'Escape') closeMenu()
    else if (e.key === 'ArrowDown') { e.preventDefault(); if (!open) openMenu(); else setHighlighted(h => Math.min(h + 1, options.length - 1)) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); if (!open) openMenu(); else setHighlighted(h => Math.max(h - 1, 0)) }
    else if (e.key === 'Tab') closeMenu()
  }

  const handleMenuKey = (e: KeyboardEvent<HTMLUListElement>) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setHighlighted(h => Math.min(h + 1, options.length - 1)) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setHighlighted(h => Math.max(h - 1, 0)) }
    else if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); if (highlighted >= 0) selectOption(options[highlighted]) }
    else if (e.key === 'Escape' || e.key === 'Tab') { closeMenu(); triggerRef.current?.focus() }
  }

  return (
    <div
      ref={wrapRef}
      className={'animated-select' + (className ? ' ' + className : '') + (disabled ? ' animated-select-disabled' : '')}
    >
      <button
        ref={triggerRef}
        type="button"
        className={'animated-select-trigger' + (open ? ' open' : '')}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        onMouseDown={e => { e.preventDefault(); open ? closeMenu() : openMenu() }}
        onKeyDown={handleTriggerKey}
      >
        <span className="animated-select-label">{label}</span>
        <ChevronDown
          size={14}
          strokeWidth={2}
          className={'animated-select-chevron' + (open ? ' rotated' : '')}
          aria-hidden="true"
        />
      </button>

      {open && createPortal(
        <ul
          ref={menuRef}
          role="listbox"
          className={'animated-select-menu' + (closing ? ' closing' : ' open')}
          style={menuStyle}
          tabIndex={-1}
          aria-label={ariaLabel}
          onKeyDown={handleMenuKey}
        >
          {options.map((opt, i) => (
            <li
              key={opt.value}
              role="option"
              aria-selected={opt.value === value}
              aria-disabled={opt.disabled}
              className={
                'animated-select-option' +
                (opt.value === value ? ' active' : '') +
                (i === highlighted ? ' highlighted' : '') +
                (opt.disabled ? ' disabled' : '')
              }
              onMouseEnter={() => setHighlighted(i)}
              onMouseDown={e => { e.preventDefault(); selectOption(opt) }}
            >
              <span>{opt.label}</span>
              {opt.value === value && (
                <Check size={12} strokeWidth={2.5} className="animated-select-check" aria-hidden="true" />
              )}
            </li>
          ))}
        </ul>,
        document.body,
      )}
    </div>
  )
}
