<template>
  <section v-if="items.length" class="card space-y-4 p-5">
    <button
      class="flex w-full items-center justify-between text-left font-semibold"
      :aria-expanded="open"
      @click="open = !open"
    >
      {{ t('groupBuy.sort') }}
      <span aria-hidden="true">{{ open ? '−' : '+' }}</span>
    </button>
    <template v-if="open">
      <p class="text-sm leading-6 text-gray-500">
        {{ t('groupBuy.sortHint') }}
      </p>
      <div v-for="group in groups" :key="group.id" class="space-y-2">
        <h3 class="text-sm font-semibold">{{ group.name }}</h3>
        <ol class="space-y-2">
          <li
            v-for="(item, index) in group.items"
            :key="refKey(item)"
            class="flex items-center gap-3 rounded-xl border border-gray-100 p-3 dark:border-dark-700"
            :draggable="!saving"
            @dragstart="dragging = { group: group.id, key: refKey(item) }"
            @dragover.prevent
            @drop.prevent="drop(group.id, index)"
          >
            <span class="text-sm text-gray-400">{{ index + 1 }}</span>
            <div class="min-w-0 flex-1">
              <p class="break-all text-sm">
                {{ item.name }}
                <span v-if="item.kind === 'legacy'" class="badge badge-gray">
                  {{ t('groupBuy.legacy') }}
                </span>
                <span v-if="item.card?.status === 'frozen'" class="badge bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-200">{{ t('groupBuy.frozenInOrder') }}</span>
              </p>
              <p v-if="index === 0 && item.card?.status !== 'frozen'" class="text-xs text-primary-600">
                {{ t('groupBuy.first') }}
              </p>
            </div>
            <button
              class="btn btn-secondary px-2"
              :aria-label="t('groupBuy.up')"
              :disabled="index === 0 || saving"
              @click="move(group.id, index, index - 1)"
            >
              ↑
            </button>
            <button
              class="btn btn-secondary px-2"
              :aria-label="t('groupBuy.down')"
              :disabled="index === group.items.length - 1 || saving"
              @click="move(group.id, index, index + 1)"
            >
              ↓
            </button>
          </li>
        </ol>
        <button
          v-if="dirty.has(group.id)"
          class="btn btn-primary"
          :disabled="saving"
          @click="save(group.id)"
        >
          {{ t('common.save') }}
        </button>
      </div>
      <button
        v-if="dirty.size"
        class="btn btn-secondary"
        :disabled="saving"
        @click="cancelChanges"
      >
        {{ t('common.cancel') }}
      </button>
      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import { useAppStore } from '@/stores/app'
import { refKey, type EntitlementItem } from './model'
const props = defineProps<{ items: EntitlementItem[] }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const app = useAppStore()
const open = ref(false)
const saving = ref(false)
const error = ref('')
const draft = ref<EntitlementItem[]>([])
const dirty = ref(new Set<number>())
function cancelChanges() {
  draft.value = [...props.items]
  dirty.value = new Set()
  error.value = ''
}
const dragging = ref<{ group: number; key: string } | null>(null)
watch(
  () => props.items,
  (value) => {
    if (!dirty.value.size) draft.value = [...value]
  },
  { immediate: true }
)
const groups = computed(() =>
  Array.from(new Set(draft.value.map((item) => item.group_id))).map((id) => {
    const items = draft.value.filter((item) => item.group_id === id)
    return {
      id,
      name:
        items[0]?.card?.group_name || items[0]?.legacy?.group?.name || `#${id}`,
      items
    }
  })
)
function move(groupId: number, from: number, to: number) {
  const group = groups.value.find((group) => group.id === groupId)
  if (!group || to < 0 || to >= group.items.length) return
  const next = [...group.items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  draft.value = [
    ...draft.value.filter((item) => item.group_id !== groupId),
    ...next
  ].sort((a, b) => a.group_id - b.group_id)
  dirty.value = new Set(dirty.value).add(groupId)
}
function drop(groupId: number, index: number) {
  if (dragging.value?.group !== groupId) return
  const from =
    groups.value
      .find((group) => group.id === groupId)
      ?.items.findIndex((item) => refKey(item) === dragging.value?.key) ?? -1
  if (from >= 0) move(groupId, from, index)
  dragging.value = null
}
async function save(groupId: number) {
  saving.value = true
  error.value = ''
  try {
    await groupBuyAPI.setOrder({
      group_id: groupId,
      items: draft.value
        .filter((item) => item.group_id === groupId)
        .map(({ kind, id }) => ({ kind, id }))
    })
    dirty.value.delete(groupId)
    app.showSuccess(t('groupBuy.saved'))
    emit('saved')
  } catch {
    error.value = t('groupBuy.orderChanged')
  } finally {
    saving.value = false
  }
}
</script>
