<template>
  <section class="ct-battle" :class="{ 'ct-battle-finished': finished }" role="region" :aria-label="t('tower.classic.combat.title')">
    <div class="ct-battle-heading"><span>{{ t('tower.classic.combat.title') }}</span><b>{{ t('tower.classic.combat.round', { n: current?.round ?? 1 }) }}</b></div>
    <div class="ct-duel">
      <div class="ct-fighter" :class="{ attacking: current?.attacker === 'hero', struck: current?.attacker === 'monster' && !current.missed }">
        <span>{{ heroName }}</span><div class="ct-fighter-art"><span class="ct-sprite" :style="sprite(hero)"></span><b v-if="current?.attacker === 'monster'" :key="index" class="ct-strike-number" :class="{ miss: current.missed, critical: current.critical }">{{ hitLabel }}</b><i v-if="current?.attacker === 'monster' && !current.missed" :key="`slash-${index}`" class="ct-slash"></i></div>
        <strong>{{ current?.heroHp ?? heroHp }} <small>HP</small></strong><progress :value="current?.heroHp ?? heroHp" :max="heroHp" :aria-label="heroName"></progress>
      </div>
      <span class="ct-versus">VS</span>
      <div class="ct-fighter ct-enemy-fighter" :class="{ attacking: current?.attacker === 'monster', struck: current?.attacker === 'hero' && !current.missed, defeated: finished }">
        <span>{{ monsterName }}</span><div class="ct-fighter-art"><span class="ct-sprite" :style="sprite(monster)"></span><b v-if="current?.attacker === 'hero'" :key="index" class="ct-strike-number" :class="{ miss: current.missed, critical: current.critical }">{{ hitLabel }}</b><i v-if="current?.attacker === 'hero' && !current.missed" :key="`slash-${index}`" class="ct-slash"></i></div>
        <strong>{{ current?.monsterHp ?? monsterHp }} <small>HP</small></strong><progress :value="current?.monsterHp ?? monsterHp" :max="monsterHp" :aria-label="monsterName"></progress>
      </div>
    </div>
    <div class="ct-battle-bottom"><p aria-live="polite">{{ finished ? t('tower.classic.combat.victory', { gold, exp }) : current?.critical ? t('tower.classic.combat.critical') : current?.missed ? t('tower.classic.combat.dodge') : t('tower.classic.combat.exchanging') }}<small v-if="current && current.hits > 1">{{ t('tower.classic.combat.combo', { n: current.hits }) }}</small></p><button class="ct-button" @click="emit('complete')">{{ t('tower.classic.combat.skip') }}</button></div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CombatFrame } from '@/games/token-tower/classic/combat'

const props = defineProps<{ frames: CombatFrame[]; hero: number; heroName: string; heroHp: number; monster: number; monsterName: string; monsterHp: number; gold: number; exp: number }>()
const emit = defineEmits<{ complete: []; health: [value: number] }>()
const { t } = useI18n()
const index = ref(-1), current = computed(() => props.frames[index.value]), finished = computed(() => current.value?.monsterHp === 0)
const hitLabel = computed(() => current.value?.missed ? t('tower.classic.combat.miss') : `−${current.value?.damage}`)
const sprite = (tile: number) => ({ backgroundPosition: `${tile % 16 / 15 * 100}% ${Math.floor(tile / 16) / 7 * 100}%` })
let timer: ReturnType<typeof setTimeout> | undefined
function advance() {
  index.value++
  if (index.value >= props.frames.length) { emit('complete'); return }
  emit('health', current.value!.heroHp)
  timer = setTimeout(advance, finished.value ? 280 : 140)
}
onMounted(() => { timer = setTimeout(advance, 180) })
onBeforeUnmount(() => clearTimeout(timer))
</script>

<style scoped src="@/games/token-tower/classic/battle.css"></style>
