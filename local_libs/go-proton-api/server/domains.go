package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleGetDomainsAvailable() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Domains": []string{s.domain},
		})
	}
}
