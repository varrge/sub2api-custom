package middleware

import "github.com/gin-gonic/gin"

// AuthSubject is the minimal authenticated identity stored in gin context.
// Account type is loaded from the database, never from client preferences.
type AuthSubject struct {
	UserID      int64
	Concurrency int
	AccountType string
}

func GetAuthSubjectFromContext(c *gin.Context) (AuthSubject, bool) {
	value, exists := c.Get(string(ContextKeyUser))
	if !exists {
		return AuthSubject{}, false
	}
	subject, ok := value.(AuthSubject)
	return subject, ok
}

func GetUserRoleFromContext(c *gin.Context) (string, bool) {
	value, exists := c.Get(string(ContextKeyUserRole))
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}
