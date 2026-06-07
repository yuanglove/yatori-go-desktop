export const THEMES = [
  { value: 'dark', label: '深色' },
  { value: 'light', label: '白色' },
  { value: 'green', label: '浅绿色' },
  { value: 'blue', label: '浅蓝色' },
  { value: 'rose', label: '柔粉色' },
  { value: 'amber', label: '暖米色' },
  { value: 'violet', label: '淡紫色' },
  { value: 'graphite', label: '石墨灰' },
  { value: 'cyan', label: '护眼青' },
] as const

export type ThemeName = typeof THEMES[number]['value']

export function normalizeTheme(theme?: string | null): ThemeName {
  return THEMES.some(item => item.value === theme) ? theme as ThemeName : 'dark'
}

export function applyTheme(theme?: string | null) {
  const normalized = normalizeTheme(theme)
  document.documentElement.setAttribute('data-theme', normalized)
  localStorage.setItem('yatori-theme', normalized)
}
