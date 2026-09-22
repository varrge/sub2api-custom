<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-center gap-2">
        <Icon name="trophy" size="md" class="text-primary-500" />
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.activityLeaderboard.title") }}
        </h2>
      </div>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.activityLeaderboard.description") }}
      </p>
    </div>

    <div class="space-y-5 p-4 sm:p-6">
      <div v-if="loading" class="flex items-center gap-2 text-gray-500">
        <div
          class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"
        ></div>
        {{ t("common.loading") }}
      </div>

      <div
        v-else-if="loadFailed"
        role="alert"
        class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300"
        data-testid="activity-leaderboard-load-error"
      >
        <span>{{ t("admin.settings.activityLeaderboard.loadFailed") }}</span>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="activity-leaderboard-retry"
          @click="loadConfig"
        >
          {{ t("admin.settings.activityLeaderboard.retry") }}
        </button>
      </div>

      <template v-else>
        <div
          class="rounded-lg border border-sky-200 bg-sky-50 p-4 dark:border-sky-800 dark:bg-sky-900/20"
        >
          <div class="flex items-start">
            <Icon
              name="infoCircle"
              size="md"
              class="mt-0.5 flex-shrink-0 text-sky-500"
            />
            <div class="ml-3 space-y-1.5 text-sm text-sky-700 dark:text-sky-300">
              <p>{{ t("admin.settings.activityLeaderboard.billingNote") }}</p>
              <p>{{ t("admin.settings.activityLeaderboard.rulesNote") }}</p>
              <p>{{ t("admin.settings.activityLeaderboard.timeZoneNote") }}</p>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t("admin.settings.activityLeaderboard.enabled")
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.enabledHint") }}
            </p>
          </div>
          <Toggle
            v-model="form.enabled"
            :aria-label="t('admin.settings.activityLeaderboard.enabled')"
            data-testid="activity-leaderboard-enabled"
          />
        </div>

        <div
          class="grid grid-cols-1 gap-6 border-t border-gray-100 pt-4 sm:grid-cols-2 dark:border-dark-700"
        >
          <div class="sm:col-span-2">
            <label for="activity-leaderboard-title"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.fieldTitle") }}
            </label>
            <input
              v-model="form.title"
              type="text"
              maxlength="80"
              class="input min-w-0"
              id="activity-leaderboard-title"
              data-testid="activity-leaderboard-title"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.fieldTitleHint") }}
            </p>
          </div>

          <div class="sm:col-span-2">
            <label for="activity-leaderboard-subtitle"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.fieldSubtitle") }}
            </label>
            <input
              v-model="form.subtitle"
              type="text"
              maxlength="160"
              class="input min-w-0"
              id="activity-leaderboard-subtitle"
              data-testid="activity-leaderboard-subtitle"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.fieldSubtitleHint") }}
            </p>
          </div>

          <div class="sm:col-span-2">
            <label for="activity-leaderboard-reward"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.fieldReward") }}
            </label>
            <textarea
              v-model="form.reward_description"
              rows="3"
              maxlength="1000"
              class="input min-w-0"
              id="activity-leaderboard-reward"
              data-testid="activity-leaderboard-reward"
            ></textarea>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.fieldRewardHint") }}
            </p>
          </div>

          <div>
            <label for="activity-leaderboard-starts-at"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.startsAt") }}
            </label>
            <input
              v-model="form.startsAtInput"
              type="datetime-local"
              step="1"
              class="input min-w-0"
              id="activity-leaderboard-starts-at"
              data-testid="activity-leaderboard-starts-at"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.timeHint") }}
            </p>
          </div>

          <div>
            <label for="activity-leaderboard-ends-at"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.endsAt") }}
            </label>
            <input
              v-model="form.endsAtInput"
              type="datetime-local"
              step="1"
              class="input min-w-0"
              id="activity-leaderboard-ends-at"
              data-testid="activity-leaderboard-ends-at"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.timeHint") }}
            </p>
          </div>

          <div class="sm:col-span-2">
            <label for="activity-leaderboard-demo-expires-at"
              class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              {{ t("admin.settings.activityLeaderboard.demoExpiresAt") }}
            </label>
            <div class="flex flex-wrap items-center gap-2 sm:flex-nowrap">
              <input
                v-model="form.demoExpiresAtInput"
                type="datetime-local"
              step="1"
                class="input min-w-0"
                id="activity-leaderboard-demo-expires-at"
              data-testid="activity-leaderboard-demo-expires-at"
              />
              <button
                v-if="form.demoExpiresAtInput"
                type="button"
                class="btn btn-secondary btn-sm"
                data-testid="activity-leaderboard-demo-clear"
                :disabled="saving"
                @click="form.demoExpiresAtInput = ''"
              >
                {{ t("admin.settings.activityLeaderboard.demoExpiresAtClear") }}
              </button>
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.activityLeaderboard.demoExpiresAtHint") }}
            </p>
          </div>
        </div>

        <div
          v-if="showValidation && validationErrors.length"
          role="alert"
          class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300"
          data-testid="activity-leaderboard-validation"
        >
          <ul class="list-disc space-y-1 pl-5">
            <li v-for="errorKey in validationErrors" :key="errorKey">
              {{ t(`admin.settings.activityLeaderboard.validation.${errorKey}`) }}
            </li>
          </ul>
        </div>

        <div
          class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700"
        >
          <button
            type="button"
            data-testid="activity-leaderboard-save"
            :disabled="saving"
            class="btn btn-primary btn-sm"
            @click="saveConfig"
          >
            <svg
              v-if="saving"
              class="mr-1 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ saving ? t("common.saving") : t("common.save") }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import type { ActivityLeaderboardSettings } from "@/api/admin/settings";
import { useAppStore } from "@/stores";
import { extractApiErrorMessage } from "@/utils/apiError";
import { notifyActivityLeaderboardConfigSaved } from "@/utils/activityLeaderboardEvents";
import {
  beijingDateTimeLocalToRfc3339,
  parseBeijingDateTimeLocal,
  toBeijingDateTimeLocal,
} from "@/utils/beijingDateTime";
import Icon from "@/components/icons/Icon.vue";
import Toggle from "@/components/common/Toggle.vue";

const { t } = useI18n();
const appStore = useAppStore();

const MAX_PERIOD_MS = 366 * 24 * 60 * 60 * 1000;

const loading = ref(true);
const loadFailed = ref(false);
const saving = ref(false);
const showValidation = ref(false);

const form = reactive({
  enabled: true,
  title: "",
  subtitle: "",
  reward_description: "",
  startsAtInput: "",
  endsAtInput: "",
  demoExpiresAtInput: "",
});

const runeLength = (value: string) => [...value].length;

const validationErrors = computed<string[]>(() => {
  const errors: string[] = [];

  const titleLength = runeLength(form.title.trim());
  if (titleLength < 1 || titleLength > 80) errors.push("title");
  if (runeLength(form.subtitle) > 160) errors.push("subtitle");
  if (runeLength(form.reward_description) > 1000) errors.push("rewardDescription");

  const startsAt = parseBeijingDateTimeLocal(form.startsAtInput);
  const endsAt = parseBeijingDateTimeLocal(form.endsAtInput);
  if (startsAt === null || endsAt === null) {
    errors.push("periodRequired");
  } else {
    if (startsAt >= endsAt) errors.push("periodOrder");
    else if (endsAt - startsAt > MAX_PERIOD_MS) errors.push("periodLength");
  }

  if (form.demoExpiresAtInput) {
    const demoExpiresAt = parseBeijingDateTimeLocal(form.demoExpiresAtInput);
    if (demoExpiresAt === null) errors.push("demoInvalid");
    else if (startsAt !== null && demoExpiresAt > startsAt) {
      errors.push("demoAfterStart");
    }
  }

  return errors;
});

function applyConfig(config: ActivityLeaderboardSettings) {
  form.enabled = config.enabled;
  form.title = config.title;
  form.subtitle = config.subtitle;
  form.reward_description = config.reward_description;
  form.startsAtInput = toBeijingDateTimeLocal(config.starts_at);
  form.endsAtInput = toBeijingDateTimeLocal(config.ends_at);
  form.demoExpiresAtInput = toBeijingDateTimeLocal(config.demo_expires_at);
}

async function loadConfig() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const config = await adminAPI.settings.getActivityLeaderboardSettings();
    applyConfig(config);
  } catch {
    // Never expose unsaved defaults as a saveable form after a failed load.
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

async function saveConfig() {
  if (saving.value || loading.value || loadFailed.value) return;
  showValidation.value = true;
  if (validationErrors.value.length) return;

  const startsAt = beijingDateTimeLocalToRfc3339(form.startsAtInput);
  const endsAt = beijingDateTimeLocalToRfc3339(form.endsAtInput);
  if (!startsAt || !endsAt) return;

  saving.value = true;
  try {
    const updated = await adminAPI.settings.updateActivityLeaderboardSettings({
      enabled: form.enabled,
      title: form.title.trim(),
      subtitle: form.subtitle,
      reward_description: form.reward_description,
      starts_at: startsAt,
      ends_at: endsAt,
      demo_expires_at: form.demoExpiresAtInput
        ? beijingDateTimeLocalToRfc3339(form.demoExpiresAtInput)
        : null,
    });
    applyConfig(updated);
    showValidation.value = false;
    appStore.showSuccess(t("admin.settings.activityLeaderboard.saved"));
    notifyActivityLeaderboardConfigSaved();
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        t("admin.settings.activityLeaderboard.saveFailed"),
      ),
    );
  } finally {
    saving.value = false;
  }
}

onMounted(loadConfig);
</script>
