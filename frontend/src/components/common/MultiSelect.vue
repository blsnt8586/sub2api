<template>
  <div ref="containerRef" class="relative min-w-0">
    <button
      ref="triggerRef"
      type="button"
      class="multi-select-trigger"
      :class="isOpen ? 'multi-select-trigger-open' : ''"
      :disabled="disabled"
      :aria-expanded="isOpen"
      aria-haspopup="listbox"
      :aria-label="ariaLabel || placeholder"
      @click="toggle"
      @keydown.down.prevent="open"
      @keydown.enter.prevent="toggle"
      @keydown.space.prevent="toggle"
    >
      <span class="multi-select-value">
        <span v-if="selectedValues.length === 0" class="text-gray-500 dark:text-dark-300">{{ placeholder }}</span>
        <span v-else class="inline-flex min-w-0 items-center gap-1.5">
          <span class="multi-select-count">{{ selectedValues.length }}</span>
          <span class="truncate">{{ selectedSummary }}</span>
        </span>
      </span>
      <Icon name="chevronDown" size="sm" class="flex-shrink-0 text-gray-400 transition-transform duration-150" :class="isOpen ? 'rotate-180' : ''" />
    </button>

    <Teleport to="body">
      <Transition name="select-dropdown">
        <div v-if="isOpen" ref="dropdownRef" class="multi-select-dropdown" :style="dropdownStyle" role="listbox" aria-multiselectable="true" @click.stop @mousedown.stop>
          <div v-if="searchable" class="multi-select-search">
            <Icon name="search" size="xs" class="flex-shrink-0 text-gray-400" />
            <input ref="searchInputRef" v-model="searchQuery" type="text" :placeholder="searchPlaceholder" :aria-label="searchPlaceholder" class="multi-select-search-input" @click.stop />
          </div>
          <div class="multi-select-options">
            <button
              v-for="option in filteredOptions"
              :key="String(option.value)"
              type="button"
              class="multi-select-option"
              :class="[isSelected(option) ? 'multi-select-option-selected' : '', option.disabled ? 'multi-select-option-disabled' : '']"
              role="option"
              :aria-selected="isSelected(option)"
              :disabled="option.disabled"
              @click="toggleOption(option)"
            >
              <span class="multi-select-checkbox" :class="isSelected(option) ? 'multi-select-checkbox-selected' : ''">
                <Icon v-if="isSelected(option)" name="check" size="xs" aria-hidden="true" />
              </span>
              <span class="min-w-0 flex-1 truncate text-left">{{ option.label }}</span>
            </button>
            <p v-if="filteredOptions.length === 0" class="px-3 py-5 text-center text-xs text-gray-400 dark:text-dark-400">{{ emptyText }}</p>
          </div>
          <div v-if="selectedValues.length > 0" class="multi-select-footer">
            <span>{{ selectedValues.length }} {{ selectedCountLabel }}</span>
            <button type="button" class="multi-select-clear" @click="clear">{{ clearLabel }}</button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'

export interface MultiSelectOption {
  value: number
  label: string
  disabled?: boolean
}

const props = withDefaults(defineProps<{
  modelValue: number[]
  options: MultiSelectOption[]
  placeholder: string
  searchPlaceholder?: string
  emptyText?: string
  selectedCountLabel?: string
  clearLabel?: string
  ariaLabel?: string
  disabled?: boolean
  searchable?: boolean
}>(), {
  searchPlaceholder: 'Search',
  emptyText: 'No options',
  selectedCountLabel: 'selected',
  clearLabel: 'Clear',
  disabled: false,
  searchable: true,
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: number[]): void
  (event: 'change', value: number[]): void
}>()

const isOpen = ref(false)
const searchQuery = ref('')
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const triggerRect = ref<DOMRect | null>(null)
const dropdownPosition = ref<'bottom' | 'top'>('bottom')

const selectedValues = computed(() => [...new Set(props.modelValue.filter(value => Number.isSafeInteger(value) && value > 0))])
const selectedOptions = computed(() => props.options.filter(option => selectedValues.value.includes(option.value)))
const selectedSummary = computed(() => {
  if (selectedOptions.value.length === 0) return props.placeholder
  if (selectedOptions.value.length === 1) return selectedOptions.value[0].label
  return selectedOptions.value.slice(0, 2).map(option => option.label).join(', ')
})
const filteredOptions = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return props.options
  return props.options.filter(option => option.label.toLowerCase().includes(query))
})
const isSelected = (option: MultiSelectOption) => selectedValues.value.includes(option.value)

const updateTriggerRect = () => {
  triggerRect.value = containerRef.value?.getBoundingClientRect() ?? null
}

const dropdownStyle = computed(() => {
  const rect = triggerRect.value
  if (!rect) return {}
  const padding = 8
  const width = Math.min(Math.max(rect.width, 230), window.innerWidth - padding * 2)
  const left = Math.min(Math.max(padding, rect.left), window.innerWidth - padding - width)
  const style: Record<string, string> = { left: `${left}px`, width: `${width}px`, zIndex: '100000020' }
  if (dropdownPosition.value === 'top') style.bottom = `${window.innerHeight - rect.top + 4}px`
  else style.top = `${rect.bottom + 4}px`
  return style
})

const calculatePosition = () => {
  updateTriggerRect()
  nextTick(() => {
    const rect = triggerRect.value
    const dropdownHeight = dropdownRef.value?.offsetHeight || 260
    dropdownPosition.value = rect && window.innerHeight - rect.bottom < dropdownHeight && rect.top > dropdownHeight ? 'top' : 'bottom'
  })
}

const open = () => {
  if (props.disabled) return
  isOpen.value = true
}
const toggle = () => {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}
const toggleOption = (option: MultiSelectOption) => {
  if (option.disabled) return
  const next = isSelected(option)
    ? selectedValues.value.filter(value => value !== option.value)
    : [...selectedValues.value, option.value].sort((a, b) => a - b)
  emit('update:modelValue', next)
  emit('change', next)
}
const clear = () => {
  emit('update:modelValue', [])
  emit('change', [])
}
const handleOutside = (event: MouseEvent) => {
  const target = event.target as Node
  if (isOpen.value && !containerRef.value?.contains(target) && !dropdownRef.value?.contains(target)) isOpen.value = false
}

watch(isOpen, openState => {
  if (openState) {
    calculatePosition()
    window.addEventListener('resize', calculatePosition)
    window.addEventListener('scroll', updateTriggerRect, true)
    nextTick(() => { if (props.searchable) searchInputRef.value?.focus() })
  } else {
    searchQuery.value = ''
    window.removeEventListener('resize', calculatePosition)
    window.removeEventListener('scroll', updateTriggerRect, true)
  }
})

onMounted(() => document.addEventListener('mousedown', handleOutside))
onUnmounted(() => {
  document.removeEventListener('mousedown', handleOutside)
  window.removeEventListener('resize', calculatePosition)
  window.removeEventListener('scroll', updateTriggerRect, true)
})
</script>

<style scoped>
.multi-select-trigger {
  align-items: center;
  background: white;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.375rem;
  color: rgb(51 65 85);
  display: flex;
  font-size: 0.75rem;
  gap: 0.5rem;
  justify-content: space-between;
  line-height: 1.125rem;
  min-height: 1.75rem;
  min-width: 0;
  padding: 0.25rem 0.5rem;
  text-align: left;
  transition: border-color 150ms, box-shadow 150ms;
  width: 100%;
}
.multi-select-trigger:hover { border-color: rgb(148 163 184); }
.multi-select-trigger:focus-visible { outline: none; border-color: rgb(59 130 246); box-shadow: 0 0 0 2px rgb(59 130 246 / 0.2); }
.multi-select-trigger-open { border-color: rgb(59 130 246); box-shadow: 0 0 0 2px rgb(59 130 246 / 0.15); }
.multi-select-trigger:disabled { cursor: not-allowed; opacity: 0.55; }
.multi-select-value { min-width: 0; flex: 1; }
.multi-select-count { align-items: center; background: rgb(219 234 254); border-radius: 999px; color: rgb(30 64 175); display: inline-flex; font-size: 0.6875rem; font-weight: 700; height: 1.125rem; justify-content: center; min-width: 1.125rem; padding: 0 0.25rem; }
:global(.dark) .multi-select-trigger { background: rgb(30 41 59); border-color: rgb(71 85 105); color: rgb(226 232 240); }
@media (max-width: 767px) {
  .multi-select-trigger {
    min-height: 2.75rem;
    padding: 0.375rem 0.5rem;
  }
}
:global(.dark) .multi-select-trigger-open { border-color: rgb(96 165 250); }
:global(.dark) .multi-select-count { background: rgb(30 64 175 / 0.45); color: rgb(191 219 254); }
</style>

<style>
.multi-select-dropdown { background: white; border: 1px solid rgb(226 232 240); border-radius: 0.5rem; box-shadow: 0 12px 30px rgb(15 23 42 / 0.16); max-height: min(22rem, calc(100vh - 1rem)); overflow: hidden; position: fixed; }
.multi-select-search { align-items: center; border-bottom: 1px solid rgb(241 245 249); display: flex; gap: 0.375rem; padding: 0.5rem 0.625rem; }
.multi-select-search-input { background: transparent; color: rgb(15 23 42); flex: 1; font-size: 0.75rem; min-width: 0; outline: none; }
.multi-select-options { max-height: 17rem; overflow-y: auto; padding: 0.25rem; }
.multi-select-option { align-items: center; background: transparent; border-radius: 0.375rem; color: rgb(51 65 85); cursor: pointer; display: flex; font-size: 0.75rem; gap: 0.5rem; padding: 0.5rem; text-align: left; transition: background-color 120ms; width: 100%; }
.multi-select-option:hover, .multi-select-option-selected { background: rgb(239 246 255); color: rgb(30 64 175); }
.multi-select-option-disabled { cursor: not-allowed; opacity: 0.45; }
.multi-select-checkbox { align-items: center; border: 1px solid rgb(148 163 184); border-radius: 0.25rem; display: inline-flex; flex-shrink: 0; height: 0.875rem; justify-content: center; width: 0.875rem; }
.multi-select-checkbox-selected { background: rgb(37 99 235); border-color: rgb(37 99 235); color: white; }
.multi-select-footer { align-items: center; border-top: 1px solid rgb(241 245 249); color: rgb(100 116 139); display: flex; font-size: 0.6875rem; justify-content: space-between; padding: 0.375rem 0.625rem; }
.multi-select-clear { color: rgb(37 99 235); cursor: pointer; font-weight: 600; }
.multi-select-clear:hover { text-decoration: underline; }
:global(.dark) .multi-select-dropdown { background: rgb(30 41 59); border-color: rgb(71 85 105); box-shadow: 0 12px 30px rgb(0 0 0 / 0.35); }
:global(.dark) .multi-select-search { border-color: rgb(51 65 85); }
:global(.dark) .multi-select-search-input { color: rgb(226 232 240); }
:global(.dark) .multi-select-option { color: rgb(203 213 225); }
:global(.dark) .multi-select-option:hover, :global(.dark) .multi-select-option-selected { background: rgb(30 64 175 / 0.3); color: rgb(191 219 254); }
:global(.dark) .multi-select-footer { border-color: rgb(51 65 85); color: rgb(148 163 184); }
</style>
