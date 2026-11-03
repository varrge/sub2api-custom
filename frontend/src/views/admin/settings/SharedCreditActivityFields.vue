<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

interface Props {
  name?: string;
  startsAt?: string | null;
  endsAt?: string | null;
  status?: string;
}

const props = withDefaults(defineProps<Props>(), {
  name: '',
  startsAt: null,
  endsAt: null,
  status: 'disabled',
});

const emit = defineEmits<{
  (e: 'update:name', value: string): void;
  (e: 'update:startsAt', value: string | null): void;
  (e: 'update:endsAt', value: string | null): void;
}>();

const { t } = useI18n();
const userTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

function isoToLocal(isoStr: string | null | undefined): string {
  if (!isoStr) return '';
  const date = new Date(isoStr);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function localToIso(localStr: string, originalIso: string | null | undefined): string | null {
  if (!localStr) return null;
  if (originalIso && isoToLocal(originalIso) === localStr) return originalIso;
  const date = new Date(localStr);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

const startsAtLocal = computed({
  get: () => isoToLocal(props.startsAt),
  set: (val: string) => emit('update:startsAt', localToIso(val, props.startsAt)),
});

const endsAtLocal = computed({
  get: () => isoToLocal(props.endsAt),
  set: (val: string) => emit('update:endsAt', localToIso(val, props.endsAt)),
});

const statusBadgeClass = computed(() => {
  switch (props.status) {
    case 'active':
      return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300 border-emerald-300 dark:border-emerald-800';
    case 'scheduled':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300 border-blue-300 dark:border-blue-800';
    case 'exhausted':
    case 'no_groups':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border-amber-300 dark:border-amber-800';
    case 'incomplete':
      return 'bg-rose-100 text-rose-800 dark:bg-rose-950/60 dark:text-rose-300 border-rose-300 dark:border-rose-800';
    case 'ended':
    case 'disabled':
    default:
      return 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 border-zinc-300 dark:border-zinc-700';
  }
});
</script>

<template>
  <div class="space-y-4 rounded-xl border border-zinc-200 bg-zinc-50/50 p-4 dark:border-zinc-800 dark:bg-zinc-900/40">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-2">
        <h4 class="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          {{ t('sharedCredit.activity.title') }}
        </h4>
        <span class="text-xs text-zinc-500 dark:text-zinc-400">({{ userTimeZone }})</span>
      </div>
      <div class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400">
        <span>{{ t('sharedCredit.activity.savedStatus') }}:</span>
        <span :class="['inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium', statusBadgeClass]">
          {{ t(`sharedCredit.activity.status.${props.status || 'disabled'}`) }}
        </span>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div class="sm:col-span-2">
        <label for="shared-credit-activity-name" class="block text-xs font-medium text-zinc-700 dark:text-zinc-300 mb-1">
          {{ t('sharedCredit.activity.name') }}
        </label>
        <input
          id="shared-credit-activity-name"
          type="text"
          maxlength="100"
          :value="props.name"
          :placeholder="t('sharedCredit.activity.namePlaceholder')"
          data-testid="shared-credit-activity-name"
          :aria-label="t('sharedCredit.activity.name')"
          class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder-zinc-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100 dark:placeholder-zinc-500"
          @input="emit('update:name', ($event.target as HTMLInputElement).value)"
        />
      </div>

      <div>
        <label for="shared-credit-starts-at" class="block text-xs font-medium text-zinc-700 dark:text-zinc-300 mb-1">
          {{ t('sharedCredit.activity.startsAt') }}
        </label>
        <input
          id="shared-credit-starts-at"
          v-model="startsAtLocal"
          type="datetime-local"
          data-testid="shared-credit-starts-at"
          :aria-label="t('sharedCredit.activity.startsAt')"
          class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100"
        />
      </div>

      <div>
        <label for="shared-credit-ends-at" class="block text-xs font-medium text-zinc-700 dark:text-zinc-300 mb-1">
          {{ t('sharedCredit.activity.endsAt') }}
        </label>
        <input
          id="shared-credit-ends-at"
          v-model="endsAtLocal"
          type="datetime-local"
          data-testid="shared-credit-ends-at"
          :aria-label="t('sharedCredit.activity.endsAt')"
          class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100"
        />
      </div>
    </div>

    <div class="rounded-lg border border-amber-200 bg-amber-50/80 p-3 text-xs leading-relaxed text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200 space-y-1">
      <p>{{ t('sharedCredit.activity.balanceNotice') }}</p>
      <p>{{ t('sharedCredit.activity.expiryNotice') }}</p>
    </div>
  </div>
</template>
