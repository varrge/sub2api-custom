export default {
  enterpriseAdmin: {
    accountType: {
      label: '账户类型',
      all: '全部',
      personal: '个人',
      enterprise: '企业',
      hint: '企业账户计费时使用企业分组倍率。',
      adminHint: '管理员账号仅支持个人账户类型。'
    },
    badge: {
      personal: '个人',
      enterprise: '企业'
    },
    columns: {
      accountType: '账户类型'
    },
    groupRates: {
      title: '企业分组倍率',
      hint: '仅对企业账户生效。清空某个分组的倍率将清除该分组的企业倍率。',
      label: '企业倍率',
      placeholder: '沿用现有倍率',
      invalid: '企业倍率必须是大于 0 的有效数字。',
      loadFailed: '分组配置加载失败，请重新打开后再保存。',
      inheritHint: '留空沿用该用户现有倍率，不清除普通专属倍率。',
      defaultRate: '默认倍率',
      empty: '暂无可用分组。'
    },
    conversion: {
      toEnterprise: '转换为企业账户',
      toPersonal: '转换为个人账户',
      titleToEnterprise: '转换为企业账户',
      titleToPersonal: '转换为个人账户',
      description: '请在转换前核对当前余额与有效月卡信息，此操作不可撤销。',
      currentBalance: '当前余额',
      cardsTitle: '有效月卡',
      noCards: '暂无有效月卡，仅变更账户类型。',
      zeroCredit: '有效月卡剩余额度合计为 0，转换时不产生折算入账，无需设置折算倍率。',
      remainingTotal: '剩余额度合计',
      conversionRate: '统一折算倍率',
      rateHint: '折算入账时将统一应用于所有有效月卡的剩余额度。',
      credited: '折算入账金额',
      balanceAfter: '转换后余额',
      col: {
        code: '月卡代码',
        group: '分组',
        totalQuota: '总额度 (USD)',
        used: '已使用 (USD)',
        remaining: '剩余额度 (USD)',
        expiresAt: '到期时间'
      },
      blockersTitle: '存在以下阻止转换的原因：',
      confirmCheckbox: '我已确认余额与月卡折算结果',
      confirm: '确认转换',
      repreview: '重新预览',
      loading: '正在加载预览…',
      previewFailed: '加载转换预览失败。',
      conflict: '用户数据已发生变化，请查看新的预览结果并再次确认。',
      failed: '转换失败。',
      success: '账户类型转换成功。'
    }
  }
}
