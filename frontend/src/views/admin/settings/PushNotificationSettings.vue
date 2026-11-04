<template>
  <div class="space-y-6 text-sm text-neutral-800 dark:text-neutral-200">
    <!-- Error Alert -->
    <div
      v-if="error"
      role="alert"
      class="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-3.5 text-xs text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300"
    >
      <Icon name="exclamationTriangle" class="mt-0.5 h-4 w-4 shrink-0 text-red-500" />
      <span class="flex-1 leading-relaxed">{{ error }}</span>
    </div>

    <!-- Initial Loading State -->
    <div
      v-if="loading"
      class="flex flex-col items-center justify-center rounded-xl border border-neutral-200 bg-white py-16 text-neutral-500 shadow-sm dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400"
    >
      <Icon name="refresh" class="h-6 w-6 animate-spin text-primary-500" />
      <p class="mt-2 text-xs">{{ localText('正在加载推送通道设置...', 'Loading push notification settings...') }}</p>
    </div>

    <!-- Main Settings Container (Non-form) -->
    <fieldset v-else-if="loaded" :disabled="saving" class="min-w-0 space-y-6">
      <!-- Top Header & Global Switch -->
      <div
        class="flex flex-col gap-4 rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="flex items-center gap-3">
          <Toggle
            v-model="config.enabled"
            aria-label="全局推送通知开关"
          />
          <div>
            <div class="flex items-center gap-2 font-medium text-neutral-900 dark:text-neutral-100">
              <span>{{ localText('全局推送通知', 'Push Notifications') }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-[10px] font-semibold tracking-wide uppercase"
                :class="config.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300' : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'"
              >
                {{ config.enabled ? localText('已启用', 'Enabled') : localText('已停用', 'Disabled') }}
              </span>
            </div>
            <p class="text-xs text-neutral-500 dark:text-neutral-400">
              {{ localText('支持通过 Server酱 或通用自定义 Webhook 发送系统管理与计费事件提醒', 'Send administrative and billing alerts via Server酱 or Custom Webhooks.') }}
            </p>
          </div>
        </div>

        <!-- Header Action Buttons -->
        <div class="flex items-center gap-2">
          <button
            type="button"
            @click="reload"
            :disabled="loading || saving"
            class="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-1.5 text-xs font-medium text-neutral-700 transition hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:bg-neutral-700"
            :title="localText('放弃未保存更改并重新加载', 'Discard changes and reload')"
          >
            <Icon name="refresh" class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" />
            <span>{{ localText('重载', 'Reload') }}</span>
          </button>
          <button
            type="button"
            @click="save"
            :disabled="loading || saving || !dirty"
            class="inline-flex items-center gap-1.5 rounded-lg bg-primary-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-sm transition hover:bg-primary-500 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-primary-600 dark:hover:bg-primary-500"
          >
            <Icon name="check" class="h-3.5 w-3.5" :class="{ 'animate-spin': saving }" />
            <span>{{ saving ? localText('保存中...', 'Saving...') : localText('保存修改', 'Save Changes') }}</span>
          </button>
        </div>
      </div>

      <!-- Main Workspace: Channel List + Detail Editor -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <!-- Left Column: Channel List -->
        <div class="min-w-0 space-y-4 lg:col-span-4">
          <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
            <div class="mb-3 flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
              <h2 class="text-xs font-semibold tracking-wider text-neutral-500 uppercase dark:text-neutral-400">
                {{ localText('通道列表', 'Channels') }}
                <span class="ml-1 text-neutral-400">({{ config.channels.length }})</span>
              </h2>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  @click="addChannel('serverchan')"
                  class="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-neutral-50 px-2 py-1 text-[11px] font-medium text-neutral-700 transition hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:border-primary-800 dark:hover:bg-neutral-800/80 dark:hover:text-primary-400"
                >
                  <Icon name="plus" class="h-3 w-3" />
                  <span>Server酱</span>
                </button>
                <button
                  type="button"
                  @click="addChannel('webhook')"
                  class="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-neutral-50 px-2 py-1 text-[11px] font-medium text-neutral-700 transition hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:border-primary-800 dark:hover:bg-neutral-800/80 dark:hover:text-primary-400"
                >
                  <Icon name="plus" class="h-3 w-3" />
                  <span>Webhook</span>
                </button>
              </div>
            </div>

            <!-- Empty Channels State -->
            <div
              v-if="config.channels.length === 0"
              class="py-8 text-center text-xs text-neutral-400 dark:text-neutral-500"
            >
              <Icon name="mail" class="mx-auto mb-2 h-6 w-6 opacity-40" />
              <p>{{ localText('暂无推送通道，请从上方添加', 'No channels yet. Add one above.') }}</p>
            </div>

            <!-- Channel Item List -->
            <div v-else class="space-y-2">
              <div
                v-for="ch in config.channels"
                :key="ch.id"
                @click="selectedId = ch.id"
                @keydown.enter.self="selectedId = ch.id"
                @keydown.space.prevent.self="selectedId = ch.id"
                role="button"
                tabindex="0"
                :aria-pressed="selectedId === ch.id"
                :class="[
                  'group relative flex cursor-pointer items-start justify-between rounded-lg border p-3 transition',
                  selectedId === ch.id
                    ? 'border-primary-500 bg-primary-50/40 ring-1 ring-primary-500 dark:border-primary-600 dark:bg-primary-950/20 dark:ring-primary-600'
                    : 'border-neutral-200 bg-white hover:border-neutral-300 dark:border-neutral-800 dark:bg-neutral-900/50 dark:hover:border-neutral-700'
                ]"
              >
                <div class="min-w-0 flex-1 pr-2">
                  <div class="flex items-center gap-2">
                    <span
                      class="inline-block h-2 w-2 shrink-0 rounded-full"
                      :class="ch.enabled ? 'bg-emerald-500' : 'bg-neutral-300 dark:bg-neutral-600'"
                      :title="ch.enabled ? localText('已启用', 'Enabled') : localText('已停用', 'Disabled')"
                    />
                    <span class="truncate font-medium text-neutral-900 dark:text-neutral-100">
                      {{ ch.name || localText('未命名通道', 'Unnamed Channel') }}
                    </span>
                  </div>
                  <div class="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-neutral-500 dark:text-neutral-400">
                    <span class="rounded bg-neutral-100 px-1.5 py-0.5 font-mono text-[10px] dark:bg-neutral-800">
                      {{ ch.type === 'serverchan' ? 'Server酱' : 'Webhook' }}
                    </span>
                    <span
                      v-if="ch.configured"
                      class="rounded bg-emerald-50 px-1 py-0.5 text-[10px] text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400"
                    >
                      {{ localText('已配置', 'Configured') }}
                    </span>
                    <span
                      v-else
                      class="rounded bg-amber-50 px-1 py-0.5 text-[10px] text-amber-600 dark:bg-amber-950/40 dark:text-amber-400"
                    >
                      {{ localText('待完善', 'Incomplete') }}
                    </span>
                    <span class="truncate text-[10px] text-neutral-400" v-if="ch.events?.length">
                      {{ ch.events.length }} {{ localText('事件', 'events') }}
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  @click.stop="removeChannel(ch.id)"
                  :title="localText('删除通道', 'Delete Channel')"
                  class="rounded p-1 text-neutral-400 opacity-60 transition hover:bg-neutral-100 hover:text-red-600 group-hover:opacity-100 dark:hover:bg-neutral-800 dark:hover:text-red-400"
                >
                  <Icon name="trash" class="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Right Column: Channel Editor -->
        <div class="min-w-0 space-y-6 lg:col-span-8">
          <div
            v-if="!selectedChannel"
            class="flex flex-col items-center justify-center rounded-xl border border-dashed border-neutral-300 bg-neutral-50/50 py-16 text-neutral-400 dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-500"
          >
            <Icon name="cog" class="mb-2 h-8 w-8 opacity-40" />
            <p class="text-xs">{{ localText('请选择左侧通道进行编辑，或新建推送通道', 'Select a channel from the left or create a new one') }}</p>
          </div>

          <div v-else class="space-y-6">
            <!-- Block 1: Basic & Destination Info -->
            <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-3 border-b border-neutral-100 pb-3 dark:border-neutral-800">
                <div class="flex items-center gap-2">
                  <span class="rounded-md bg-primary-50 p-1 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
                    <Icon name="globe" class="h-4 w-4" />
                  </span>
                  <h2 class="text-xs font-semibold tracking-wider text-neutral-700 uppercase dark:text-neutral-200">
                    {{ localText('基本与目标通道设置', 'Basic & Destination') }}
                  </h2>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-xs text-neutral-500">{{ localText('启用此通道', 'Enable Channel') }}</span>
                  <Toggle
                    v-model="selectedChannel.enabled"
                    :aria-label="localText('启用此通道', 'Enable Channel')"
                  />
                </div>
              </div>

              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <!-- Channel Name -->
                <div>
                  <label for="channel-name" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('通道名称', 'Channel Name') }} <span class="text-red-500">*</span>
                  </label>
                  <input
                    id="channel-name"
                    v-model="selectedChannel.name"
                    type="text"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="localText('如: 运维群 / 财务企业微信', 'e.g. Ops Team / Finance Group')"
                  />
                </div>

                <!-- Channel Type Display -->
                <div>
                  <label class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('通道协议类型', 'Protocol Type') }}
                  </label>
                  <div class="flex h-[34px] items-center rounded-lg border border-neutral-200 bg-neutral-50 px-3 text-xs text-neutral-600 dark:border-neutral-700 dark:bg-neutral-800/60 dark:text-neutral-300">
                    <span class="font-mono font-medium">
                      {{ selectedChannel.type === 'serverchan' ? 'Server酱 (SendKey)' : '通用自定义 Webhook (HTTP POST)' }}
                    </span>
                  </div>
                </div>

                <!-- Credential / Webhook URL -->
                <div class="sm:col-span-2">
                  <div class="mb-1 flex items-center justify-between">
                    <label for="channel-credential" class="text-xs font-medium text-neutral-700 dark:text-neutral-300">
                      {{ selectedChannel.type === 'serverchan' ? 'Server酱 SendKey' : 'Webhook URL' }}
                      <span class="text-red-500">*</span>
                    </label>
                    <span
                      v-if="selectedChannel.configured && selectedChannel.endpoint_hint"
                      class="font-mono text-[11px] text-neutral-500 dark:text-neutral-400"
                    >
                      {{ localText('当前已绑定: ', 'Bound host: ') }}{{ selectedChannel.endpoint_hint }}
                    </span>
                  </div>
                  <input
                    id="channel-credential"
                    v-model="selectedChannel.credential"
                    type="password"
                    autocomplete="new-password"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs font-mono text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="selectedChannel.configured ? localText('留空保留已保存配置', 'Leave blank to keep saved credential') : (selectedChannel.type === 'serverchan' ? localText('SCT 开头或 sctp<数字 UID>t<密钥>', 'SCT key or sctp<UID>t<token>') : 'https://example.com/webhook')"
                  />
                  <p class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                    {{ selectedChannel.configured
                      ? localText('后端已保存该凭据。如无需修改请保持为空。', 'Saved on server. Leave empty if you do not want to change it.')
                      : localText('请输入推送凭据或可访问的 Webhook 回调地址。', 'Enter the SendKey or webhook endpoint URL.')
                    }}
                  </p>
                  <p v-if="selectedChannel.type === 'serverchan'" class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                    {{ localText('支持 Server 酱 Turbo 和 Server 酱 3。请使用官网复制按钮获取完整 SendKey；不要粘贴带省略号的展示文本或整条 API 链接。', 'Supports ServerChan Turbo and ServerChan 3. Use the website’s copy button to get the full SendKey, not shortened text with ellipses or the full API URL.') }}
                  </p>
                </div>

                <!-- Serverchan Channel Param -->
                <div v-if="selectedChannel.type === 'serverchan'" class="sm:col-span-2">
                  <label for="serverchan-channel" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('Server酱 推送通道 (可选)', 'Server酱 Channel (Optional)') }}
                  </label>
                  <input
                    id="serverchan-channel"
                    v-model="selectedChannel.serverchan_channel"
                    type="text"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs font-mono text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="localText('留空使用默认通道，如: 9|66 或 18', 'Blank uses default, e.g. 9|66')"
                  />
                  <p class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                    {{ localText('可指定推送子通道（如企业微信应用、钉钉等），支持管道符 | 组合。', 'Specify delivery channels like WeCom or DingTalk, multiple supported by |.') }}
                  </p>
                </div>

                <!-- Webhook Format Param -->
                <div v-if="selectedChannel.type === 'webhook'" class="sm:col-span-2">
                  <label for="webhook-format" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('Webhook 编码格式', 'Webhook Payload Format') }}
                  </label>
                  <select
                    id="webhook-format"
                    v-model="selectedChannel.webhook_format"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs text-neutral-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
                  >
                    <option value="json">JSON (application/json)</option>
                    <option value="form">Form-URL-Encoded (application/x-www-form-urlencoded)</option>
                  </select>
                  <p class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                    {{ localText('POST 请求包含: title, desp, tags, event, event_id, occurred_at', 'POST body includes: title, desp, tags, event, event_id, occurred_at') }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Block 2: Trigger Events -->
            <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
              <div class="mb-3 flex items-center gap-2 border-b border-neutral-100 pb-3 dark:border-neutral-800">
                <span class="rounded-md bg-primary-50 p-1 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
                  <Icon name="bell" class="h-4 w-4" />
                </span>
                <h2 class="text-xs font-semibold tracking-wider text-neutral-700 uppercase dark:text-neutral-200">
                  {{ localText('触发时机（可多选）', 'Trigger Events (Multiple)') }}
                </h2>
              </div>

              <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                <label
                  v-for="opt in eventOptions"
                  :key="opt.id"
                  :for="'evt-' + opt.id"
                  class="relative flex cursor-pointer items-start gap-2.5 rounded-lg border border-neutral-200 p-2.5 transition hover:border-neutral-300 hover:bg-neutral-50/50 dark:border-neutral-700 dark:hover:border-neutral-600 dark:hover:bg-neutral-800/40"
                  :class="{
                    'border-primary-300 bg-primary-50/20 dark:border-primary-900/60 dark:bg-primary-950/10': selectedChannel.events.includes(opt.id)
                  }"
                >
                  <input
                    :id="'evt-' + opt.id"
                    v-model="selectedChannel.events"
                    type="checkbox"
                    :value="opt.id"
                    class="mt-0.5 h-4 w-4 accent-primary-600 rounded border-neutral-300 text-primary-600 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:ring-offset-neutral-900"
                  />
                  <div class="flex-1 text-xs">
                    <div class="font-medium text-neutral-800 dark:text-neutral-200">{{ opt.name }}</div>
                    <div class="mt-0.5 text-[11px] text-neutral-500 dark:text-neutral-400">{{ opt.description }}</div>
                  </div>
                </label>
              </div>

              <!-- Special Field for shared_credit.low -->
              <div
                v-if="selectedChannel.events.includes('shared_credit.low')"
                class="mt-4 rounded-lg border border-primary-100 bg-primary-50/40 p-3 dark:border-primary-950/50 dark:bg-primary-950/20"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <label for="quota-threshold" class="text-xs font-medium text-primary-900 dark:text-primary-200">
                    {{ localText('共享额度告警阈值 (USD)', 'Shared Credit Alert Threshold (USD)') }}
                  </label>
                  <div class="relative w-full sm:w-44">
                    <span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5 text-xs text-neutral-500">$</span>
                    <input
                      id="quota-threshold"
                      v-model.number="selectedChannel.quota_threshold"
                      type="number"
                      step="0.01"
                      min="0"
                      class="w-full rounded-lg border border-neutral-300 bg-white py-1.5 pr-2.5 pl-6 text-xs text-neutral-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
                      placeholder="0.00"
                    />
                  </div>
                </div>
                <p class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                  {{ localText('当剩余可用额度低于或等于此值时提醒；同一活动配置版本只提醒一次。', 'Notify at or below this value, once per activity configuration version.') }}
                </p>
              </div>
            </div>

            <!-- Block 3: Message Content Template -->
            <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
              <div class="mb-4 flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
                <div class="flex items-center gap-2">
                  <span class="rounded-md bg-primary-50 p-1 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
                    <Icon name="document" class="h-4 w-4" />
                  </span>
                  <h2 class="text-xs font-semibold tracking-wider text-neutral-700 uppercase dark:text-neutral-200">
                    {{ localText('消息内容模板 (Markdown)', 'Message Template (Markdown)') }}
                  </h2>
                </div>
              </div>

              <div class="space-y-4">
                <!-- Title Template -->
                <div>
                  <label for="title-template" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('消息标题模板', 'Title Template') }}
                  </label>
                  <input
                    id="title-template"
                    v-model="selectedChannel.title_template"
                    type="text"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="localText('如: [{{site_name}}] {{event_name}} 提醒', 'e.g. [{{site_name}}] {{event_name}}')"
                  />
                </div>

                <!-- Tags Template -->
                <div>
                  <label for="tags-template" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('标签 / 分组 (Tags)', 'Tags Template') }}
                  </label>
                  <input
                    id="tags-template"
                    v-model="selectedChannel.tags_template"
                    type="text"
                    class="w-full rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs font-mono text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="localText('如: 账务|系统 或者 {{event_name}}', 'e.g. Billing|Alert or {{event_name}}')"
                  />
                  <p class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                    {{ localText('以 | 分隔。原生标签分组仅 Server酱³ 支持，Turbo 将保留在正文中', 'Separated by |. Native tags are Server酱³ only; Turbo keeps tags in body') }}
                  </p>
                </div>

                <!-- Body Template -->
                <div>
                  <label for="body-template" class="mb-1 block text-xs font-medium text-neutral-700 dark:text-neutral-300">
                    {{ localText('消息正文模板 (支持 Markdown 语法)', 'Body Template (Markdown supported)') }}
                  </label>
                  <textarea
                    id="body-template"
                    v-model="selectedChannel.body_template"
                    rows="6"
                    class="w-full rounded-lg border border-neutral-300 bg-white p-3 font-mono text-xs text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:placeholder-neutral-500"
                    :placeholder="localText('### 发生事件: {{event_name}}\n- 时间: {{occurred_at}}\n- 描述: {{summary}}\n- 详情: {{details}}', '### Event: {{event_name}}\n- Time: {{occurred_at}}\n- Summary: {{summary}}')"
                  ></textarea>
                </div>

                <!-- Available Variables Helper Pills -->
                <div>
                  <div class="mb-1.5 text-xs font-medium text-neutral-600 dark:text-neutral-400">
                    {{ localText('可用变量插槽（可直接复制或填入模板）:', 'Available Variables:') }}
                  </div>
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="v in variables"
                      :key="v"
                      class="inline-flex items-center rounded-md border border-neutral-200 bg-neutral-50 px-2 py-0.5 font-mono text-[11px] text-neutral-600 select-all dark:border-neutral-700 dark:bg-neutral-800/80 dark:text-neutral-300"
                    >
                      &#123;&#123;{{ v }}&#125;&#125;
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Block 4: Preview & Test Delivery -->
            <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-3 border-b border-neutral-100 pb-3 dark:border-neutral-800">
                <div class="flex items-center gap-2">
                  <span class="rounded-md bg-primary-50 p-1 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
                    <Icon name="eye" class="h-4 w-4" />
                  </span>
                  <h2 class="text-xs font-semibold tracking-wider text-neutral-700 uppercase dark:text-neutral-200">
                    {{ localText('预览与测试发送', 'Preview & Test Delivery') }}
                  </h2>
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <label for="preview-event-select" class="sr-only">{{ localText('选择模拟事件', 'Select event for preview') }}</label>
                  <select
                    id="preview-event-select"
                    v-model="previewEvent"
                    class="h-8 rounded-lg border border-neutral-300 bg-white px-2 text-xs text-neutral-900 focus:border-primary-500 focus:outline-none dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100"
                  >
                    <option v-for="opt in eventOptions" :key="opt.id" :value="opt.id">
                      {{ opt.name }}
                    </option>
                  </select>

                  <button
                    type="button"
                    @click="renderPreview"
                    :disabled="previewLoading"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 text-xs font-medium text-neutral-700 transition hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:bg-neutral-700"
                  >
                    <Icon name="refresh" class="h-3.5 w-3.5" :class="{ 'animate-spin': previewLoading }" />
                    <span>{{ previewLoading ? localText('渲染中...', 'Rendering...') : localText('生成预览', 'Render Preview') }}</span>
                  </button>

                  <button
                    type="button"
                    @click="testSelected"
                    :disabled="dirty || !selectedChannel?.configured || testing"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg bg-neutral-900 px-3 text-xs font-medium text-white transition hover:bg-neutral-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-200"
                    :title="dirty ? localText('请先保存配置后再测试发送', 'Please save settings before sending a test notification') : ''"
                  >
                    <Icon name="mail" class="h-3.5 w-3.5" :class="{ 'animate-spin': testing }" />
                    <span>{{ testing ? localText('发送中...', 'Sending...') : localText('真实测试发送', 'Send Test Notification') }}</span>
                  </button>
                </div>
              </div>

              <div v-if="dirty" class="mb-3 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/40 dark:text-amber-300">
                <Icon name="infoCircle" class="mr-1.5 inline h-3.5 w-3.5 align-text-bottom text-amber-500" />
                <span>{{ localText('检测到当前有未保存的更改。测试发送将仅基于服务端已保存的配置执行，请先点击右上角保存。', 'Unsaved changes detected. Test notification only uses saved settings. Please save first.') }}</span>
              </div>

              <!-- Preview Output Panel -->
              <div
                v-if="preview"
                class="rounded-lg border border-neutral-200 bg-neutral-50/70 p-3.5 dark:border-neutral-800 dark:bg-neutral-950/50"
              >
                <div class="mb-2 flex flex-wrap items-center gap-2 border-b border-neutral-200 pb-2 dark:border-neutral-800">
                  <span class="text-xs font-semibold text-neutral-900 dark:text-neutral-100">
                    {{ preview.title || localText('(无标题)', '(No Title)') }}
                  </span>
                  <span
                    v-if="preview.tags"
                    class="rounded bg-primary-100 px-1.5 py-0.5 font-mono text-[10px] text-primary-700 dark:bg-primary-950/60 dark:text-primary-300"
                  >
                    {{ preview.tags }}
                  </span>
                </div>
                <div class="push-markdown break-words text-sm leading-relaxed text-neutral-800 dark:text-neutral-200" data-testid="push-preview" v-html="previewHTML"></div>
              </div>
              <div
                v-else
                class="py-6 text-center text-xs text-neutral-400 dark:text-neutral-500"
              >
                {{ localText('点击「生成预览」可根据所选事件模拟渲染标题与 Markdown 内容。', 'Click "Render Preview" to simulate rendering title and Markdown body.') }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Block 5: Delivery Logs / History -->
      <div class="rounded-xl border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
        <div class="mb-3 flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
          <div class="flex items-center gap-2">
            <span class="rounded-md bg-primary-50 p-1 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
              <Icon name="clock" class="h-4 w-4" />
            </span>
            <h2 class="text-xs font-semibold tracking-wider text-neutral-700 uppercase dark:text-neutral-200">
              {{ localText('近 50 次推送投递历史', 'Delivery History (Last 50)') }}
            </h2>
          </div>
          <button
            type="button"
            @click="loadLogs"
            class="inline-flex items-center gap-1 text-xs text-primary-600 hover:text-primary-500 dark:text-primary-400"
          >
            <Icon name="refresh" class="h-3 w-3" />
            <span>{{ localText('刷新日志', 'Refresh Logs') }}</span>
          </button>
        </div>

        <!-- Logs Table / Empty -->
        <div v-if="logs.length === 0" class="py-8 text-center text-xs text-neutral-400 dark:text-neutral-500">
          {{ localText('暂无推送日志记录', 'No delivery logs found.') }}
        </div>

        <div v-else class="overflow-x-auto">
          <table class="min-w-[820px] w-full divide-y divide-neutral-200 text-left text-xs dark:divide-neutral-800">
            <thead>
              <tr class="text-[11px] font-semibold text-neutral-500 dark:text-neutral-400">
                <th class="py-2 pr-2">ID</th>
                <th class="px-2 py-2">{{ localText('通道名称', 'Channel') }}</th>
                <th class="px-2 py-2">{{ localText('触发事件', 'Event') }}</th>
                <th class="px-2 py-2">{{ localText('标题', 'Title') }}</th>
                <th class="px-2 py-2">{{ localText('状态', 'Status') }}</th>
                <th class="px-2 py-2">{{ localText('发送次数', 'Attempts') }}</th>
                <th class="px-2 py-2">{{ localText('创建时间', 'Created At') }}</th>
                <th class="py-2 pl-2">{{ localText('投递时间', 'Sent At') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-neutral-100 dark:divide-neutral-800/60">
              <tr
                v-for="log in logs"
                :key="log.id"
                class="transition hover:bg-neutral-50 dark:hover:bg-neutral-800/40"
              >
                <td class="py-2 pr-2 font-mono text-[11px] text-neutral-400">#{{ log.id }}</td>
                <td class="px-2 py-2 font-medium text-neutral-900 dark:text-neutral-100">{{ log.channel_name }}</td>
                <td class="px-2 py-2 font-mono text-[11px] text-neutral-600 dark:text-neutral-300">{{ log.event === 'test' ? localText('测试推送', 'Test') : (eventOptions.find(e => e.id === log.event)?.name || log.event) }}</td>
                <td class="max-w-xs truncate px-2 py-2 text-neutral-700 dark:text-neutral-300" :title="log.title">
                  {{ log.title }}
                </td>
                <td class="px-2 py-2">
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium tracking-wide uppercase"
                    :class="getStatusBadgeClass(log.status)"
                  >
                    {{ getStatusLabel(log.status) }}
                  </span>
                  <p v-if="log.error" class="mt-1 max-w-[220px] text-[11px] text-red-600 dark:text-red-400">{{ log.error }}</p>
                </td>
                <td class="px-2 py-2 text-neutral-600 dark:text-neutral-400">{{ log.attempts }}</td>
                <td class="px-2 py-2 font-mono text-[11px] text-neutral-500">{{ formatDateTime(log.created_at) }}</td>
                <td class="py-2 pl-2 font-mono text-[11px] text-neutral-500">
                  {{ log.sent_at ? formatDateTime(log.sent_at) : '-' }}
                  <span
                    v-if="log.error"
                    class="ml-1 inline-block text-red-500"
                    :title="log.error"
                  >
                    ⚠️
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </fieldset>
    <button v-else type="button" class="btn btn-secondary" @click="reload">{{ localText('重新加载', 'Retry') }}</button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { usePushNotifications } from '@/features/push-notifications/usePushNotifications'

const {
  localText,
  config,
  loading,
  loaded,
  saving,
  error,
  dirty,
  selectedId,
  selectedChannel,
  eventOptions,
  variables,
  logs,
  preview,
  previewEvent,
  previewLoading,
  testing,
  addChannel,
  removeChannel,
  save,
  reload,
  loadLogs,
  renderPreview,
  testSelected
} = usePushNotifications()

const previewHTML = computed(() => DOMPurify.sanitize(marked.parse(preview.value?.body || '', { async: false, gfm: true, breaks: true }), { FORBID_TAGS: ['img', 'form', 'input', 'button', 'video', 'audio'] }))

const STATUS_MAP: Record<string, { labelZh: string; labelEn: string; class: string }> = {
  pending: {
    labelZh: '待发送',
    labelEn: 'Pending',
    class: 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
  },
  sending: {
    labelZh: '发送中',
    labelEn: 'Sending',
    class: 'bg-blue-50 text-blue-600 dark:bg-blue-950/50 dark:text-blue-400'
  },
  sent: {
    labelZh: '已成功',
    labelEn: 'Sent',
    class: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
  },
  failed: {
    labelZh: '失败',
    labelEn: 'Failed',
    class: 'bg-red-50 text-red-600 dark:bg-red-950/50 dark:text-red-400'
  },
  cancelled: {
    labelZh: '已取消',
    labelEn: 'Cancelled',
    class: 'bg-neutral-100 text-neutral-500 dark:bg-neutral-800 dark:text-neutral-500'
  }
}

function getStatusLabel(status: string): string {
  const s = STATUS_MAP[status]
  if (!s) return status
  return localText(s.labelZh, s.labelEn)
}

function getStatusBadgeClass(status: string): string {
  return STATUS_MAP[status]?.class || 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
}

function formatDateTime(str: string): string {
  if (!str) return '-'
  try {
    const d = new Date(str)
    if (isNaN(d.getTime())) return str
    return d.toLocaleString()
  } catch {
    return str
  }
}
</script>

<style scoped>
.push-markdown :deep(h1), .push-markdown :deep(h2), .push-markdown :deep(h3) { margin: .75em 0 .5em; font-weight: 600; font-size: 1.15em; }
.push-markdown :deep(p) { margin: .5em 0; }
.push-markdown :deep(ul) { list-style: disc; padding-left: 1.5em; }
.push-markdown :deep(ol) { list-style: decimal; padding-left: 1.5em; }
.push-markdown :deep(pre) { overflow-x: auto; padding: .75em; background: rgb(128 128 128 / .1); border-radius: .5em; }
.push-markdown :deep(hr) { margin: 1em 0; }
.push-markdown :deep(a) { text-decoration: underline; }
</style>
