package handler

import (
	"net/http"
	"strings"

	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// pathID reads the uuid path parameter param. When it's missing or malformed
// it answers 400 and returns false, and the handler just returns.
func pathID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return uuid.Nil, false
	}
	return id, true
}

// parseUUIDs parses id filters. Clients send them repeated (id[]=a&id[]=b),
// comma-joined (id=a,b) or both, so every value is split on commas; empty
// parts are skipped.
func parseUUIDs(values ...string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, joined := range values {
		for _, part := range strings.Split(joined, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := uuid.Parse(part)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// currentUser is the signed-in user. Without one it answers 401 and returns
// false, and the handler just returns.
func currentUser(c *gin.Context) (uuid.UUID, bool) {
	userID, err := utils.UserIDFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return uuid.Nil, false
	}
	return userID, true
}
