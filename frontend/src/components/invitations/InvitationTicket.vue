<template>
  <div class="rounded-2xl border border-primary-300 bg-primary-50 dark:border-primary-800 dark:bg-primary-900/20" data-testid="invitation-ticket">
    <div class="px-5 pb-5 pt-5 sm:px-6">
      <p class="text-sm text-primary-900/80 dark:text-primary-200/80">{{ t('invitations.issue.current') }}</p>
      <span v-if="invite.rebate_eligible === false" class="badge badge-primary mt-2" data-testid="invitation-no-rebate">{{ t('invitations.enterprise.noRebate') }}</span>
      <p class="mt-3 flex flex-wrap gap-x-2.5 gap-y-1 font-mono text-lg font-semibold text-primary-950 dark:text-primary-50 sm:text-xl" :aria-label="invite.code" data-testid="invitation-ticket-code">
        <span v-for="(group, index) in groups" :key="index" aria-hidden="true">{{ group }}</span>
      </p>
    </div>
    <div class="border-t-2 border-dashed border-primary-300 dark:border-primary-800" aria-hidden="true"></div>
    <div class="flex flex-wrap items-center justify-between gap-3 px-5 py-4 sm:px-6">
      <p class="text-sm text-primary-900 dark:text-primary-100">
        <span>{{ t('invitations.issue.expires', { date: formatDateTimeToMinute(invite.expires_at) }) }}</span>
        <span class="ml-2 font-medium" :class="daysLeft <= 1 ? 'text-amber-800 dark:text-amber-300' : ''" data-testid="invitation-ticket-left">
          {{ daysLeft <= 1 ? t('invitations.issue.lastDay') : t('invitations.issue.daysLeft', { days: daysLeft }) }}
        </span>
      </p>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary btn-sm" data-testid="copy-invitation-code" @click="copyToClipboard(invite.code, t('invitations.issue.codeCopied'))">
          <Icon name="copy" size="sm" />
          <span>{{ t('invitations.issue.copyCode') }}</span>
        </button>
        <button type="button" class="btn btn-primary btn-sm" data-testid="copy-invitation-link" @click="copyToClipboard(registrationInvitationLink(invite.code), t('invitations.issue.linkCopied'))">
          <Icon name="link" size="sm" />
          <span>{{ t('invitations.issue.copyLink') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { InvitationCode } from '@/api/invitations'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTimeToMinute } from '@/utils/format'
import { codeGroups } from '@/utils/invitationFormat'
import { registrationInvitationLink } from '@/utils/registrationInvitation'

const props = defineProps<{ invite: InvitationCode }>()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const groups = computed(() => codeGroups(props.invite.code))
const daysLeft = computed(() => Math.ceil((new Date(props.invite.expires_at).getTime() - Date.now()) / 86_400_000))
</script>
