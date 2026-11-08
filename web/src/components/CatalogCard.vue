<script setup lang="ts">
import { computed } from 'vue'
import {
  SparklesOutline, CodeSlashOutline, ChatbubblesOutline, LayersOutline, EyeOutline,
  GitBranchOutline, CubeOutline, HardwareChipOutline, ServerOutline,
  GitNetworkOutline, RocketOutline, ColorWandOutline, ExtensionPuzzleOutline,
} from '@vicons/ionicons5'
import type { CatalogItem, Service } from '@/types'
import { statusOf } from '@/utils/format'

const props = defineProps<{ item: CatalogItem; service?: Service }>()
defineEmits<{ (e: 'deploy'): void; (e: 'view'): void }>()

const iconMap: Record<string, any> = {
  qwen: SparklesOutline,
  deepseek: ColorWandOutline,
  code: CodeSlashOutline,
  llama: LayersOutline,
  gemma: SparklesOutline,
  embed: GitNetworkOutline,
  vision: EyeOutline,
  chat: ChatbubblesOutline,
  agent: HardwareChipOutline,
  lobe: ExtensionPuzzleOutline,
  flow: GitBranchOutline,
  db: ServerOutline,
  storage: ServerOutline,
  cube: CubeOutline,
}

const icon = computed(() => iconMap[props.item.icon] || RocketOutline)
const deployed = computed(() => !!props.service)
const status = computed(() => (props.service ? statusOf(props.service.status) : null))

function memLabel(mb: number): string {
  return mb >= 1024 ? `${(mb / 1024).toFixed(0)} GB` : `${mb} MB`
}
</script>

<template>
  <article class="catalog-card glass" :style="{ '--accent': item.accent }">
    <header class="cc-head">
      <div class="cc-icon">
        <n-icon :component="icon" :size="23" />
      </div>
      <div class="cc-title-wrap">
        <div class="cc-title">{{ item.name }}</div>
        <div class="cc-meta">{{ item.vendor }} · {{ item.category }}</div>
      </div>
      <span v-if="deployed && status" class="badge" :class="status.badge">{{ status.label }}</span>
      <span v-else-if="item.requires_gpu" class="badge warn">GPU</span>
    </header>

    <p class="cc-summary">{{ item.summary }}</p>

    <div class="cc-tags">
      <span v-for="t in item.tags.slice(0, 3)" :key="t" class="cc-tag">{{ t }}</span>
    </div>

    <div class="cc-stats">
      <span v-if="item.params"><b>{{ item.params }}</b> 参数</span>
      <span v-if="item.size_gb"><b>{{ item.size_gb }}</b> GB</span>
      <span v-if="item.resources?.memory_mb"><b>{{ memLabel(item.resources.memory_mb) }}</b> 内存</span>
      <span v-if="item.default_host_access" class="host-hint">宿主机权限</span>
    </div>

    <footer class="cc-foot">
      <n-button v-if="!deployed" size="small" type="primary" secondary @click="$emit('deploy')">
        <template #icon><n-icon :component="RocketOutline" /></template>
        一键部署
      </n-button>
      <n-button v-else size="small" type="primary" secondary @click="$emit('view')">查看服务</n-button>
      <n-button size="small" quaternary @click="$emit('deploy')">自定义部署</n-button>
    </footer>
  </article>
</template>

<style scoped>
.catalog-card {
  position: relative; padding: 18px 18px 16px; display: flex; flex-direction: column; gap: 12px;
  overflow: hidden; transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}
.catalog-card:hover { border-color: rgba(59,130,246,0.45); }

.cc-head { display: flex; align-items: center; gap: 12px; position: relative; }
.cc-icon {
  width: 46px; height: 46px; border-radius: 13px; display: grid; place-items: center; flex: none;
  color: var(--accent); border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
  background: rgba(255,255,255,0.04);
}
.cc-title-wrap { flex: 1; min-width: 0; }
.cc-title { font-size: 14.5px; font-weight: 680; letter-spacing: -0.2px; }
.cc-meta { font-size: 11px; color: var(--muted); margin-top: 3px; }
.cc-summary { font-size: 12px; color: #97a4c2; line-height: 1.7; margin: 0; min-height: 40px; }
.cc-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.cc-tag { font-size: 10.5px; color: #93a2c4; border: 1px solid rgba(255,255,255,0.08); background: rgba(255,255,255,0.03); padding: 2px 8px; border-radius: 999px; }
.cc-stats { display: flex; gap: 14px; font-size: 11px; color: #6e7c9c; flex-wrap: wrap; }
.cc-stats b { color: #c3cee6; font-weight: 650; }
.host-hint { color: #fcd34d; }
.cc-foot { display: flex; gap: 8px; margin-top: auto; padding-top: 4px; }
</style>
