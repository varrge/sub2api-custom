<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ localText('特权设置', 'Privilege Settings') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ localText(
          '以下为展示特权设置，仅影响界面呈现，不会修改企业身份、分组权限或计费倍率。',
          'These are display privileges only. They do not change enterprise identity, group permissions, or billing multipliers.'
        ) }}
      </p>
    </div>

    <div class="space-y-8 p-6">
      <div
        v-if="loading"
        class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400"
      >
        <span class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></span>
        {{ localText('加载中…', 'Loading…') }}
      </div>

      <div
        v-else-if="loadError"
        class="flex flex-wrap items-center gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300"
      >
        <span>{{ loadError }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">
          {{ localText('重试', 'Retry') }}
        </button>
      </div>

      <template v-else-if="settings">
        <!-- 企业用户 -->
        <section class="space-y-6">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ localText('企业用户', 'Enterprise users') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ localText(
                '企业身份由管理员在用户管理中设置，此处仅配置欢迎提示。',
                'Enterprise identity is set by admins in user management; only the welcome message is configured here.'
              ) }}
            </p>
          </div>

          <div class="flex items-center justify-between gap-4">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ localText('企业用户欢迎提示', 'Enterprise welcome message') }}
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ localText(
                  '开启后，企业用户每次整页打开或刷新时会在顶部看到一次欢迎提示。',
                  'When enabled, enterprise users see a welcome capsule at the top once per full page load or refresh.'
                ) }}
              </p>
            </div>
            <span data-testid="enterprise-welcome-enabled">
              <Toggle
                v-model="settings.enterprise_welcome.enabled"
                :disabled="saving"
                :aria-label="localText('企业用户欢迎提示', 'Enterprise welcome message')"
              />
            </span>
          </div>

          <div>
            <label class="input-label" for="enterprise-welcome-message">
              {{ localText('欢迎文案', 'Welcome message') }}
            </label>
            <input
              id="enterprise-welcome-message"
              v-model="settings.enterprise_welcome.message"
              type="text"
              class="input"
              data-testid="enterprise-welcome-message"
              :disabled="saving"
              :aria-invalid="Boolean(enterpriseValidationError)"
              :aria-describedby="enterpriseValidationError ? 'enterprise-welcome-message-error' : 'enterprise-welcome-message-hint'"
              :placeholder="localText('欢迎尊贵的企业用户', 'Welcome, valued enterprise user')"
            />
            <div class="mt-1.5 flex items-center justify-between gap-3">
              <p v-if="enterpriseValidationError" id="enterprise-welcome-message-error" class="input-error-text">
                {{ enterpriseValidationError }}
              </p>
              <p v-else id="enterprise-welcome-message-hint" class="input-hint">
                {{ localText('最多 80 个字符，单行展示。', 'Up to 80 characters, shown on a single line.') }}
              </p>
              <span class="flex-shrink-0 text-xs text-gray-400 dark:text-gray-500">
                {{ messageLength }}/80
              </span>
            </div>
          </div>

          <div class="space-y-2">
            <div class="input-label">{{ localText('欢迎动效', 'Welcome effect') }}</div>
            <WelcomeEffectPicker
              v-model="settings.enterprise_welcome.effect"
              data-testid="enterprise-welcome-effect"
              :disabled="fieldsBusy"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ localText(
                '选择仅修改草稿，点击「预览动效」查看，保存后生效。每次整页打开或刷新展示一次，停留约 3.5 秒，可手动关闭；站内切页不重播。',
                'Selection changes the draft only. Preview below and save to apply. Shown once per full page load for about 3.5 seconds; dismissible, without replay on in-app navigation.'
              ) }}
            </p>
          </div>
        </section>

        <!-- 普通用户 -->
        <section class="space-y-4">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ localText('普通用户头衔规则', 'Personal user title rules') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ localText(
                  '规则按列表从上到下依次匹配，同时满足多条仅显示最靠前一条；如设置消费档位，请把高档排在上方。保存前的删除与排序都只是草稿，点「保存特权设置」后生效。',
                  'Rules are matched top to bottom; when several match, only the first shows. If you set spend tiers, put higher tiers above. Deletions and reordering are drafts until you press "Save privilege settings".'
                ) }}
              </p>
            </div>
            <button
              v-if="settings.personal_rules.length"
              type="button"
              class="btn btn-secondary btn-sm"
              data-testid="personal-rule-add"
              :disabled="fieldsBusy || settings.personal_rules.length >= 50"
              @click="addRule"
            >
              {{ localText('添加规则', 'Add rule') }}
            </button>
          </div>

          <div
            v-if="!settings.personal_rules.length"
            class="rounded-xl border border-dashed border-gray-300 p-6 text-center dark:border-dark-600"
          >
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ localText(
                '暂无普通用户特权规则，可点击下面按钮添加；也可以留空，表示不为普通用户展示头衔。',
                'No personal user rules yet. Add one below, or leave empty to show no title to personal users.'
              ) }}
            </p>
            <button
              type="button"
              class="btn btn-secondary btn-sm mt-3"
              data-testid="personal-rule-add"
              :disabled="fieldsBusy"
              @click="addRule"
            >
              {{ localText('添加规则', 'Add rule') }}
            </button>
          </div>

          <div
            v-for="(rule, index) in settings.personal_rules"
            :key="rule.id"
            data-testid="personal-rule"
            class="space-y-4 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60"
          >
            <div class="flex flex-wrap items-center gap-3">
              <span class="w-5 text-center text-xs font-medium text-gray-400 dark:text-gray-500">
                {{ index + 1 }}
              </span>
              <div class="min-w-[200px] flex-1">
                <label class="input-label" :for="`personal-rule-title-${rule.id}`">
                  {{ localText('头衔名称', 'Title') }}
                </label>
                <input
                  :id="`personal-rule-title-${rule.id}`"
                  v-model="rule.title"
                  type="text"
                  class="input"
                  data-testid="personal-rule-title"
                  :disabled="fieldsBusy"
                  :aria-invalid="Boolean(ruleErrors[rule.id])"
                  :aria-describedby="ruleErrors[rule.id] ? `personal-rule-error-${rule.id}` : undefined"
                  :placeholder="localText('例如：资深用户', 'e.g. Veteran user')"
                />
              </div>
              <div class="flex items-center gap-2">
                <span data-testid="personal-rule-enabled">
                  <Toggle
                    v-model="rule.enabled"
                    :disabled="fieldsBusy"
                    :aria-label="localText('启用规则', 'Enable rule')"
                  />
                </span>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ localText('启用', 'Enabled') }}
                </span>
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  data-testid="personal-rule-up"
                  :disabled="fieldsBusy || index === 0"
                  :aria-label="localText('上移', 'Move up')"
                  @click="moveRule(index, -1)"
                >
                  {{ localText('上移', 'Up') }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  data-testid="personal-rule-down"
                  :disabled="fieldsBusy || index === settings.personal_rules.length - 1"
                  :aria-label="localText('下移', 'Move down')"
                  @click="moveRule(index, 1)"
                >
                  {{ localText('下移', 'Down') }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  data-testid="personal-rule-delete"
                  :disabled="fieldsBusy"
                  :aria-label="localText('删除规则', 'Delete rule')"
                  @click="removeRule(index)"
                >
                  {{ localText('删除', 'Delete') }}
                </button>
              </div>
            </div>

            <p
              v-if="ruleErrors[rule.id]"
              :id="`personal-rule-error-${rule.id}`"
              class="input-error-text"
            >
              {{ ruleErrors[rule.id] }}
            </p>

            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label class="input-label" :for="`personal-rule-kind-${rule.id}`">
                  {{ localText('条件类型', 'Condition type') }}
                </label>
                <select
                  :id="`personal-rule-kind-${rule.id}`"
                  class="input"
                  data-testid="personal-rule-kind"
                  :value="rule.kind"
                  :disabled="fieldsBusy"
                  @change="onKindChange(index, $event)"
                >
                  <option
                    v-for="option in kindOptions"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ localText(option.zh, option.en) }}
                  </option>
                </select>
              </div>
              <div v-if="rule.kind !== 'manual'">
                <label class="input-label" :for="`personal-rule-threshold-${rule.id}`">
                  {{ thresholdLabel(rule.kind) }}
                </label>
                <input
                  :id="`personal-rule-threshold-${rule.id}`"
                  v-model.number="rule.threshold"
                  type="number"
                  class="input"
                  data-testid="personal-rule-threshold"
                  :disabled="fieldsBusy"
                  :aria-invalid="Boolean(ruleErrors[rule.id])"
                  :aria-describedby="ruleErrors[rule.id] ? `personal-rule-error-${rule.id}` : undefined"
                  :min="rule.kind === 'account_age_days' ? 1 : undefined"
                  :step="rule.kind === 'account_age_days' ? 1 : 'any'"
                />
              </div>
            </div>

            <div v-if="rule.kind === 'manual'">
              <label class="input-label" :for="`personal-rule-user-ids-${rule.id}`">
                {{ localText('用户 ID', 'User IDs') }}
              </label>
              <textarea
                :id="`personal-rule-user-ids-${rule.id}`"
                v-model="userIDsText[rule.id]"
                class="input"
                rows="2"
                data-testid="personal-rule-user-ids"
                :disabled="fieldsBusy"
                :aria-invalid="Boolean(ruleErrors[rule.id])"
                :aria-describedby="ruleErrors[rule.id] ? `personal-rule-error-${rule.id}` : undefined"
                :placeholder="localText('例如：101, 102, 103', 'e.g. 101, 102, 103')"
              ></textarea>
              <p class="input-hint mt-1">
                {{ localText(
                  '可在用户管理查看 ID，只允许现有普通用户；用逗号或空格分隔，最多 200 个。',
                  'Find IDs in user management; existing personal users only. Separate with commas or spaces, up to 200.'
                ) }}
              </p>
            </div>

            <div class="flex flex-wrap items-end gap-3">
              <div class="flex items-center gap-2 pb-2.5">
                <span data-testid="personal-rule-welcome-enabled">
                  <Toggle
                    v-model="rule.welcome_enabled"
                    :disabled="fieldsBusy"
                    :aria-label="localText('欢迎动效', 'Welcome animation')"
                  />
                </span>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ localText('欢迎动效', 'Welcome animation') }}
                </span>
              </div>
              <div class="min-w-[200px] flex-1">
                <label class="input-label" :for="`personal-rule-welcome-message-${rule.id}`">
                  {{ localText('欢迎文案', 'Welcome message') }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    :id="`personal-rule-welcome-message-${rule.id}`"
                    v-model="rule.welcome_message"
                    type="text"
                    class="input"
                    data-testid="personal-rule-welcome-message"
                    :disabled="fieldsBusy || !rule.welcome_enabled"
                    :aria-invalid="Boolean(ruleErrors[rule.id])"
                    :aria-describedby="ruleErrors[rule.id] ? `personal-rule-error-${rule.id}` : undefined"
                    :placeholder="localText('欢迎回来', 'Welcome back')"
                  />
                  <span class="flex-shrink-0 text-xs text-gray-400 dark:text-gray-500">
                    {{ Array.from(rule.welcome_message ?? '').length }}/80
                  </span>
                </div>
              </div>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                data-testid="personal-rule-preview"
                :disabled="fieldsBusy || !rule.welcome_enabled || Boolean(ruleErrors[rule.id])"
                @click="showRulePreview(index)"
              >
                {{ localText('预览动效', 'Preview animation') }}
              </button>
            </div>
            <div class="space-y-2">
              <div class="input-label">{{ localText('欢迎动效', 'Welcome effect') }}</div>
              <WelcomeEffectPicker
                v-model="rule.welcome_effect"
                data-testid="personal-rule-welcome-effect"
                :disabled="fieldsBusy || !rule.welcome_enabled"
              />
            </div>

            <p class="input-hint">
              {{ localText(
                '最多 80 个字符，开启欢迎动效时必填；关闭欢迎动效后仍会展示头衔。',
                'Up to 80 characters, required when the welcome animation is on; the title still shows when it is off.'
              ) }}
            </p>
          </div>
        </section>

        <!-- 数据口径说明 -->
        <section
          class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-xs leading-relaxed text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400"
        >
          <div class="mb-1.5 text-sm font-medium text-gray-900 dark:text-white">
            {{ localText('数据口径', 'How the numbers are computed') }}
          </div>
          <ul class="list-disc space-y-1 pl-5">
            <li>
              {{ localText(
                '消费以当前保留的使用记录 actual_cost 汇总，包含套餐抵扣的 API 用量，不是充值额；已清理的历史记录无法补算。',
                'Spending sums actual_cost over retained usage records, including API usage covered by plans, not recharge amounts; purged history cannot be recomputed.'
              ) }}
            </li>
            <li>
              {{ localText(
                '充值仅已完成余额充值的入账金额，包含当时充值倍率，扣除成功退款；不含赠送、返佣、兑换码、订阅/月卡或月卡折算，不是支付渠道现金金额。',
                'Recharge counts completed balance top-ups only, including the multiplier at the time, minus successful refunds; it excludes gifts, rebates, redeem codes, subscriptions/monthly cards or their conversion, and is not the cash amount paid via the payment channel.'
              ) }}
            </li>
            <li>
              {{ localText(
                '注册天数按注册时间到现在的完整 24 小时计算。',
                'Account age counts complete 24-hour periods from registration to now.'
              ) }}
            </li>
            <li>
              {{ localText(
                '刷新或重新登录会重新匹配；规则关闭/删除、退款或数据变化都可能改变头衔，头衔不是永久授予。头衔不改变企业身份、计费倍率或分组权限。',
                'Refreshing or signing in again re-matches the rules; disabling/deleting a rule, refunds, or data changes can change the title, which is never permanent. Titles do not change enterprise identity, billing multipliers, or group permissions.'
              ) }}
            </li>
          </ul>
        </section>

        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            data-testid="privilege-preview"
            :disabled="previewDisabled"
            @click="showPreview"
          >
            {{ localText('预览动效', 'Preview animation') }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            data-testid="privilege-save"
            :disabled="saveDisabled"
            @click="save"
          >
            {{ saving ? localText('保存中…', 'Saving…') : localText('保存特权设置', 'Save privilege settings') }}
          </button>
        </div>
      </template>
    </div>

    <PrivilegeWelcome
      v-if="previewMessage"
      :message="previewMessage"
      :effect="previewEffect"
      :phase="previewPhase"
      @close="previewDismiss"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import PrivilegeWelcome from '@/components/common/PrivilegeWelcome.vue'
import WelcomeEffectPicker from '@/components/common/WelcomeEffectPicker.vue'
import { usePrivilegeSettings } from '@/composables/usePrivilegeSettings'
import type { PrivilegeRuleKind } from '@/api/privileges'

const { locale } = useI18n()
const localText = (zh: string, en: string) => (locale.value.toLowerCase().startsWith('zh') ? zh : en)

const {
  settings,
  loading,
  saving,
  loadError,
  validationError,
  enterpriseValidationError,
  ruleErrors,
  userIDsText,
  load,
  save,
  showPreview,
  showRulePreview,
  addRule,
  removeRule,
  moveRule,
  changeRuleKind,
  preview
} = usePrivilegeSettings()

const { message: previewMessage, effect: previewEffect, phase: previewPhase, dismiss: previewDismiss } = preview

const kindOptions: { value: PrivilegeRuleKind; zh: string; en: string }[] = [
  { value: 'total_spend', zh: '累计 API 消费（USD）', en: 'Total API spending (USD)' },
  { value: 'total_recharge', zh: '累计充值入账（USD）', en: 'Total recharge credited (USD)' },
  { value: 'account_age_days', zh: '注册时长（天）', en: 'Account age (days)' },
  { value: 'manual', zh: '管理员指定用户', en: 'Manually selected users' }
]

function onKindChange(index: number, event: Event) {
  changeRuleKind(index, (event.target as HTMLSelectElement).value as PrivilegeRuleKind)
}

function thresholdLabel(kind: string) {
  if (kind === 'account_age_days') return localText('门槛（天，1–365000 的整数）', 'Threshold (days, whole number 1–365000)')
  if (kind === 'total_recharge') return localText('门槛（累计充值入账 USD，正数）', 'Threshold (total recharge credited USD, positive)')
  return localText('门槛（累计 API 消费 USD，正数）', 'Threshold (total API spending USD, positive)')
}

const messageLength = computed(() =>
  Array.from(settings.value?.enterprise_welcome.message ?? '').length
)

const fieldsBusy = computed(() => loading.value || saving.value)
const previewDisabled = computed(
  () => !settings.value || fieldsBusy.value || Boolean(loadError.value) || Boolean(enterpriseValidationError.value)
)
const saveDisabled = computed(
  () => !settings.value || fieldsBusy.value || Boolean(loadError.value) || Boolean(validationError.value)
)

defineExpose({ save })
</script>
