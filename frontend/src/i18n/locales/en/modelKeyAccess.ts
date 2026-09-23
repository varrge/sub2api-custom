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
    catalogWarning: 'The model catalog did not fully load. Saved models and manually entered model IDs still work.',
    allBlocked: 'All models are blocked; check models to allow them again.',

    // Model picker
    modelSectionTitle: 'Models',
    modelSearchPlaceholder: 'Search or enter a full model ID',
    useThisModel: 'Use this model',
    noModels: 'No models available yet.',
    noModelMatches: 'No catalog match. You can still use the full ID entered above.',

    // Key list
    stats: '{allowed} of {total} keys allow this model',
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
    catalogUnlisted: 'Not in catalog',
    catalogUnlistedHint: 'This model is not listed in the key’s group catalog; it may still be callable.',
    catalogUnknown: 'Catalog pending',
    catalogUnknownHint: 'The model catalog did not fully load, so we cannot confirm whether this key’s groups list the model.',
    expiresAt: 'Expires: {time}',

    // Empty states
    noModelSelectedTitle: 'Select a model first',
    noModelSelectedHint: 'Choose a model from the list on the left, or enter a full model ID and click “Use this model”.',
    noKeysTitle: 'No API keys yet',
    noKeysHint: 'Create a key on the API Keys page first, then come back to manage access by model.',
    noMatchesTitle: 'No matching keys',
    noMatchesHint: 'Try adjusting the search or group filter.',

    // Save bar
    added: '{count} newly allowed',
    removed: '{count} newly blocked',
    noChanges: 'No changes yet',
    saveScopeNote: 'Saving applies to every loaded key, not just the filtered results.',
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
