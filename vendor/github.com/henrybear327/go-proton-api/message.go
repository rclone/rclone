package proton

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strconv"

	"github.com/ProtonMail/gluon/async"
	"github.com/bradenaw/juniper/parallel"
	"github.com/bradenaw/juniper/xslices"
	"github.com/go-resty/resty/v2"
)

const maxMessageIDs = 1000

func (c *Client) GetFullMessage(ctx context.Context, messageID string, scheduler Scheduler, storageProvider AttachmentAllocator) (FullMessage, error) {
	message, err := c.GetMessage(ctx, messageID)
	if err != nil {
		return FullMessage{}, err
	}

	attDataBuffers, err := scheduler.Schedule(ctx, xslices.Map(message.Attachments, func(att Attachment) string {
		return att.ID
	}), storageProvider, func(ctx context.Context, s string, buffer *bytes.Buffer) error {
		return c.GetAttachmentInto(ctx, s, buffer)
	})
	if err != nil {
		return FullMessage{}, err
	}

	return FullMessage{
		Message: message,
		AttData: xslices.Map(attDataBuffers, func(b *bytes.Buffer) []byte {
			return b.Bytes()
		}),
	}, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string) (Message, error) {
	var res struct {
		Message Message
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/mail/v4/messages/" + messageID)
	}); err != nil {
		return Message{}, err
	}

	return res.Message, nil
}

func (c *Client) CountMessages(ctx context.Context) (int, error) {
	return c.countMessages(ctx, MessageFilter{})
}

func (c *Client) GetMessageMetadata(ctx context.Context, filter MessageFilter) ([]MessageMetadata, error) {
	count, err := c.countMessages(ctx, filter)
	if err != nil {
		return nil, err
	}

	return fetchPaged(ctx, count, maxPageSize, c, func(ctx context.Context, page, pageSize int) ([]MessageMetadata, error) {
		return c.GetMessageMetadataPage(ctx, page, pageSize, filter)
	})
}

func (c *Client) GetAllMessageIDs(ctx context.Context, afterID string) ([]string, error) {
	var messageIDs []string

	for ; ; afterID = messageIDs[len(messageIDs)-1] {
		page, err := c.GetMessageIDs(ctx, afterID, maxMessageIDs)
		if err != nil {
			return nil, err
		}

		if len(page) == 0 {
			return messageIDs, nil
		}

		messageIDs = append(messageIDs, page...)
	}
}

func (c *Client) DeleteMessage(ctx context.Context, messageIDs ...string) error {
	pages := xslices.Chunk(messageIDs, maxPageSize)

	return parallel.DoContext(ctx, runtime.NumCPU(), len(pages), func(ctx context.Context, idx int) error {
		defer async.HandlePanic(c.m.panicHandler)

		return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(MessageActionReq{IDs: pages[idx]}).Put("/mail/v4/messages/delete")
		})
	})
}

func (c *Client) MarkMessagesRead(ctx context.Context, messageIDs ...string) error {
	for _, page := range xslices.Chunk(messageIDs, maxPageSize) {
		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(MessageActionReq{IDs: page}).Put("/mail/v4/messages/read")
		}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) MarkMessagesUnread(ctx context.Context, messageIDs ...string) error {
	for _, page := range xslices.Chunk(messageIDs, maxPageSize) {
		req := MessageActionReq{IDs: page}

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(req).Put("/mail/v4/messages/unread")
		}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) LabelMessages(ctx context.Context, messageIDs []string, labelID string) error {
	var results []LabelMessagesRes

	for _, chunk := range xslices.Chunk(messageIDs, maxPageSize) {
		var res LabelMessagesRes

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(LabelMessagesReq{
				LabelID: labelID,
				IDs:     chunk,
			}).SetResult(&res).Put("/mail/v4/messages/label")
		}); err != nil {
			return err
		}

		if ok, errStr := res.ok(); !ok {
			tokens := xslices.Map(results, func(res LabelMessagesRes) UndoToken {
				return res.UndoToken
			})

			if _, undoErr := c.UndoActions(ctx, tokens...); undoErr != nil {
				return fmt.Errorf("failed to undo label actions (undo reason: %v): %w", errStr, undoErr)
			}

			return fmt.Errorf("failed to label messages: %v", errStr)
		}

		results = append(results, res)
	}

	return nil
}

func (c *Client) UnlabelMessages(ctx context.Context, messageIDs []string, labelID string) error {
	var results []LabelMessagesRes

	for _, chunk := range xslices.Chunk(messageIDs, maxPageSize) {
		var res LabelMessagesRes

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(LabelMessagesReq{
				LabelID: labelID,
				IDs:     chunk,
			}).SetResult(&res).Put("/mail/v4/messages/unlabel")
		}); err != nil {
			return err
		}

		if ok, errStr := res.ok(); !ok {
			tokens := xslices.Map(results, func(res LabelMessagesRes) UndoToken {
				return res.UndoToken
			})

			if _, undoErr := c.UndoActions(ctx, tokens...); undoErr != nil {
				return fmt.Errorf("failed to undo unlabel actions (undo reason: %v): %w", errStr, undoErr)
			}

			return fmt.Errorf("failed to unlabel messages: %v", errStr)

		}

		results = append(results, res)
	}

	return nil
}

func (c *Client) GetMessageIDs(ctx context.Context, afterID string, limit int) ([]string, error) {
	if limit > maxMessageIDs {
		limit = maxMessageIDs
	}

	var res struct {
		IDs []string
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		if afterID != "" {
			r = r.SetQueryParam("AfterID", afterID)
		}

		return r.SetQueryParam("Limit", strconv.Itoa(limit)).SetResult(&res).Get("/mail/v4/messages/ids")
	}); err != nil {
		return nil, err
	}

	return res.IDs, nil
}

func (c *Client) countMessages(ctx context.Context, filter MessageFilter) (int, error) {
	var res struct {
		Total int
	}

	req := struct {
		MessageFilter

		Limit int `json:",,string"`
	}{
		MessageFilter: filter,

		Limit: 0,
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).SetResult(&res).SetHeader("X-HTTP-Method-Override", "GET").Post("/mail/v4/messages")
	}); err != nil {
		return 0, err
	}

	return res.Total, nil
}

func (c *Client) GetMessageMetadataPage(ctx context.Context, page, pageSize int, filter MessageFilter) ([]MessageMetadata, error) {
	var res struct {
		Messages []MessageMetadata
		Stale    Bool
	}

	req := struct {
		MessageFilter

		Page     int
		PageSize int

		Sort string
	}{
		MessageFilter: filter,

		Page:     page,
		PageSize: pageSize,

		Sort: "ID",
	}

	for {
		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(req).SetResult(&res).SetHeader("X-HTTP-Method-Override", "GET").Post("/mail/v4/messages")
		}); err != nil {
			return nil, err
		}

		if !res.Stale {
			break
		}
	}

	return res.Messages, nil
}

func (c *Client) GetGroupedMessageCount(ctx context.Context) ([]MessageGroupCount, error) {
	var res struct {
		Counts []MessageGroupCount
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/mail/v4/messages/count")
	}); err != nil {
		return nil, err
	}

	return res.Counts, nil
}
