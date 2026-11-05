<template>
  <canvas ref="canvas" class="ocean-canvas" :aria-label="label" role="img" />
</template>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { type OceanState } from '@/games/token-ocean/engine'
import {
  fishSprites,
  spritePath,
  type FishSprite,
} from '@/games/token-ocean/species'
import { renderOcean, type OceanArt } from '@/games/token-ocean/renderer'
const props = defineProps<{
  state: OceanState
  label: string
  labels: {
    you: string
    eat: string
    danger: string
    hunter?: string
    tail?: string
    shoal?: string
    spines?: string
    venom?: string
    ambush?: string
    stamina?: string
    relic?: string
    migration?: string
    gentle?: string
  }
}>()
const emit = defineEmits<{ ready: []; error: [] }>()
const canvas = ref<HTMLCanvasElement>()
let active = true,
  frame = 0,
  previous = '',
  art: OceanArt | null = null,
  observer: ResizeObserver | undefined
function load(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = reject
    img.src = src
  })
}
function draw() {
  if (!active) return
  const el = canvas.value,
    ctx = el?.getContext('2d')
  const signature = `${props.state.time}:${props.state.event}:${props.state.phase}:${el?.width}:${el?.height}:${JSON.stringify(props.labels)}`
  if (ctx && art && el && signature !== previous) {
    previous = signature
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    renderOcean(
      ctx,
      props.state,
      art,
      el.width,
      el.height,
      props.labels,
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false,
    )
  }
  frame = requestAnimationFrame(draw)
}
onMounted(async () => {
  try {
    const [cove, abyss, crystal, ...sprites] = await Promise.all([
      load('/games/token-fishing/cove.webp'),
      load('/games/token-ocean/abyss-scene.webp'),
      load('/games/token-miner/gem.webp'),
      ...fishSprites.map((id) => load(spritePath(id))),
    ])
    if (!active || !canvas.value) return
    if (!canvas.value.getContext('2d')) throw new Error('Canvas unavailable')
    art = {
      cove: cove!,
      abyss: abyss!,
      crystal: crystal!,
      fish: Object.fromEntries(
        fishSprites.map((id, i) => [id, sprites[i]!]),
      ) as Record<FishSprite, HTMLImageElement>,
    }
    const resize = () => {
      const el = canvas.value
      if (!el) return
      const dpr = Math.min(2, window.devicePixelRatio || 1)
      el.width = Math.max(1, Math.min(2000, Math.round(el.clientWidth * dpr)))
      el.height = Math.max(
        1,
        Math.round((el.width * el.clientHeight) / Math.max(1, el.clientWidth)),
      )
      previous = ''
    }
    observer = new ResizeObserver(resize)
    observer.observe(canvas.value)
    resize()
    draw()
    emit('ready')
  } catch {
    if (active) emit('error')
  }
})
onBeforeUnmount(() => {
  active = false
  cancelAnimationFrame(frame)
  observer?.disconnect()
  art = null
})
</script>
<style scoped>
.ocean-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
