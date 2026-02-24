package server

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handlePostAuthInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.AuthInfoReq

		if err := c.BindJSON(&req); err != nil {
			return
		}

		info, err := s.b.NewAuthInfo(req.Username)
		if err != nil {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}

		c.JSON(http.StatusOK, info)
	}
}

func (s *Server) handlePostAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.AuthReq

		if err := c.BindJSON(&req); err != nil {
			return
		}

		clientEphemeral, err := base64.StdEncoding.DecodeString(req.ClientEphemeral)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		clientProof, err := base64.StdEncoding.DecodeString(req.ClientProof)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		auth, err := s.b.NewAuth(req.Username, clientEphemeral, clientProof, req.SRPSession)
		if err != nil {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}

		c.JSON(http.StatusOK, auth)
	}
}

func (s *Server) handlePostAuthRefresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.AuthRefreshReq

		if err := c.BindJSON(&req); err != nil {
			return
		}

		auth, err := s.b.NewAuthRef(req.UID, req.RefreshToken)
		if err != nil {
			_ = c.AbortWithError(http.StatusUnprocessableEntity, err)
			return
		}

		c.JSON(http.StatusOK, auth)
	}
}

func (s *Server) handleDeleteAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.b.DeleteSession(c.GetString("UserID"), c.GetString("AuthUID")); err != nil {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
	}
}

func (s *Server) handleGetAuthSessions() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := s.b.GetSessions(c.GetString("UserID"))
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"Sessions": sessions})
	}
}

func (s *Server) handleDeleteAuthSessions() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := s.b.GetSessions(c.GetString("UserID"))
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		for _, session := range sessions {
			if session.UID != c.GetString("AuthUID") {
				if err := s.b.DeleteSession(c.GetString("UserID"), session.UID); err != nil {
					_ = c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
		}
	}
}

func (s *Server) handleDeleteAuthSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.b.DeleteSession(c.GetString("UserID"), c.Param("authUID")); err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}
}
