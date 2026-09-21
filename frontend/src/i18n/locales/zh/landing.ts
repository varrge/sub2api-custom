export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    marketing: {
      skip: '跳至主要内容', navigation: '首页导航',
      nav: { highlights: '亮点', start: '如何接入', pricing: '费用' },
      eyebrow: '多模型 API · 编程与应用开发', heroFirst: '让灵感，', heroSecond: '接上强大的 AI。',
      description: '在一个平台接入所需模型，让每一次调用清晰可查。', browseModels: '查看支持模型', discover: '向下探索，了解更多',
      demo: {
        title: '接入场景演示', choose: '选择演示场景', application: '你的应用', connection: 'API 连接', model: '模型能力',
        caption: '场景示意。可用模型与能力以实际服务及账户权限为准。',
        code: { label: '编程辅助', prompt: '从一个问题开始', result: '梳理思路，完善代码' },
        write: { label: '内容创作', prompt: '从一个灵感开始', result: '展开想法，打磨表达' },
        build: { label: '应用开发', prompt: '从一个需求开始', result: '将 AI 接入你的应用' }
      },
      highlights: { eyebrow: '为下一次创造，准备就绪', title: '把复杂留在背后。', description: '把时间，用在下一次创造。' },
      models: { label: '模型选择', title: '找到适合任务的 AI。', description: '根据任务选择模型，按账户授权接入所需能力。' },
      tools: { label: '接入体验', title: '接着用，你熟悉的工具。', description: '按工具的接入方式配置服务地址、密钥和模型，将 AI 带入工作流。' },
      usage: { label: '用量管理', title: '每一次调用，心中有数。', description: '在控制台查看使用量与消费明细，让花费有迹可循。', caption: '用量趋势示意' },
      start: {
        eyebrow: '开始，只需清楚的几步', title: '从想用，到用上。', description: '跟随接入指引，为你的工具和应用配置 AI。',
        account: { title: '登录账户', description: '了解可用服务与开通方式，确认账户已有相应权益。' },
        key: { title: '创建密钥', description: '在控制台创建 API Key，选择已获授权的服务。' },
        app: { title: '配置应用', description: '按工具指引填入服务地址、密钥和模型，完成首次调用。' }
      },
      pricing: {
        eyebrow: '了解费用，再开始', title: '用得清楚，选得从容。', description: '模型、计费单位和额度规则，都是选择服务时值得了解的细节。',
        models: '查看模型计费', plans: '查看可购方案',
        model: { title: '按模型，了解计费', description: '不同模型与服务分组可能采用不同价格，调用前查看对应的计费说明。' },
        quota: { title: '按需求，选择权益', description: '选择方案时核对模型范围、有效期、额度及使用限制，以实际商品说明为准。' },
        records: { title: '用过多少，随时可查', description: '登录控制台查看调用记录和消费明细，了解自己的使用情况。' }
      },
      faq: {
        title: '你可能想知道。',
        models: { question: '我可以使用哪些模型？', answer: '可用模型取决于站点配置、服务分组及账户权限。模型广场开放时，可在其中查看目录；实际调用以账户获授权的模型为准。' },
        tools: { question: '如何连接我的工具或应用？', answer: '通常需要配置服务地址、API Key 和模型名称。不同工具使用的协议与配置方式可能不同，请按站点文档或控制台中的密钥使用指引完成接入。' },
        billing: { question: '费用和额度如何计算？', answer: '费用取决于使用的模型、服务分组及计费规则。购买方案前，请查看有效期、额度与使用限制；使用后可在控制台查看明细。' }
      },
      closing: { title: '下一个想法，从这里开始。', description: '为你的工具和应用，找到合适的 AI 能力。' }
    },
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    // 新增：面向用户的价值主张
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多'
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    billingType: '计费方式',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
