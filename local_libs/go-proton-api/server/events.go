package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		event, more, err := s.b.GetEvent(c.GetString("UserID"), c.Param("eventID"))
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.JSON(
			http.StatusOK,
			struct {
				proton.Event
				More proton.Bool
			}{
				event,
				proton.Bool(more),
			},
		)
	}
}

func (s *Server) handleGetEventsLatest() gin.HandlerFunc {
	return func(c *gin.Context) {
		eventID, err := s.b.GetLatestEventID(c.GetString("UserID"))
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"EventID": eventID,
		})
	}
}
