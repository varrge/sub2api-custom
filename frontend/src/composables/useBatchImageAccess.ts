import { computed, ref } from 'vue'
import { keysAPI } from '@/api/keys'
import { supportsImageGeneration } from '@/api/imageGeneration'
import { useAuthStore } from '@/stores/auth'
import type { ApiKey } from '@/types'

const loaded = ref(false)
const loading = ref(false)
const hasAllowedBatchImageKey = ref(false)
const hasAllowedImageGenerationKey = ref(false)
let pendingLoad: Promise<boolean> | null = null
const pageSize = 100

function keyAllowsBatchImage(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    key.group?.platform === 'gemini' &&
    key.group?.allow_batch_image_generation === true
  )
}

function keyAllowsImageGeneration(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    key.group?.allow_image_generation === true &&
    supportsImageGeneration(key.group.platform)
  )
}

async function loadBatchImageAccess(force = false): Promise<boolean> {
  const authStore = useAuthStore()
  if (!authStore.isAuthenticated) {
    loaded.value = true
    hasAllowedBatchImageKey.value = false
    hasAllowedImageGenerationKey.value = false
    return false
  }

  if (loaded.value && !force) {
    return hasAllowedBatchImageKey.value
  }

  if (pendingLoad && !force) {
    return pendingLoad
  }

  loading.value = true
  pendingLoad = (async () => {
    let page = 1
    let batchAccess = false
    let imageAccess = false
    while (true) {
      const response = await keysAPI.list(page, pageSize, {
        status: 'active',
        sort_by: 'created_at',
        sort_order: 'desc'
      })
      const items = response.items || []
      batchAccess ||= items.some(keyAllowsBatchImage)
      imageAccess ||= items.some(keyAllowsImageGeneration)

      if ((batchAccess && imageAccess) || page >= response.pages || items.length === 0) {
        hasAllowedBatchImageKey.value = batchAccess
        hasAllowedImageGenerationKey.value = imageAccess
        loaded.value = true
        return batchAccess
      }

      page += 1
    }
  })()
    .catch(() => {
      hasAllowedBatchImageKey.value = false
      hasAllowedImageGenerationKey.value = false
      loaded.value = true
      return false
    })
    .finally(() => {
      loading.value = false
      pendingLoad = null
    })

  return pendingLoad
}

export function useBatchImageAccess() {
  const canUseBatchImage = computed(() => hasAllowedBatchImageKey.value)

  return {
    canUseBatchImage,
    batchImageAccessLoaded: computed(() => loaded.value),
    batchImageAccessLoading: computed(() => loading.value),
    refreshBatchImageAccess: loadBatchImageAccess,
  }
}

export function useImageGenerationAccess() {
  const canUseImageGeneration = computed(() => hasAllowedImageGenerationKey.value)

  return {
    canUseImageGeneration,
    imageGenerationAccessLoaded: computed(() => loaded.value),
    imageGenerationAccessLoading: computed(() => loading.value),
    refreshImageGenerationAccess: async (force = false) => {
      await loadBatchImageAccess(force)
      return hasAllowedImageGenerationKey.value
    },
  }
}
