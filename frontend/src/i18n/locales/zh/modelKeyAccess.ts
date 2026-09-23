export default {
  modelKeyAccess: {
    title: '按模型管理',
    subtitle: '选择一个模型，决定哪些 Key 允许调用它。',
    backToKeys: '返回 API 密钥',

    // Toasts and messages used by the page logic
    loadFailed: '加载失败，请重试。',
    saveFailed: '保存失败，请重试。',
    conflict: '这些 Key 的模型权限刚在其他地方被修改，请刷新后重试。',
    invalidModel: '请输入完整有效的模型 ID。',
    saved: '模型权限已保存。',
    catalogWarning: '模型目录加载不完整；已保存的模型和手动输入的模型 ID 仍可正常使用。',
    allBlocked: '已禁止所有模型；勾选模型可重新允许。',

    // Model picker
    modelSectionTitle: '模型',
    modelSearchPlaceholder: '搜索或输入完整模型 ID',
    useThisModel: '使用此模型',
    noModels: '暂无可用模型。',
    noModelMatches: '目录中没有匹配的模型，可直接使用上方输入的完整 ID。',

    // Key list
    stats: '全部 {total} 个 Key，其中 {allowed} 个允许此模型',
    filterNote: '筛选只用于查找，不会缩小保存范围。',
    keySearchPlaceholder: '搜索 Key 名称或 ID',
    allGroups: '全部分组',
    allowVisible: '允许当前筛选',
    blockVisible: '禁止当前筛选',
    visibleScopeHint: '这两个按钮只影响当前筛选显示的 Key；被筛掉的 Key 保持原选择。',
    columnAllow: '允许',
    columnKey: 'Key 名称',
    columnGroup: '分组',
    columnStatus: '状态',
    noGroups: '无分组',
    modifiedBadge: '已修改',
    catalogUnlisted: '目录未列出',
    catalogUnlistedHint: '该 Key 的分组目录中未列出此模型，不代表一定不能调用。',
    catalogUnknown: '目录待确认',
    catalogUnknownHint: '模型目录未成功加载完整，暂时无法确认该 Key 的分组是否列出此模型。',
    expiresAt: '到期：{time}',

    // Empty states
    noModelSelectedTitle: '先选择一个模型',
    noModelSelectedHint: '从左侧列表选择模型，或输入完整模型 ID 后点击「使用此模型」。',
    noKeysTitle: '还没有 API Key',
    noKeysHint: '请先在 API 密钥页面创建 Key，再回到这里按模型管理权限。',
    noMatchesTitle: '没有匹配的 Key',
    noMatchesHint: '试试调整搜索内容或分组筛选。',

    // Save bar
    added: '新增允许 {count} 个',
    removed: '新增禁止 {count} 个',
    noChanges: '暂无修改',
    saveScopeNote: '保存会应用到全部已加载的 Key，而不仅是当前筛选结果。',
    save: '保存修改',
    discard: '放弃修改',
    saving: '保存中…',
    retry: '重试',
    reload: '刷新',
    loading: '加载中…',

    // Unsaved-changes dialog
    switchTitle: '有未保存的修改',
    switchMessage: '继续操作前，要如何处理对「{model}」的修改？',
    saveAndContinue: '保存并继续',
    discardAndContinue: '放弃修改',
    stay: '留在当前页面'
  }
}
