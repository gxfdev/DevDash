<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-notification-provider>
        <n-dialog-provider>
          <router-view />
        </n-dialog-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#388bfd',
    primaryColorHover: '#58a6ff',
    primaryColorPressed: '#1f6feb',
    primaryColorSuppl: '#388bfd',
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
  },
  Button: {
    textColorPrimary: '#ffffff',
  },
  Card: {
    colorEmbedded: '#161b22',
  },
}
</script>