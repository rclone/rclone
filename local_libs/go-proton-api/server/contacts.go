package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetContactsEmails() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ContactEmails": []proton.ContactEmail{},
		})
	}
}
