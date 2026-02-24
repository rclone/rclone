package proton

import (
	"bytes"
	"context"
)

func (m *Manager) ReportBug(ctx context.Context, req ReportBugReq, atts ...ReportBugAttachment) error {
	r := m.r(ctx).SetMultipartFormData(req.toFormData())

	for _, att := range atts {
		r = r.SetMultipartField(att.Name, att.Filename, string(att.MIMEType), bytes.NewReader(att.Body))
	}

	if _, err := r.Post("/core/v4/reports/bug"); err != nil {
		return err
	}

	return nil
}
