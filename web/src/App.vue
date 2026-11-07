<script setup lang="ts">
import { darkTheme, zhCN, dateZhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'

const overrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#3b82f6',
    primaryColorHover: '#60a5fa',
    primaryColorPressed: '#2563eb',
    primaryColorSuppl: '#3b82f6',
    infoColor: '#0ea5e9',
    successColor: '#34d399',
    warningColor: '#fbbf24',
    errorColor: '#fb7185',
    bodyColor: 'transparent',
    cardColor: '#121821',
    modalColor: '#121821',
    popoverColor: '#161d28',
    tableColor: 'transparent',
    inputColor: 'rgba(255,255,255,0.04)',
    borderColor: 'rgba(255,255,255,0.1)',
    dividerColor: 'rgba(255,255,255,0.08)',
    textColorBase: '#e9eefc',
    textColor1: '#e9eefc',
    textColor2: '#c3cee6',
    textColor3: '#8b98b8',
    fontFamily: "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
    borderRadius: '8px',
  },
  Card: { color: 'rgba(17,23,41,0.7)', borderColor: 'rgba(255,255,255,0.08)' },
  Button: { borderRadiusMedium: '9px', fontWeight: '600' },
  DataTable: { thColor: 'rgba(255,255,255,0.03)', tdColor: 'transparent', thTextColor: '#9aa7c4', borderColor: 'rgba(255,255,255,0.06)' },
  Tag: { borderRadius: '999px' },
  Drawer: { color: '#0b1020' },
  Dialog: { color: '#0d1323' },
  Input: { border: '1px solid rgba(255,255,255,0.1)' },
  Tabs: { tabTextColorActiveLine: '#c4b5fd', barColor: '#3b82f6' },
}
</script>

<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="overrides" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider :max="3">
      <n-dialog-provider>
        <n-notification-provider>
          <router-view />
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
