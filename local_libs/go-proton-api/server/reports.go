package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handlePostReportBug() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := c.MultipartForm(); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}
}
