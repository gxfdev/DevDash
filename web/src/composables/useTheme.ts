import { ref, computed, watch } from 'vue'
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'

export type ThemeMode = 'dark' | 'light' | 'auto'
export type DensityMode = 'compact' | 'default' | 'comfortable'

const themeMode = ref<ThemeMode>((localStorage.getItem('theme-mode') as ThemeMode) || 'dark')
const densityMode = ref<DensityMode>((localStorage.getItem('density-mode') as DensityMode) || 'default')
const themeColor = ref(localStorage.getItem('theme-color') || '#388bfd')

watch(themeMode, (v) => localStorage.setItem('theme-mode', v))
watch(densityMode, (v) => localStorage.setItem('density-mode', v))
watch(themeColor, (v) => {
  localStorage.setItem('theme-color', v)
  document.documentElement.style.setProperty('--primary-color', v)
})

function isSystemDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function isDarkActive(): boolean {
  if (themeMode.value === 'auto') return isSystemDark()
  return themeMode.value === 'dark'
}

function syncDataTheme() {
  document.documentElement.setAttribute('data-theme', isDarkActive() ? 'dark' : 'light')
}

watch([themeMode], () => { syncDataTheme() }, { immediate: true })

function buildDarkOverrides(color: string): GlobalThemeOverrides {
  return {
    common: {
      primaryColor: color,
      primaryColorHover: lighten(color, 20),
      primaryColorPressed: darken(color, 15),
      primaryColorSuppl: color,
      bodyColor: '#0d1117',
      cardColor: '#161b22',
      modalColor: '#161b22',
      tableColor: '#161b22',
      tableColorHover: '#1c2128',
      tableColorStriped: '#1c2128',
      popoverColor: '#161b22',
      inputColor: '#0d1117',
      borderColor: '#30363d',
      dividerColor: '#21262d',
      textColorBase: '#e6edf3',
      textColor1: '#e6edf3',
      textColor2: '#8b949e',
      textColor3: '#6e7681',
      fontSize: densityFont(),
      fontSizeMini: densityFontMini(),
    },
    Button: { textColorPrimary: '#ffffff' },
    Card: { colorEmbedded: '#161b22' },
    Menu: {
      itemTextColor: '#8b949e',
      itemTextColorHover: '#e6edf3',
      itemTextColorActive: color,
      itemTextColorChildActive: color,
      itemIconColor: '#8b949e',
      itemIconColorHover: '#e6edf3',
      itemIconColorActive: color,
      itemColorActive: 'rgba(56,139,253,0.08)',
      itemColorHover: 'rgba(56,139,253,0.06)',
    },
    DataTable: {
      thColor: '#1c2128',
      tdColor: '#161b22',
      tdColorHover: '#1c2128',
      thTextColor: '#8b949e',
      tdTextColor: '#e6edf3',
      borderColor: '#30363d',
    },
    Input: {
      color: '#0d1117',
      colorFocus: '#0d1117',
      border: '1px solid #30363d',
      borderHover: `1px solid ${color}`,
      borderFocus: `1px solid ${color}`,
      textColor: '#e6edf3',
      placeholderColor: '#6e7681',
    },
    Tabs: {
      tabTextColorLine: '#8b949e',
      tabTextColorActiveLine: color,
      tabTextColorHoverLine: '#e6edf3',
      barColor: color,
    },
  }
}

function buildLightOverrides(color: string): GlobalThemeOverrides {
  return {
    common: {
      primaryColor: color,
      primaryColorHover: lighten(color, 20),
      primaryColorPressed: darken(color, 15),
      primaryColorSuppl: color,
      bodyColor: '#ffffff',
      cardColor: '#f6f8fa',
      modalColor: '#ffffff',
      tableColor: '#ffffff',
      tableColorHover: '#f6f8fa',
      tableColorStriped: '#f6f8fa',
      popoverColor: '#ffffff',
      inputColor: '#ffffff',
      borderColor: '#d0d7de',
      dividerColor: '#d8dee4',
      textColorBase: '#1f2328',
      textColor1: '#1f2328',
      textColor2: '#656d76',
      textColor3: '#8b949e',
      fontSize: densityFont(),
      fontSizeMini: densityFontMini(),
    },
    Button: { textColorPrimary: '#ffffff' },
    Card: { colorEmbedded: '#f6f8fa' },
    Menu: {
      itemTextColor: '#656d76',
      itemTextColorHover: '#1f2328',
      itemTextColorActive: color,
      itemTextColorChildActive: color,
      itemIconColor: '#656d76',
      itemIconColorHover: '#1f2328',
      itemIconColorActive: color,
      itemColorActive: `rgba(${hexToRgb(color)},0.08)`,
      itemColorHover: `rgba(${hexToRgb(color)},0.06)`,
    },
    DataTable: {
      thColor: '#f6f8fa',
      tdColor: '#ffffff',
      tdColorHover: '#f6f8fa',
      thTextColor: '#656d76',
      tdTextColor: '#1f2328',
      borderColor: '#d0d7de',
    },
    Input: {
      color: '#ffffff',
      colorFocus: '#ffffff',
      border: '1px solid #d0d7de',
      borderHover: `1px solid ${color}`,
      borderFocus: `1px solid ${color}`,
      textColor: '#1f2328',
      placeholderColor: '#8b949e',
    },
    Tabs: {
      tabTextColorLine: '#656d76',
      tabTextColorActiveLine: color,
      tabTextColorHoverLine: '#1f2328',
      barColor: color,
    },
  }
}

function densityFont(): string {
  const map: Record<DensityMode, string> = { compact: '13px', default: '14px', comfortable: '15px' }
  return map[densityMode.value]
}

function densityFontMini(): string {
  const map: Record<DensityMode, string> = { compact: '11px', default: '12px', comfortable: '13px' }
  return map[densityMode.value]
}

function lighten(hex: string, pct: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const nr = Math.min(255, r + Math.round((255 - r) * pct / 100))
  const ng = Math.min(255, g + Math.round((255 - g) * pct / 100))
  const nb = Math.min(255, b + Math.round((255 - b) * pct / 100))
  return `#${nr.toString(16).padStart(2, '0')}${ng.toString(16).padStart(2, '0')}${nb.toString(16).padStart(2, '0')}`
}

function darken(hex: string, pct: number): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const nr = Math.max(0, Math.round(r * (1 - pct / 100)))
  const ng = Math.max(0, Math.round(g * (1 - pct / 100)))
  const nb = Math.max(0, Math.round(b * (1 - pct / 100)))
  return `#${nr.toString(16).padStart(2, '0')}${ng.toString(16).padStart(2, '0')}${nb.toString(16).padStart(2, '0')}`
}

function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `${r},${g},${b}`
}

export function useTheme() {
  const naiveTheme = computed(() => isDarkActive() ? darkTheme : null)
  const themeOverrides = computed(() =>
    isDarkActive() ? buildDarkOverrides(themeColor.value) : buildLightOverrides(themeColor.value)
  )
  const isDark = computed(() => isDarkActive())

  function setThemeMode(mode: ThemeMode) { themeMode.value = mode }
  function setDensity(mode: DensityMode) { densityMode.value = mode }
  function setColor(color: string) { themeColor.value = color }

  return {
    themeMode, densityMode, themeColor,
    naiveTheme, themeOverrides, isDark,
    setThemeMode, setDensity, setColor,
    isDarkActive,
  }
}
