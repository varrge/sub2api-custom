import { ref, watch } from 'vue'
import { getUserPrivileges } from '@/api/privileges'
import { useAuthStore } from '@/stores/auth'
import { useWelcomePlayback } from './useWelcomePlayback'

// Mounted once in App.vue: route changes and token refreshes do not replay it.
export function usePrivilegeWelcome() {
  const auth = useAuthStore()
  const playback = useWelcomePlayback()
  const title = ref<string | null>(null)

  watch(
    [() => auth.isAuthenticated, () => auth.user?.id, () => auth.user?.account_type, () => auth.user?.role],
    async ([authenticated, userID], _previous, onCleanup) => {
      playback.clear()
      title.value = null
      let current = true
      if (!authenticated || !userID) return
      const controller = new AbortController()
      onCleanup(() => {
        current = false
        controller.abort()
        title.value = null
      })
      try {
        const privileges = await getUserPrivileges(controller.signal)
        if (!current) return
        title.value = privileges.title || null
        if (privileges.welcome) playback.play(privileges.welcome.message, privileges.welcome.effect)
      } catch {
        // A decorative greeting must not interrupt login or ordinary page use.
      }
    },
    { immediate: true }
  )

  return { ...playback, title }
}
