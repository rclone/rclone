package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
	"golang.org/x/exp/slices"
)

const (
	defaultPage     = 0
	defaultPageSize = 100
)

func (s *Server) handleGetMailMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.getMailMessages(
			c,
			mustParseInt(c.DefaultQuery("Page", strconv.Itoa(defaultPage))),
			mustParseInt(c.DefaultQuery("PageSize", strconv.Itoa(defaultPageSize))),
			proton.MessageFilter{ID: c.QueryArray("ID")},
		)
	}
}

func (s *Server) getMailMessages(c *gin.Context, page, pageSize int, filter proton.MessageFilter) {
	// Set default page.
	if page <= 0 {
		page = defaultPage
	}

	// Set default page size.
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	messages, err := s.b.GetMessages(c.GetString("UserID"), page, pageSize, filter)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	total, err := s.b.CountMessages(c.GetString("UserID"))
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Messages": messages,
		"Total":    total,
		"Stale":    proton.APIFalse,
	})
}

func (s *Server) handlePostMailMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.GetHeader("X-HTTP-Method-Override") {
		case "GET":
			var req struct {
				proton.MessageFilter

				Page     int
				PageSize int
			}

			if err := c.BindJSON(&req); err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			s.getMailMessages(c, req.Page, req.PageSize, req.MessageFilter)

		default:
			s.postMailMessages(c)
		}
	}
}

func (s *Server) postMailMessages(c *gin.Context) {
	var req proton.CreateDraftReq

	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	addrID, err := s.b.GetAddressID(req.Message.Sender.Address)
	if err != nil {
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	}

	message, err := s.b.CreateDraft(c.GetString("UserID"), addrID, req.Message, req.ParentID)
	if err != nil {
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Message": message,
	})
}

func (s *Server) handleGetMailMessageIDs() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := strconv.Atoi(c.Query("Limit"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		messageIDs, err := s.b.GetMessageIDs(c.GetString("UserID"), c.Query("AfterID"), limit)
		if err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"IDs": messageIDs,
		})
	}
}

func (s *Server) handleGetMailMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		message, err := s.b.GetMessage(c.GetString("UserID"), c.Param("messageID"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, proton.APIError{
				Code:    proton.InvalidValue,
				Message: fmt.Sprintf("Message %s not found", c.Param("messageID")),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Message": message,
		})
	}
}

func (s *Server) handlePostMailMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.SendDraftReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		message, err := s.b.SendMessage(c.GetString("UserID"), c.Param("messageID"), req.Packages)
		if err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Sent": message,
		})
	}
}

func (s *Server) handlePutMailMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.UpdateDraftReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		message, err := s.b.UpdateDraft(c.GetString("UserID"), c.Param("messageID"), req.Message)
		if err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Message": message,
		})
	}
}

func (s *Server) handlePutMailMessagesRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.MessageActionReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if err := s.b.SetMessagesRead(c.GetString("UserID"), true, req.IDs...); err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}
	}
}

func (s *Server) handlePutMailMessagesUnread() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.MessageActionReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if err := s.b.SetMessagesRead(c.GetString("UserID"), false, req.IDs...); err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}
	}
}

func (s *Server) handlePutMailMessagesLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.LabelMessagesReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if err := s.b.LabelMessages(c.GetString("UserID"), req.LabelID, req.IDs...); err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}
	}
}

func (s *Server) handlePutMailMessagesUnlabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.LabelMessagesReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if err := s.b.UnlabelMessages(c.GetString("UserID"), req.LabelID, req.IDs...); err != nil {
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}
	}
}

func (s *Server) handlePutMailMessagesImport() gin.HandlerFunc {
	return func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		var metadata map[string]proton.ImportMetadata

		if err := json.Unmarshal([]byte(form.Value["Metadata"][0]), &metadata); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		files := make(map[string][]byte)

		for name, file := range form.File {
			files[name] = mustReadFileHeader(file[0])
		}

		type response struct {
			Name     string
			Response proton.ImportRes
		}

		var responses []response

		for name, literal := range files {
			res := response{Name: name}

			messageID, err := s.importMessage(
				c.GetString("UserID"),
				metadata[name].AddressID,
				metadata[name].LabelIDs,
				literal,
				metadata[name].Flags,
				bool(metadata[name].Unread),
			)
			if err != nil {
				res.Response = proton.ImportRes{
					APIError: proton.APIError{
						Code:    proton.InvalidValue,
						Message: fmt.Sprintf("failed to import: %v", err),
					},
				}
			} else {
				res.Response = proton.ImportRes{
					APIError:  proton.APIError{Code: proton.SuccessCode},
					MessageID: messageID,
				}
			}

			responses = append(responses, res)
		}

		c.JSON(http.StatusOK, gin.H{
			"Code":      proton.MultiCode,
			"Responses": responses,
		})
	}
}

func (s *Server) handleDeleteMailMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req proton.MessageActionReq

		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		for _, messageID := range req.IDs {
			if err := s.b.DeleteMessage(c.GetString("UserID"), messageID); err != nil {
				c.AbortWithStatus(http.StatusUnprocessableEntity)
				return
			}
		}
	}
}

func (s *Server) handleMessageGroupCount() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := s.b.GetMessageGroupCount(c.GetString("UserID"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, proton.APIError{
				Code:    proton.InvalidValue,
				Message: fmt.Sprintf("Message %s not found", c.Param("messageID")),
			})

			return
		}

		c.JSON(http.StatusOK, gin.H{
			"Counts": count,
		})
	}
}

func (s *Server) importMessage(
	userID, addrID string,
	labelIDs []string,
	literal []byte,
	flags proton.MessageFlag,
	unread bool,
) (string, error) {
	var exclusive int

	for _, labelID := range labelIDs {
		switch labelID {
		case proton.AllDraftsLabel, proton.AllSentLabel, proton.AllMailLabel, proton.OutboxLabel:
			return "", fmt.Errorf("invalid label ID: %s", labelID)
		}

		label, err := s.b.GetLabel(userID, labelID)
		if err != nil {
			return "", fmt.Errorf("invalid label ID: %s", labelID)
		}

		if label.Type != proton.LabelTypeLabel {
			exclusive++
		}
	}

	if exclusive > 1 {
		return "", fmt.Errorf("too many exclusive labels")
	}

	header, body, atts, mimeType, err := s.parseMessage(literal)
	if err != nil {
		return "", fmt.Errorf("failed to parse message: %w", err)
	}

	messageID, err := s.importBody(userID, addrID, header, body, mimeType, flags, unread, slices.Contains(labelIDs, proton.StarredLabel))
	if err != nil {
		return "", fmt.Errorf("failed to import message: %w", err)
	}

	for _, att := range atts {
		if _, err := s.importAttachment(userID, messageID, att); err != nil {
			return "", fmt.Errorf("failed to import attachment: %w", err)
		}
	}

	for _, labelID := range labelIDs {
		if err := s.b.LabelMessagesNoEvents(userID, labelID, messageID); err != nil {
			return "", fmt.Errorf("failed to label message: %w", err)
		}
	}

	return messageID, nil
}

func (s *Server) parseMessage(literal []byte) (*rfc822.Header, []string, []*rfc822.Section, rfc822.MIMEType, error) {
	root := rfc822.Parse(literal)

	header, err := root.ParseHeader()
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to parse header: %w", err)
	}

	body, atts, err := collect(root)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to collect body and attachments: %w", err)
	}

	mimeType, _, err := root.ContentType()
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to parse content type: %w", err)
	}

	// Force all multipart types to be multipart/mixed.
	if mimeType.Type() == "multipart" {
		mimeType = "multipart/mixed"
	}

	return header, body, atts, mimeType, nil
}

func collect(section *rfc822.Section) ([]string, []*rfc822.Section, error) {
	mimeType, _, err := section.ContentType()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse content type: %w", err)
	}

	switch mimeType.Type() {
	case "text":
		return []string{string(section.Body())}, nil, nil

	case "multipart":
		children, err := section.Children()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse children: %w", err)
		}

		switch mimeType.SubType() {
		case "encrypted":
			if len(children) != 2 {
				return nil, nil, fmt.Errorf("expected two children for multipart/encrypted, got %d", len(children))
			}

			return []string{string(children[1].Body())}, nil, nil

		default:
			var (
				multiBody []string
				multiAtts []*rfc822.Section
			)

			for _, child := range children {
				body, atts, err := collect(child)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to collect child: %w", err)
				}

				multiBody = append(multiBody, body...)
				multiAtts = append(multiAtts, atts...)
			}

			return multiBody, multiAtts, nil
		}

	default:
		return nil, []*rfc822.Section{section}, nil
	}
}

func (s *Server) importBody(
	userID, addrID string,
	header *rfc822.Header,
	body []string,
	mimeType rfc822.MIMEType,
	flags proton.MessageFlag,
	unread, starred bool,
) (string, error) {
	subject := header.Get("Subject")
	sender := tryParseAddress(header.Get("From"))
	toList := tryParseAddressList(header.Get("To"))
	ccList := tryParseAddressList(header.Get("Cc"))
	bccList := tryParseAddressList(header.Get("Bcc"))
	replytos := tryParseAddressList(header.Get("Reply-To"))
	date := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	headerDate := header.Get("Date")
	if len(headerDate) != 0 {
		d, err := mail.ParseDate(headerDate)
		if err != nil {
			return "", err
		}

		date = d
	}

	// NOTE: Importing without sender adds empty sender on API side
	if sender == nil {
		sender = &mail.Address{}
	}

	// NOTE: Importing without sender adds empty reply to on API side
	if len(replytos) == 0 {
		replytos = []*mail.Address{{}}
	}

	// NOTE: Importing just the first body part matches API behaviour but sucks!
	return s.b.CreateMessage(
		userID, addrID,
		subject,
		sender,
		toList, ccList, bccList, replytos,
		string(body[0]),
		rfc822.MIMEType(mimeType),
		flags,
		date,
		unread, starred,
	)
}

func (s *Server) importAttachment(userID, messageID string, att *rfc822.Section) (proton.Attachment, error) {
	header, err := att.ParseHeader()
	if err != nil {
		return proton.Attachment{}, fmt.Errorf("failed to parse attachment header: %w", err)
	}

	mimeType, _, err := att.ContentType()
	if err != nil {
		return proton.Attachment{}, fmt.Errorf("failed to parse attachment content type: %w", err)
	}

	var disposition, filename string

	if !header.Has("Content-Disposition") {
		disposition = "attachment"
		filename = "attachment.bin"
	} else if dispType, dispParams, err := mime.ParseMediaType(header.Get("Content-Disposition")); err == nil {
		disposition = dispType
		filename = dispParams["filename"]
	} else {
		disposition = "attachment"
		filename = "attachment.bin"
	}

	var body *crypto.PGPSplitMessage

	if header.Get("Content-Transfer-Encoding") == "base64" {
		b := make([]byte, base64.StdEncoding.DecodedLen(len(att.Body())))

		n, err := base64.StdEncoding.Decode(b, att.Body())
		if err != nil {
			return proton.Attachment{}, fmt.Errorf("failed to decode attachment body: %w", err)
		}

		split, err := crypto.NewPGPMessage(b[:n]).SplitMessage()
		if err != nil {
			return proton.Attachment{}, fmt.Errorf("failed to split attachment body: %w", err)
		}

		body = split
	} else {
		msg, err := crypto.NewPGPMessageFromArmored(string(att.Body()))
		if err != nil {
			return proton.Attachment{}, fmt.Errorf("failed to parse attachment body: %w", err)
		}

		split, err := msg.SplitMessage()
		if err != nil {
			return proton.Attachment{}, fmt.Errorf("failed to split attachment body: %w", err)
		}

		body = split
	}

	// TODO: What about the signature?
	return s.b.CreateAttachment(
		userID, messageID,
		filename,
		mimeType,
		proton.Disposition(disposition),
		header.Get("Content-Id"),
		body.GetBinaryKeyPacket(),
		body.GetBinaryDataPacket(),
		"",
	)
}

func tryParseAddress(s string) *mail.Address {
	if s == "" {
		return nil
	}

	addr, err := mail.ParseAddress(s)
	if err != nil {
		return &mail.Address{
			Name: s,
		}
	}

	return addr
}

func tryParseAddressList(s string) []*mail.Address {
	if s == "" {
		return nil
	}

	addrs, err := mail.ParseAddressList(s)
	if err != nil {
		return []*mail.Address{{
			Name: s,
		}}
	}

	return addrs
}

func mustParseInt(num string) int {
	val, err := strconv.Atoi(num)
	if err != nil {
		panic(err)
	}

	return val
}
