package service

// ApplyEnterpriseRecharge keeps the checkout preview and actual order on the
// same independent multiplier, without personal recharge promotions.
func (c *PaymentConfig) ApplyEnterpriseRecharge() {
	c.BalanceRechargeMultiplier = normalizeBalanceRechargeMultiplier(c.EnterpriseBalanceRechargeMultiplier)
	c.RechargeBonusTiers = []RechargeBonusTier{}
	c.RechargeBonusNotice = ""
}
