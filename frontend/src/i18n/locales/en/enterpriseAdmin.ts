export default {
  enterpriseAdmin: {
    accountType: {
      label: 'Account Type',
      all: 'All',
      personal: 'Personal',
      enterprise: 'Enterprise',
      hint: 'Enterprise accounts use enterprise group rates for billing.',
      adminHint: 'Admin accounts only support the personal account type.'
    },
    badge: {
      personal: 'Personal',
      enterprise: 'Enterprise'
    },
    columns: {
      accountType: 'Account Type'
    },
    groupRates: {
      loadFailed: 'Group configuration could not be loaded. Reopen this dialog before saving.',
      title: 'Enterprise Group Rates',
      hint: 'Applies only to enterprise accounts. Clear a rate to remove the enterprise multiplier for that group.',
      label: 'Enterprise rate',
      placeholder: 'Use existing rate',
      invalid: 'Enterprise rate must be a valid number greater than 0.',
      inheritHint: 'Leave empty to keep the current pricing, including the ordinary user rate.',
      defaultRate: 'Default rate',
      empty: 'No active standard groups available.'
    },
    conversion: {
      toEnterprise: 'Convert to Enterprise Account',
      toPersonal: 'Convert to Personal Account',
      titleToEnterprise: 'Convert to Enterprise Account',
      titleToPersonal: 'Convert to Personal Account',
      description: 'Review the current balance and valid monthly cards below before converting. This operation cannot be undone.',
      currentBalance: 'Current Balance',
      cardsTitle: 'Valid Monthly Cards',
      noCards: 'No valid monthly cards. Only the account type will change.',
      zeroCredit: 'The remaining value of valid monthly cards totals 0. Nothing will be credited during conversion, so no conversion rate is required.',
      remainingTotal: 'Total Remaining',
      conversionRate: 'Unified Conversion Rate',
      rateHint: 'Applied to the remaining value of all valid monthly cards when crediting the balance.',
      credited: 'Credited Amount',
      balanceAfter: 'Balance After Conversion',
      col: {
        code: 'Card Code',
        group: 'Group',
        totalQuota: 'Total Quota (USD)',
        used: 'Used (USD)',
        remaining: 'Remaining (USD)',
        expiresAt: 'Expires At'
      },
      blockersTitle: 'Conversion is blocked:',
      confirmCheckbox: 'I have reviewed the balance and card conversion results',
      confirm: 'Convert',
      repreview: 'Re-preview',
      loading: 'Loading preview...',
      previewFailed: 'Failed to load the conversion preview.',
      conflict: 'User data changed. Please review the new preview and confirm again.',
      failed: 'Conversion failed.',
      success: 'Account type converted successfully.'
    }
  }
}
