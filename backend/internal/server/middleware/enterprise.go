package middleware

import "strings"

// Account authority comes from the database on each JWT request. The browser's
// navigation preferences never grant access to personal-only features.
func EnterprisePanelPathAllowed(path string) bool {
	for _, prefix := range []string{"/api/v1/redeem", "/api/v1/subscriptions", "/api/v1/group-buy", "/api/v1/activities", "/api/v1/user/aff", "/api/v1/promo", "/api/v1/payment/plans", "/api/v1/payment/coupons"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	return true
}
