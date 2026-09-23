export default {
  modelKeyAccess: {
    title: 'Manage by Model',
    subtitle: 'Pick a model and decide which keys may call it.',
    backToKeys: 'Back to API Keys',

    // Toasts and messages used by the page logic
    loadFailed: 'Failed to load. Please try again.',
    saveFailed: 'Failed to save. Please try again.',
    conflict: 'These keys’ model permissions were just changed elsewhere. Refresh and try again.',
    invalidModel: 'Please enter a complete, valid model ID.',
    saved: 'Model permissions saved.',
    catalogWarning: 'The model catalog did not fully load. Only keys whose groups are confirmed to list this model are shown; refresh to retry.',
    allBlocked: 'All models are blocked; check models to allow them again.',

    // Model picker
    modelSectionTitle: 'Models',
    modelSearchPlaceholder: 'Search or enter a full model ID',
    useThisModel: 'Use this model',
    noModels: 'No models available yet.',
    noModelMatches: 'No catalog match. You can still use the full ID entered above.',

    // Key list
    stats: '{allowed} of {total} eligible keys allow this model',
    filterNote: 'Filters only narrow what you see, not what gets saved.',
    keySearchPlaceholder: 'Search key name or ID',
    allGroups: 'All groups',
    allowVisible: 'Allow filtered',
    blockVisible: 'Block filtered',
    visibleScopeHint: 'These buttons only affect the keys currently shown by filters; hidden keys keep their selection.',
    columnAllow: 'Allow',
    columnKey: 'Key name',
    columnGroup: 'Groups',
    columnStatus: 'Status',
    noGroups: 'No groups',
    modifiedBadge: 'Modified',
    expiresAt: 'Expires: {time}',

    // Empty states
    noModelSelectedTitle: 'Select a model first',
    noModelSelectedHint: 'Choose a model from the list on the left, or enter a full model ID and click “Use this model”.',
    noKeysTitle: 'No keys can use this model',
    noKeysHint: 'None of your keys are bound to a group whose catalog lists this model.',
    noMatchesTitle: 'No matching keys',
    noMatchesHint: 'Try adjusting the search or group filter.',

    // Save bar
    added: '{count} newly allowed',
    removed: '{count} newly blocked',
    noChanges: 'No changes yet',
    saveScopeNote: 'Saving applies to all eligible keys, including ones hidden by the search filters.',
    save: 'Save changes',
    discard: 'Discard changes',
    saving: 'Saving…',
    retry: 'Retry',
    reload: 'Reload',
    loading: 'Loading…',

    // Unsaved-changes dialog
    switchTitle: 'Unsaved changes',
    switchMessage: 'Before continuing, what should we do with your changes to “{model}”?',
    saveAndContinue: 'Save and continue',
    discardAndContinue: 'Discard changes',
    stay: 'Stay on this page'
  }
}
