package server

import (
	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
	"net/http"
)

func (s *Server) handlePostDataStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SendStatsReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if !validateSendStatReq(&req) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Code": proton.SuccessCode,
		})
	}
}

func (s *Server) handlePostDataStatsMultiple() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SendStatsMultiReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		for _, event := range req.EventInfo {
			if !validateSendStatReq(&event) {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"Code": proton.SuccessCode,
		})
	}
}

func validateSendStatReq(req *proton.SendStatsReq) bool {
	return req.MeasurementGroup != ""
}
