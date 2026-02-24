package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetKeys() gin.HandlerFunc {
	return func(c *gin.Context) {
		if pubKeys, err := s.b.GetPublicKeys(c.Query("Email")); err == nil && len(pubKeys) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"Keys":          pubKeys,
				"RecipientType": proton.RecipientTypeInternal,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"RecipientType": proton.RecipientTypeExternal,
			})
		}
	}
}

func (s *Server) handleGetKeySalts() gin.HandlerFunc {
	return func(c *gin.Context) {
		salts, err := s.b.GetKeySalts(c.GetString("UserID"))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"KeySalts": salts,
		})
	}
}
