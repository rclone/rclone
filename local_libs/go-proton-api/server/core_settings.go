package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetUserSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := s.b.GetUserSettings(c.GetString("UserID"))
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"UserSettings": settings,
		})
	}
}

func (s *Server) handlePutUserSettingsTelemetry() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SetTelemetryReq

		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		settings, err := s.b.SetUserSettingsTelemetry(c.GetString("UserID"), req.Telemetry)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"UserSettings": settings,
		})
	}
}

func (s *Server) handlePutUserSettingsCrashReports() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SetCrashReportReq

		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		settings, err := s.b.SetUserSettingsCrashReports(c.GetString("UserID"), req.CrashReports)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"UserSettings": settings,
		})
	}
}
