package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleGetUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := s.b.GetUser(c.GetString("UserID"))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"User": user,
		})
	}
}
