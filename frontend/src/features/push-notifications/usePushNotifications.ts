import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { pushNotificationsApi, type PushChannel, type PushConfig, type PushDelivery, type PushEvent, type PushPreview } from '@/api/admin/pushNotifications'

export const pushVariables = ['site_name', 'event_name', 'event_id', 'occurred_at', 'summary', 'details', 'user_id', 'order_id', 'amount', 'currency', 'product_name', 'team_code', 'remaining_quota']
const events: [PushEvent, string, string, string, string][] = [
  ['payment.recharged', '充值成功', 'Recharge completed', '在线充值到账后通知', 'After an online recharge is fulfilled'],
  ['subscription.purchased', '订阅购买成功', 'Subscription purchased', '订阅订单完成后通知', 'After a subscription order is fulfilled'],
  ['monthcard.purchased', '月卡购买成功', 'Month card purchased', '月卡订单完成后通知', 'After a month-card order is fulfilled'],
  ['team.full', '拼团已满', 'Team filled', '拼团首次达到满团时通知', 'When a team first becomes full'],
  ['ops.alert', '运行告警', 'Operations alert', '监控规则产生告警时通知，请先配置运行监控规则', 'When configured monitoring rules raise an alert'],
  ['shared_credit.low', '共享额度不足', 'Shared credit low', '活动可用额度低于或等于阈值；每个活动配置版本提醒一次', 'Available activity credit reaches the threshold; once per pool revision'],
]
export function usePushNotifications() {
  const { locale } = useI18n()
  const app = useAppStore()
  const localText = (zh: string, en: string) => locale.value.startsWith('zh') ? zh : en
  const config = ref<PushConfig>({ revision: 1, enabled: false, channels: [] })
  const baseline = ref('')
  const loaded = computed(() => baseline.value !== '')
  const loading = ref(true), saving = ref(false), error = ref(''), selectedId = ref('')
  const selectedChannel = computed(() => config.value.channels.find(c => c.id === selectedId.value))
  const dirty = computed(() => baseline.value !== '' && JSON.stringify(config.value) !== baseline.value)
  const eventOptions = computed(() => events.map(([id, zh, en, hintZh, hintEn]) => ({ id, name: localText(zh, en), description: localText(hintZh, hintEn) })))
  const logs = ref<PushDelivery[]>([]), preview = ref<PushPreview | null>(null)
  const previewEvent = ref<PushEvent>('payment.recharged'), previewLoading = ref(false), testing = ref(false)
  let previewVersion = 0
  const message = (e: unknown) => extractApiErrorMessage(e, localText('操作失败，请稍后重试', 'Request failed. Please try again.'))
  const accept = (value: PushConfig) => {
    config.value = { ...value, channels: value.channels.map(c => ({ ...c, credential: '' })) }
    baseline.value = JSON.stringify(config.value)
    if (!config.value.channels.some(c => c.id === selectedId.value)) selectedId.value = config.value.channels[0]?.id || ''
  }
  async function loadLogs() {
    try { logs.value = await pushNotificationsApi.history() } catch (e) { error.value = message(e) }
  }
  async function reload() {
    if (saving.value || (dirty.value && !window.confirm(localText('重新加载会放弃未保存的推送配置，继续吗？', 'Discard unsaved notification changes?')))) return
    loading.value = true; error.value = ''
    try { accept(await pushNotificationsApi.get()); await loadLogs() } catch (e) { error.value = message(e) }
    finally { loading.value = false }
  }
  async function save() {
    if (saving.value || loading.value || !dirty.value) return
    saving.value = true; error.value = ''
    try { accept(await pushNotificationsApi.save(JSON.parse(JSON.stringify(config.value)))); app.showSuccess(localText('推送设置已保存', 'Push settings saved')) }
    catch (e) { error.value = message(e) }
    finally { saving.value = false }
  }
  function addChannel(type: PushChannel['type']) {
    if (saving.value || loading.value || !loaded.value) return
    if (config.value.channels.length >= 10) { error.value = localText('最多添加 10 个推送渠道', 'Up to 10 channels'); return }
    const id = crypto.randomUUID()
    config.value.channels.push({ id, name: type === 'serverchan' ? 'Server 酱' : 'Webhook', type, enabled: true, credential: '', configured: false, endpoint_hint: '',
      events: ['payment.recharged'], title_template: '{{event_name}} · {{site_name}}',
      body_template: '## {{event_name}}\n\n{{summary}}\n\n{{details}}\n\n---\n**站点**：{{site_name}}\n\n**时间**：{{occurred_at}}',
      tags_template: '{{event_name}}', quota_threshold: 20, serverchan_channel: '', webhook_format: 'json' })
    selectedId.value = id
  }
  function removeChannel(id: string) {
    if (saving.value || !window.confirm(localText('删除该推送渠道？保存后生效。', 'Delete this channel? The change takes effect after saving.'))) return
    config.value.channels = config.value.channels.filter(c => c.id !== id)
    if (selectedId.value === id) selectedId.value = config.value.channels[0]?.id || ''
  }
  async function renderPreview() {
    const channel = selectedChannel.value
    if (!channel) return
    const version = ++previewVersion
    previewLoading.value = true; error.value = ''
    try { const rendered = await pushNotificationsApi.preview({ ...channel, credential: '' }, previewEvent.value); if (version === previewVersion) preview.value = rendered }
    catch (e) { if (version === previewVersion) error.value = message(e) }
    finally { if (version === previewVersion) previewLoading.value = false }
  }
  async function testSelected() {
    const channel = selectedChannel.value
    if (!channel?.configured || dirty.value || testing.value || saving.value || loading.value) return
    testing.value = true; error.value = ''
    try {
      await pushNotificationsApi.test(channel.id, config.value.revision)
      app.showSuccess(localText('测试通知已加入队列，请稍后刷新发送记录查看结果', 'Test queued. Refresh delivery history shortly to see the result.'))
      await loadLogs()
    } catch (e) { error.value = message(e) } finally { testing.value = false }
  }
  watch(() => [selectedId.value, selectedChannel.value?.type], () => {
    previewEvent.value = selectedChannel.value?.events[0] || 'payment.recharged'
  })
  watch(() => JSON.stringify([selectedChannel.value, previewEvent.value]), () => { ++previewVersion; preview.value = null; previewLoading.value = false })
  onMounted(reload)
  return { localText, config, loading, loaded, saving, error, dirty, selectedId, selectedChannel, eventOptions, variables: pushVariables, logs, preview, previewEvent, previewLoading, testing, addChannel, removeChannel, save, reload, loadLogs, renderPreview, testSelected }
}
