package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetMailSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := s.b.GetMailSettings(c.GetString("UserID"))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"MailSettings": settings,
		})
	}
}

func (s *Server) handlePutMailSettingsAttachPublicKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SetAttachPublicKeyReq

		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		settings, err := s.b.SetMailSettingsAttachPublicKey(c.GetString("UserID"), bool(req.AttachPublicKey))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"MailSettings": settings,
		})
	}
}
