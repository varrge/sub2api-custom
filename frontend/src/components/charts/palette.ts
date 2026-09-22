import { onMounted, onUnmounted, ref } from 'vue'

// Copper leads; muted secondary hues keep adjacent data series distinguishable.
export const chartPalette = [
  '#c9976f', '#839b93', '#8c91ac', '#beaa82', '#ac8d96', '#7e9eac',
  '#d7bca4', '#78867c', '#a5a4b7', '#b59875', '#8b717b', '#a1b7ba',
]

export function useChartDarkMode() {
  const isDark = ref(document.documentElement.classList.contains('dark'))
  let observer: MutationObserver | undefined
  onMounted(() => {
    observer = new MutationObserver(() => {
      isDark.value = document.documentElement.classList.contains('dark')
    })
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  })
  onUnmounted(() => observer?.disconnect())
  return isDark
}
