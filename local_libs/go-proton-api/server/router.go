package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
)

func initRouter(s *Server) {
	s.r.Use(
		s.requireValidAppVersion(),
		s.setSessionCookie(),
		s.applyStatusHooks(),
		s.applyRateLimit(),
	)

	if core := s.r.Group("/core/v4"); core != nil {
		// Domains routes don't need authentication.
		if domains := core.Group("/domains"); domains != nil {
			domains.GET("/available", s.handleGetDomainsAvailable())
		}

		// Reporting a bug is also possible without authentication.
		if reports := core.Group("/reports"); reports != nil {
			reports.POST("/bug", s.handlePostReportBug())
		}

		// These routes require auth.
		if core := core.Group("", s.requireAuth()); core != nil {
			if users := core.Group("/users"); users != nil {
				users.GET("", s.handleGetUsers())
			}

			if addresses := core.Group("/addresses"); addresses != nil {
				addresses.GET("", s.handleGetAddresses())
				addresses.GET("/:addressID", s.handleGetAddress())
				addresses.DELETE("/:addressID", s.handleDeleteAddress())
				addresses.PUT("/:addressID/enable", s.handlePutAddressEnable())
				addresses.PUT("/:addressID/disable", s.handlePutAddressDisable())
				addresses.PUT("/order", s.handlePutAddressesOrder())
			}

			if labels := core.Group("/labels"); labels != nil {
				labels.GET("", s.handleGetMailLabels())
				labels.POST("", s.handlePostMailLabels())
				labels.PUT("/:labelID", s.handlePutMailLabel())
				labels.DELETE("/:labelID", s.handleDeleteMailLabel())
			}

			if keys := core.Group("/keys"); keys != nil {
				keys.GET("", s.handleGetKeys())
				keys.GET("/salts", s.handleGetKeySalts())
			}

			if events := core.Group("/events"); events != nil {
				events.GET("/:eventID", s.handleGetEvents())
				events.GET("/latest", s.handleGetEventsLatest())
			}
			if settings := core.Group("/settings"); settings != nil {
				settings.GET("", s.handleGetUserSettings())
				settings.PUT("/telemetry", s.handlePutUserSettingsTelemetry())
				settings.PUT("/crashreports", s.handlePutUserSettingsCrashReports())
			}
		}
	}

	// All mail routes need authentication.
	if mail := s.r.Group("/mail/v4", s.requireAuth()); mail != nil {
		if settings := mail.Group("/settings"); settings != nil {
			settings.GET("", s.handleGetMailSettings())
			settings.PUT("/attachpublic", s.handlePutMailSettingsAttachPublicKey())
		}

		if messages := mail.Group("/messages"); messages != nil {
			messages.GET("", s.handleGetMailMessages())
			messages.POST("", s.handlePostMailMessages())
			messages.GET("/ids", s.handleGetMailMessageIDs())
			messages.GET("/:messageID", s.handleGetMailMessage())
			messages.POST("/:messageID", s.handlePostMailMessage())
			messages.PUT("/:messageID", s.handlePutMailMessage())
			messages.PUT("/read", s.handlePutMailMessagesRead())
			messages.PUT("/unread", s.handlePutMailMessagesUnread())
			messages.PUT("/label", s.handlePutMailMessagesLabel())
			messages.PUT("/unlabel", s.handlePutMailMessagesUnlabel())
			messages.POST("/import", s.handlePutMailMessagesImport())
			messages.PUT("/delete", s.handleDeleteMailMessages())
			messages.GET("/count", s.handleMessageGroupCount())
		}

		if attachments := mail.Group("/attachments"); attachments != nil {
			attachments.POST("", s.handlePostMailAttachments())
			attachments.GET(":attachID", s.handleGetMailAttachment())
		}
	}

	// All contacts routes need authentication.
	if contacts := s.r.Group("/contacts/v4", s.requireAuth()); contacts != nil {
		contacts.GET("/emails", s.handleGetContactsEmails())
	}

	// All data routes need authentication.
	if data := s.r.Group("/data/v1", s.requireAuth()); data != nil {
		if stats := data.Group("/stats"); stats != nil {
			stats.POST("", s.handlePostDataStats())
			stats.POST("/multiple", s.handlePostDataStatsMultiple())
		}
	}

	// Top level auth routes don't need authentication.
	if auth := s.r.Group("/auth/v4"); auth != nil {
		auth.POST("", s.handlePostAuth())
		auth.POST("/info", s.handlePostAuthInfo())
		auth.POST("/refresh", s.handlePostAuthRefresh())

		// These routes require auth.
		if auth := auth.Group("", s.requireAuth()); auth != nil {
			auth.DELETE("", s.handleDeleteAuth())

			if sessions := auth.Group("/sessions"); sessions != nil {
				sessions.GET("", s.handleGetAuthSessions())
				sessions.DELETE("", s.handleDeleteAuthSessions())
				sessions.DELETE("/:authUID", s.handleDeleteAuthSession())
			}
		}
	}

	// Test routes don't need authentication.
	if tests := s.r.Group("/tests"); tests != nil {
		tests.GET("/ping", s.handleGetPing())
	}

	// Quark routes don't need authentication.
	if quark := s.r.Group("/internal/quark"); quark != nil {
		quark.GET("/:command", s.handleQuarkCommand())
	}

	// Proxy any calls to the upstream server.
	if proxy := s.r.Group("/proxy"); proxy != nil {
		proxy.Any("/*path", s.handleProxy(proxy.BasePath()))
	}
}

func (s *Server) requireValidAppVersion() gin.HandlerFunc {
	return func(c *gin.Context) {
		appVersion := c.Request.Header.Get("x-pm-appversion")

		if appVersion == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, proton.APIError{
				Code:    proton.AppVersionMissingCode,
				Message: "Missing x-pm-appversion header",
			})
		} else if ok := s.validateAppVersion(appVersion); !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, proton.APIError{
				Code:    proton.AppVersionBadCode,
				Message: "This version of the app is no longer supported, please update to continue using the app",
			})
		}
	}
}

func (s *Server) setSessionCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		url, err := url.Parse(s.s.URL)
		if err != nil {
			panic(err)
		}

		host, _, err := net.SplitHostPort(url.Host)
		if err != nil {
			panic(err)
		}

		if cookie, err := c.Request.Cookie("Session-Id"); errors.Is(err, http.ErrNoCookie) {
			c.SetCookie("Session-Id", uuid.NewString(), int(90*24*time.Hour.Seconds()), "/", host, true, true)
		} else {
			c.SetCookie("Session-Id", cookie.Value, int(90*24*time.Hour.Seconds()), "/", host, true, true)
		}
	}
}

func (s *Server) applyStatusHooks() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.statusHooksLock.RLock()
		defer s.statusHooksLock.RUnlock()

		for _, hook := range s.statusHooks {
			if status, ok := hook(c.Request); ok {
				c.AbortWithStatusJSON(status, proton.APIError{
					Code:    proton.InvalidValue,
					Message: fmt.Sprintf("Request failed with status %d", status),
				})

				return
			}
		}
	}
}

func (s *Server) applyRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.rateLimit == nil {
			return
		}

		if wait := s.rateLimit.exceeded(); wait > 0 {
			c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())))
			c.AbortWithStatus(s.rateLimit.statusCode)
		}
	}
}

func (s *Server) logCalls() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		req, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		} else {
			c.Request.Body = io.NopCloser(bytes.NewReader(req))
		}

		res, err := newBodyWriter(c.Writer)
		if err != nil {
			panic(err)
		} else {
			c.Writer = res
		}

		c.Next()

		s.callWatchersLock.RLock()
		defer s.callWatchersLock.RUnlock()

		for _, call := range s.callWatchers {
			if call.isWatching(c.Request.URL.Path) {
				call.publish(Call{
					URL:    c.Request.URL,
					Method: c.Request.Method,
					Status: c.Writer.Status(),

					Time:     start,
					Duration: time.Since(start),

					RequestHeader: c.Request.Header,
					RequestBody:   req,

					ResponseHeader: c.Writer.Header(),
					ResponseBody:   res.bytes(),
				})
			}
		}
	}
}

func (s *Server) handleOffline() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.offline {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
	}
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authUID := c.Request.Header.Get("x-pm-uid")
		if authUID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		userID, err := s.b.VerifyAuth(authUID, strings.Split(auth, " ")[1])
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("UserID", userID)

		c.Set("AuthUID", authUID)
	}
}

func (s *Server) validateAppVersion(appVersion string) bool {
	if s.minAppVersion == nil {
		return true
	}

	split := strings.Split(appVersion, "_")

	if len(split) != 2 {
		return false
	}

	version, err := semver.NewVersion(split[1])
	if err != nil {
		return false
	}

	if version.LessThan(s.minAppVersion) {
		return false
	}

	return true
}

type bodyWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func newBodyWriter(w gin.ResponseWriter) (*bodyWriter, error) {
	if w == nil {
		return nil, errors.New("response writer is nil")
	}

	return &bodyWriter{
		ResponseWriter: w,

		buf: &bytes.Buffer{},
	}, nil
}

func (w bodyWriter) Write(b []byte) (int, error) {
	if n, err := w.buf.Write(b); err != nil {
		return n, err
	}

	return w.ResponseWriter.Write(b)
}

func (w bodyWriter) bytes() []byte {
	return w.buf.Bytes()
}
