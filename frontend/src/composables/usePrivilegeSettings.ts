import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPrivilegeSettings, updatePrivilegeSettings, type PrivilegeSettings, type PersonalPrivilegeRule, type PrivilegeRuleKind } from '@/api/privileges'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useWelcomePlayback } from './useWelcomePlayback'
import { welcomeEffects } from '@/utils/welcomeEffects'

export function usePrivilegeSettings() {
  const { locale } = useI18n()
  const app = useAppStore()
  const settings = ref<PrivilegeSettings | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const loadError = ref('')
  const preview = useWelcomePlayback()
  const userIDsText = ref<Record<string, string>>({})
  const localText = (zh: string, en: string) => locale.value.startsWith('zh') ? zh : en
  const validText = (text: string, max: number, required: boolean) => {
    // Deliberately reject single-line control and line-separator characters.
    // eslint-disable-next-line no-control-regex
    return (!required || Boolean(text.trim())) && Array.from(text).length <= max && !/[\u0000-\u001f\u007f-\u009f\u2028\u2029]/u.test(text)
  }
  const enterpriseValidationError = computed(() => {
    if (!welcomeEffects.includes(settings.value?.enterprise_welcome.effect ?? 'capsule')) return localText('请选择有效的欢迎动效', 'Choose a valid welcome effect')
    const text = settings.value?.enterprise_welcome.message ?? ''
    return !validText(text, 80, true)
      ? localText('请输入 1–80 个字符的单行欢迎文案', 'Enter a single-line welcome message of 1–80 characters')
      : ''
  })

  function parseUserIDs(text: string): number[] | null {
    const parts = text.trim().split(/[,，\s]+/u).filter(Boolean)
    if (!parts.length || parts.length > 200 || parts.some(part => !/^\d+$/.test(part) || !Number.isSafeInteger(Number(part)) || Number(part) <= 0)) return null
    const ids = parts.map(Number)
    return new Set(ids).size === ids.length ? ids : null
  }

  function ruleError(rule: PersonalPrivilegeRule): string {
    if (!welcomeEffects.includes(rule.welcome_effect ?? 'capsule')) return localText('请选择有效的欢迎动效', 'Choose a valid welcome effect')
    if (!validText(rule.title, 24, true)) return localText('头衔需为 1–24 个字符的单行文本', 'Title must be 1–24 single-line characters')
    if (!validText(rule.welcome_message, 80, rule.welcome_enabled)) return localText('欢迎文案需为 1–80 个字符的单行文本；关闭欢迎时可留空', 'Welcome text must be 1–80 single-line characters; it may be empty when disabled')
    if (rule.kind === 'manual') {
      return parseUserIDs(userIDsText.value[rule.id] ?? '') ? '' : localText('请输入 1–200 个不重复的有效用户 ID，以逗号或空格分隔', 'Enter 1–200 unique positive user IDs, separated by commas or spaces')
    }
    if (!['total_spend', 'total_recharge', 'account_age_days'].includes(rule.kind)) return localText('请选择有效的条件类型', 'Choose a valid condition type')
    if (typeof rule.threshold !== 'number' || !Number.isFinite(rule.threshold) || rule.threshold <= 0 || rule.threshold > 1e12) return localText('门槛必须大于 0 且不超过 1 万亿', 'Threshold must be greater than 0 and at most 1 trillion')
    if (rule.kind === 'account_age_days' && (!Number.isInteger(rule.threshold) || rule.threshold > 365000)) return localText('注册天数必须是 1–365000 的整数', 'Account age must be a whole number from 1 to 365000 days')
    return ''
  }

  const ruleErrors = computed(() => Object.fromEntries((settings.value?.personal_rules ?? []).map(rule => [rule.id, ruleError(rule)])))
  const validationError = computed(() => enterpriseValidationError.value || Object.values(ruleErrors.value).find(Boolean) || '')

  function acceptSettings(value: PrivilegeSettings) {
    settings.value = { ...value,
      enterprise_welcome: { ...value.enterprise_welcome, effect: value.enterprise_welcome.effect || 'capsule' },
      personal_rules: (value.personal_rules ?? []).map(rule => ({ ...rule, welcome_effect: rule.welcome_effect || 'capsule' })) }
    userIDsText.value = Object.fromEntries(settings.value.personal_rules.map(rule => [rule.id, (rule.user_ids ?? []).join(', ')]))
  }

  function addRule() {
    if (!settings.value || saving.value || settings.value.personal_rules.length >= 50) return
    const id = crypto.randomUUID()
    settings.value.personal_rules.push({ id, enabled: true, title: '', kind: 'total_spend', threshold: 1,
      user_ids: [], welcome_enabled: true, welcome_message: '欢迎回来', welcome_effect: 'capsule' })
    userIDsText.value[id] = ''
  }

  function removeRule(index: number) {
    if (!settings.value || saving.value) return
    const [rule] = settings.value.personal_rules.splice(index, 1)
    if (rule) delete userIDsText.value[rule.id]
  }

  function moveRule(index: number, direction: -1 | 1) {
    const rules = settings.value?.personal_rules
    const target = index + direction
    if (!rules || saving.value || target < 0 || target >= rules.length) return
    ;[rules[index], rules[target]] = [rules[target], rules[index]]
  }

  function changeRuleKind(index: number, kind: PrivilegeRuleKind) {
    const rule = settings.value?.personal_rules[index]
    if (!rule || saving.value) return
    rule.kind = kind
    rule.threshold = kind === 'manual' ? 0 : 1
    rule.user_ids = []
    userIDsText.value[rule.id] = ''
  }

  async function load() {
    if (loading.value) return
    loading.value = true
    loadError.value = ''
    try {
      acceptSettings(await getPrivilegeSettings())
    } catch (error) {
      loadError.value = extractApiErrorMessage(error, localText('特权设置加载失败，请重试', 'Could not load privilege settings. Please retry.'))
    } finally {
      loading.value = false
    }
  }

  async function save() {
    if (!settings.value || saving.value || loading.value || loadError.value) return
    if (validationError.value) {
      app.showError(validationError.value)
      return
    }
    saving.value = true
    const payload: PrivilegeSettings = {
      enterprise_welcome: {
        enabled: settings.value.enterprise_welcome.enabled,
        message: settings.value.enterprise_welcome.message.trim(),
        effect: settings.value.enterprise_welcome.effect ?? 'capsule'
      },
      personal_rules: settings.value.personal_rules.map(rule => ({ ...rule,
        welcome_effect: rule.welcome_effect ?? 'capsule',
        title: rule.title.trim(), welcome_message: rule.welcome_message.trim(),
        threshold: rule.kind === 'manual' ? 0 : rule.threshold,
        user_ids: rule.kind === 'manual' ? parseUserIDs(userIDsText.value[rule.id])! : []
      }))
    }
    try {
      acceptSettings(await updatePrivilegeSettings(payload))
      app.showSuccess(localText('特权设置已保存', 'Privilege settings saved'))
    } catch (error) {
      app.showError(extractApiErrorMessage(error, localText('特权设置保存失败', 'Could not save privilege settings')))
    } finally {
      saving.value = false
    }
  }

  function showPreview() {
    if (settings.value && !enterpriseValidationError.value) preview.play(settings.value.enterprise_welcome.message.trim(), settings.value.enterprise_welcome.effect)
  }

  function showRulePreview(index: number) {
    const rule = settings.value?.personal_rules[index]
    if (rule?.welcome_enabled && !ruleError(rule)) preview.play(rule.welcome_message.trim(), rule.welcome_effect)
  }

  onMounted(load)
  return { settings, loading, saving, loadError, validationError, enterpriseValidationError, ruleErrors, userIDsText,
    load, save, preview, showPreview, showRulePreview, addRule, removeRule, moveRule, changeRuleKind }
}
