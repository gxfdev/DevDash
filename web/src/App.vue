<script setup lang="ts">
import { darkTheme, zhCN, dateZhCN } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'

const overrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#7c5cff',
    primaryColorHover: '#8f74ff',
    primaryColorPressed: '#6a4ef0',
    primaryColorSuppl: '#7c5cff',
    infoColor: '#22d3ee',
    successColor: '#34d399',
    warningColor: '#fbbf24',
    errorColor: '#fb7185',
    bodyColor: 'transparent',
    cardColor: 'rgba(17,23,41,0.78)',
    modalColor: '#0f1526',
    popoverColor: '#111827',
    tableColor: 'transparent',
    inputColor: 'rgba(255,255,255,0.04)',
    borderColor: 'rgba(255,255,255,0.1)',
    dividerColor: 'rgba(255,255,255,0.08)',
    textColorBase: '#e9eefc',
    textColor1: '#e9eefc',
    textColor2: '#c3cee6',
    textColor3: '#8b98b8',
    fontFamily: "'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
    borderRadius: '10px',
  },
  Card: { color: 'rgba(17,23,41,0.7)', borderColor: 'rgba(255,255,255,0.08)' },
  Button: { borderRadiusMedium: '9px', fontWeight: '600' },
  DataTable: { thColor: 'rgba(255,255,255,0.03)', tdColor: 'transparent', thTextColor: '#9aa7c4', borderColor: 'rgba(255,255,255,0.06)' },
  Tag: { borderRadius: '999px' },
  Drawer: { color: '#0b1020' },
  Dialog: { color: '#0d1323' },
  Input: { border: '1px solid rgba(255,255,255,0.1)' },
  Tabs: { tabTextColorActiveLine: '#c4b5fd', barColor: '#7c5cff' },
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
