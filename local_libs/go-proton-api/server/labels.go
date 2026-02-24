package server

import (
	"net/http"
	"strconv"

	"github.com/bradenaw/juniper/xslices"
	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handleGetMailLabels() gin.HandlerFunc {
	return func(c *gin.Context) {
		types := xslices.Map(c.QueryArray("Type"), func(val string) proton.LabelType {
			labelType, err := strconv.Atoi(val)
			if err != nil {
				panic(err)
			}

			return proton.LabelType(labelType)
		})

		labels, err := s.b.GetLabels(c.GetString("UserID"), types...)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Labels": labels,
		})
	}
}

func (s *Server) handlePostMailLabels() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.CreateLabelReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if _, has, err := s.b.HasLabel(c.GetString("UserID"), req.Name); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if has {
			c.AbortWithStatus(http.StatusConflict)
			return
		}

		label, err := s.b.CreateLabel(c.GetString("UserID"), req.Name, req.ParentID, req.Type)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Label": label,
		})
	}
}

func (s *Server) handlePutMailLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.UpdateLabelReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if labelID, has, err := s.b.HasLabel(c.GetString("UserID"), req.Name); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if has && labelID != c.Param("labelID") {
			c.AbortWithStatus(http.StatusConflict)
			return
		}

		label, err := s.b.UpdateLabel(c.GetString("UserID"), c.Param("labelID"), req.Name, req.ParentID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Label": label,
		})
	}
}

func (s *Server) handleDeleteMailLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.b.DeleteLabel(c.GetString("UserID"), c.Param("labelID")); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}
}
