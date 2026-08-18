<template>
  <div class="border-t border-gray-200 pt-3 dark:border-dark-600">
    <div class="flex flex-wrap items-center gap-2">
      <a
        href="/sub2api-credential-exporter.user.js"
        target="_blank"
        rel="noopener noreferrer"
        class="btn btn-secondary"
        :title="t('admin.sub2apiProviders.form.credentialScriptInstallTitle')"
      >
        <Icon name="download" size="sm" class="mr-1.5" />
        {{ t('admin.sub2apiProviders.form.installCredentialScript') }}
      </a>
      <button type="button" class="btn btn-secondary" @click="showImporter = !showImporter">
        <Icon name="clipboard" size="sm" class="mr-1.5" />
        {{ t('admin.sub2apiProviders.form.importCredentialBundle') }}
      </button>
    </div>

    <div
      class="mt-2 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      data-test="credential-session-warning"
    >
      <Icon name="exclamationTriangle" size="sm" class="mt-0.5 flex-shrink-0" />
      <span>{{ t('admin.sub2apiProviders.form.credentialSessionWarning') }}</span>
    </div>

    <div v-if="showImporter" class="mt-3 space-y-2">
      <div class="relative">
        <textarea
          v-model="bundleText"
          data-test="credential-bundle-input"
          rows="5"
          autocomplete="off"
          spellcheck="false"
          class="input min-h-28 resize-y pr-10 font-mono text-xs"
          :class="showSecrets ? '' : 'credential-bundle-masked'"
          :placeholder="t('admin.sub2apiProviders.form.credentialBundlePlaceholder')"
        ></textarea>
        <button
          type="button"
          class="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-dark-100"
          :title="showSecrets ? t('admin.sub2apiProviders.form.hideCredentialBundle') : t('admin.sub2apiProviders.form.showCredentialBundle')"
          :aria-label="showSecrets ? t('admin.sub2apiProviders.form.hideCredentialBundle') : t('admin.sub2apiProviders.form.showCredentialBundle')"
          @click="showSecrets = !showSecrets"
        >
          <Icon :name="showSecrets ? 'eyeOff' : 'eye'" size="sm" />
        </button>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary" @click="readClipboard">
          <Icon name="clipboard" size="sm" class="mr-1.5" />
          {{ t('admin.sub2apiProviders.form.readCredentialClipboard') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          data-test="credential-bundle-apply"
          :disabled="!bundleText.trim()"
          @click="applyBundle"
        >
          <Icon name="check" size="sm" class="mr-1.5" />
          {{ t('admin.sub2apiProviders.form.applyCredentialBundle') }}
        </button>
        <button v-if="bundleText" type="button" class="btn btn-secondary" @click="clearBundle">
          {{ t('admin.sub2apiProviders.form.clearCredentialBundle') }}
        </button>
      </div>
    </div>

    <div
      v-if="summary"
      class="mt-3 grid gap-2 border-l-2 border-green-500 bg-green-50 px-3 py-2 text-xs text-green-800 dark:bg-green-900/20 dark:text-green-200 sm:grid-cols-2"
      data-test="credential-bundle-summary"
    >
      <span class="truncate">{{ t('admin.sub2apiProviders.form.credentialSource') }}: {{ summary.sourceOrigin }}</span>
      <span v-if="summary.cookieCount === 0" data-test="credential-cookie-none">
        {{ t('admin.sub2apiProviders.form.credentialCookieNone') }}
      </span>
      <span v-else>
        {{ t('admin.sub2apiProviders.form.credentialCookieSummary', { count: summary.cookieCount, httpOnly: summary.httpOnlyCookieCount }) }}
      </span>
      <span v-if="summary.email" class="truncate">{{ t('admin.sub2apiProviders.form.email') }}: {{ summary.email }}</span>
      <span>{{ t('admin.sub2apiProviders.form.credentialCookiesNotStored') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import {
  parseSub2APICredentialBundle,
  Sub2APICredentialBundleError,
  type Sub2APICredentialBundle,
} from '@/utils/sub2apiCredentialBundle'

const props = defineProps<{
  expectedBaseUrl?: string
}>()

const emit = defineEmits<{
  imported: [bundle: Sub2APICredentialBundle]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const showImporter = ref(false)
const showSecrets = ref(false)
const bundleText = ref('')
const summary = ref<Sub2APICredentialBundle | null>(null)

const errorMessage = (error: unknown) => {
  if (error instanceof Sub2APICredentialBundleError) {
    return t(`admin.sub2apiProviders.form.credentialErrors.${error.code}`)
  }
  return t('admin.sub2apiProviders.form.credentialErrors.invalidFormat')
}

const applyBundle = () => {
  try {
    const parsed = parseSub2APICredentialBundle(bundleText.value, props.expectedBaseUrl)
    summary.value = parsed
    bundleText.value = ''
    showSecrets.value = false
    emit('imported', parsed)
    appStore.showSuccess(t('admin.sub2apiProviders.form.credentialBundleImported'))
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

const readClipboard = async () => {
  try {
    bundleText.value = await navigator.clipboard.readText()
    applyBundle()
  } catch {
    showImporter.value = true
    appStore.showError(t('admin.sub2apiProviders.form.credentialClipboardUnavailable'))
  }
}

const clearBundle = () => {
  bundleText.value = ''
  showSecrets.value = false
}
</script>

<style scoped>
.credential-bundle-masked {
  -webkit-text-security: disc;
}
</style>
