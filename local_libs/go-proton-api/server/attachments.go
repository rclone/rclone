package server

import (
	"io"
	"mime/multipart"
	"net/http"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
)

func (s *Server) handlePostMailAttachments() gin.HandlerFunc {
	return func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		attachment, err := s.b.CreateAttachment(
			c.GetString("UserID"),
			form.Value["MessageID"][0],
			form.Value["Filename"][0],
			rfc822.MIMEType(form.Value["MIMEType"][0]),
			proton.Disposition(form.Value["Disposition"][0]),
			form.Value["ContentID"][0],
			mustReadFileHeader(form.File["KeyPackets"][0]),
			mustReadFileHeader(form.File["DataPacket"][0]),
			string(mustReadFileHeader(form.File["Signature"][0])),
		)
		if err != nil {
			_ = c.AbortWithError(http.StatusUnprocessableEntity, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Attachment": attachment,
		})
	}
}

func (s *Server) handleGetMailAttachment() gin.HandlerFunc {
	return func(c *gin.Context) {
		attData, err := s.b.GetAttachment(c.Param("attachID"))
		if err != nil {
			_ = c.AbortWithError(http.StatusUnprocessableEntity, err)
			return
		}

		c.Data(http.StatusOK, "application/octet-stream", attData)
	}
}

func mustReadFileHeader(fh *multipart.FileHeader) []byte {
	f, err := fh.Open()
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	return data
}
