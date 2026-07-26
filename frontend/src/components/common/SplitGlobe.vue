<template>
  <!-- 剖半点阵地球：球心压在视口左右边界线上、垂直居中、fixed 随滚动全程陪伴。
       左边缘露出右半球、右边缘露出左半球，两半相位相同 —— 同一颗地球被剖开分置两侧 -->
  <canvas ref="leftRef" class="split-globe split-globe-l" aria-hidden="true" />
  <canvas ref="rightRef" class="split-globe split-globe-r" aria-hidden="true" />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { createDotGlobe, type DotGlobeHandle } from '@/composables/useDotGlobe'

const leftRef = ref<HTMLCanvasElement | null>(null)
const rightRef = ref<HTMLCanvasElement | null>(null)
const globes: DotGlobeHandle[] = []
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches

// 主题统一读 html.dark（首页/认证页切主题时都会同步该 class）
const isDark = () => document.documentElement.classList.contains('dark')

// reduced-motion 下 canvas 只画首帧，监听主题切换手动重画
let themeObserver: MutationObserver | null = null

onMounted(() => {
  const opts = { isDark, reducedMotion, phase: 0 }
  if (leftRef.value) globes.push(createDotGlobe(leftRef.value, opts))
  if (rightRef.value) globes.push(createDotGlobe(rightRef.value, opts))
  if (reducedMotion) {
    themeObserver = new MutationObserver(() => globes.forEach(g => g.redraw()))
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  }
})

onUnmounted(() => {
  globes.forEach(g => g.stop())
  themeObserver?.disconnect()
})
</script>

<style scoped>
.split-globe {
  position: fixed;
  top: 50%;
  width: min(880px, 78vw);
  height: min(880px, 78vw);
  pointer-events: none;
  opacity: 0.6;
  -webkit-mask-image: radial-gradient(circle at 50% 50%, black 45%, transparent 74%);
  mask-image: radial-gradient(circle at 50% 50%, black 45%, transparent 74%);
}
.split-globe-l { left: 0; transform: translate(-50%, -50%); }
.split-globe-r { right: 0; transform: translate(50%, -50%); }
:global(html.dark .split-globe) { opacity: 0.75; }
@media (max-width: 900px) { .split-globe { display: none; } }
</style>
