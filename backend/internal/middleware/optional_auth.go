package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// OptionalAuth parses the JWT when present but never blocks anonymous callers.
// Handlers decide whether the missing identity is acceptable for their scope.
func OptionalAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			if claims, err := util.ParseToken(strings.TrimPrefix(header, "Bearer "), cfg.JWTSecret); err == nil {
				c.Set(UserKey, claims)
			}
		}
		c.Next()
	}
}

// EnsureViewerID returns the authenticated user id or writes 401 itself.
// The second result is false when the response has already been written.
func EnsureViewerID(c *gin.Context) (uint, bool) {
	uid := GetUserID(c)
	if uid == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.Fail(constants.CodeUnauthorized, constants.MsgUnauthorized))
		return 0, false
	}
	return uid, true
}
