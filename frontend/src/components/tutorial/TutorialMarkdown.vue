<template>
  <!-- Only the constrained Markdown renderer can supply this HTML. -->
  <div class="tutorial-markdown" v-html="html" />
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { renderTutorialMarkdown } from '@/utils/tutorial'
const props = defineProps<{ content: string }>()
const html = computed(() => renderTutorialMarkdown(props.content))
</script>
<style scoped>
.tutorial-markdown { @apply min-w-0 break-words text-sm leading-7 text-gray-700 dark:text-gray-300; overflow-wrap: anywhere; }
.tutorial-markdown :deep(h1), .tutorial-markdown :deep(h2), .tutorial-markdown :deep(h3), .tutorial-markdown :deep(h4) { @apply mb-3 mt-5 font-semibold text-gray-900 dark:text-white; }
.tutorial-markdown :deep(h1) { @apply text-2xl; }
.tutorial-markdown :deep(h2) { @apply text-xl; }
.tutorial-markdown :deep(p) { @apply my-3; }
.tutorial-markdown :deep(a) { @apply text-primary-700 underline dark:text-primary-300; }
.tutorial-markdown :deep(pre) { @apply my-3 overflow-x-auto rounded-xl bg-gray-950 p-4 text-xs text-gray-100; }
.tutorial-markdown :deep(code) { @apply font-mono; }
.tutorial-markdown :deep(img) { @apply my-4 h-auto max-w-full rounded-xl; }
.tutorial-markdown :deep(ul) { @apply list-disc pl-6; }
.tutorial-markdown :deep(ol) { @apply list-decimal pl-6; }
.tutorial-markdown :deep(blockquote) { @apply border-l-4 border-primary-300 pl-4; }
.tutorial-markdown :deep(table) { @apply block max-w-full overflow-x-auto border-collapse; }
.tutorial-markdown :deep(td), .tutorial-markdown :deep(th) { @apply border border-gray-200 px-3 py-2 dark:border-gray-700; }
</style>
