package proton

import (
	"bytes"
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Quark runs a quark command.
func (m *Manager) Quark(ctx context.Context, command string, args ...string) error {
	if _, err := m.r(ctx).SetQueryParam("strInput", strings.Join(args, " ")).Get("/internal/quark/" + command); err != nil {
		return err
	}

	return nil
}

// QuarkRes is the same as Quark, but returns the content extracted from the response body.
func (m *Manager) QuarkRes(ctx context.Context, command string, args ...string) ([]byte, error) {
	res, err := m.r(ctx).SetQueryParam("strInput", strings.Join(args, " ")).Get("/internal/quark/" + command)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(bytes.NewReader(res.Body()))
	if err != nil {
		return nil, err
	}

	return []byte(strings.TrimSpace(goquery.NewDocumentFromNode(doc).Find(".content").Text())), nil
}
