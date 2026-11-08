<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-4">
      <header class="border-b border-gray-200 pb-4 dark:border-dark-700">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">留言板</h1>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">提交问题或建议，管理员回复后会显示在这里。</p>
      </header>
      <FeedbackBoard />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import FeedbackBoard from '@/components/feedback/FeedbackBoard.vue'
</script>
