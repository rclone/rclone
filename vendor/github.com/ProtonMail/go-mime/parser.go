package gomime

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
)

// VisitAcceptor decidest what to do with part which is processed
// It is used by MIMEVisitor
type VisitAcceptor interface {
	Accept(partReader io.Reader, header textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error)
}

func VisitAll(part io.Reader, h textproto.MIMEHeader, accepter VisitAcceptor) (err error) {
	mediaType, _, err := getContentType(h)
	if err != nil {
		return
	}
	return accepter.Accept(part, h, mediaType == "text/plain", true, true)
}

func IsLeaf(h textproto.MIMEHeader) bool {
	return !strings.HasPrefix(h.Get("Content-Type"), "multipart/")
}

// MIMEVisitor is main object to parse (visit) and process (accept) all parts of MIME message
type MimeVisitor struct {
	target VisitAcceptor
}

// Accept reads part recursively if needed
// hasPlainSibling is there when acceptor want to check alternatives
func (mv *MimeVisitor) Accept(part io.Reader, h textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error) {
	if !isFirst {
		return
	}

	parentMediaType, params, err := getContentType(h)
	if err != nil {
		return
	}

	if err = mv.target.Accept(part, h, hasPlainSibling, true, false); err != nil {
		return
	}

	if !IsLeaf(h) {
		var multiparts []io.Reader
		var multipartHeaders []textproto.MIMEHeader
		if multiparts, multipartHeaders, err = GetMultipartParts(part, params); err != nil {
			return
		}
		hasPlainChild := false
		for _, header := range multipartHeaders {
			mediaType, _, _ := getContentType(header)
			if mediaType == "text/plain" {
				hasPlainChild = true
			}
		}
		if hasPlainSibling && parentMediaType == "multipart/related" {
			hasPlainChild = true
		}

		for i, p := range multiparts {
			if err = mv.Accept(p, multipartHeaders[i], hasPlainChild, true, true); err != nil {
				return
			}
			if err = mv.target.Accept(part, h, hasPlainSibling, false, i == (len(multiparts)-1)); err != nil {
				return
			}
		}
	}
	return
}

// NewMIMEVisitor initialiazed with acceptor
func NewMimeVisitor(targetAccepter VisitAcceptor) *MimeVisitor {
	return &MimeVisitor{targetAccepter}
}

func GetRawMimePart(rawdata io.Reader, boundary string) (io.Reader, io.Reader) {
	b, _ := ioutil.ReadAll(rawdata)
	tee := bytes.NewReader(b)

	reader := bufio.NewReader(bytes.NewReader(b))
	byteBoundary := []byte(boundary)
	bodyBuffer := &bytes.Buffer{}
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return tee, bytes.NewReader(bodyBuffer.Bytes())
		}
		if bytes.HasPrefix(line, byteBoundary) {
			break
		}
	}
	lineEndingLength := 0
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return tee, bytes.NewReader(bodyBuffer.Bytes())
		}
		if bytes.HasPrefix(line, byteBoundary) {
			break
		}
		lineEndingLength = 0
		bodyBuffer.Write(line)
		if !isPrefix {
			reader.UnreadByte()
			reader.UnreadByte()
			token, _ := reader.ReadByte()
			if token == '\r' {
				lineEndingLength++
				bodyBuffer.WriteByte(token)
			}
			lineEndingLength++
			bodyBuffer.WriteByte(token)
		}
	}
	ioutil.ReadAll(reader)
	data := bodyBuffer.Bytes()
	return tee, bytes.NewReader(data[0 : len(data)-lineEndingLength])
}

func GetAllChildParts(part io.Reader, h textproto.MIMEHeader) (parts []io.Reader, headers []textproto.MIMEHeader, err error) {
	mediaType, params, err := getContentType(h)
	if err != nil {
		return
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		var multiparts []io.Reader
		var multipartHeaders []textproto.MIMEHeader
		if multiparts, multipartHeaders, err = GetMultipartParts(part, params); err != nil {
			return
		}
		if strings.Contains(mediaType, "alternative") {
			var chosenPart io.Reader
			var chosenHeader textproto.MIMEHeader
			if chosenPart, chosenHeader, err = pickAlternativePart(multiparts, multipartHeaders); err != nil {
				return
			}
			var childParts []io.Reader
			var childHeaders []textproto.MIMEHeader
			if childParts, childHeaders, err = GetAllChildParts(chosenPart, chosenHeader); err != nil {
				return
			}
			parts = append(parts, childParts...)
			headers = append(headers, childHeaders...)
		} else {
			for i, p := range multiparts {
				var childParts []io.Reader
				var childHeaders []textproto.MIMEHeader
				if childParts, childHeaders, err = GetAllChildParts(p, multipartHeaders[i]); err != nil {
					return
				}
				parts = append(parts, childParts...)
				headers = append(headers, childHeaders...)
			}
		}
	} else {
		parts = append(parts, part)
		headers = append(headers, h)
	}
	return
}

func GetMultipartParts(r io.Reader, params map[string]string) (parts []io.Reader, headers []textproto.MIMEHeader, err error) {
	mr := multipart.NewReader(r, params["boundary"])
	parts = []io.Reader{}
	headers = []textproto.MIMEHeader{}
	var p *multipart.Part
	for {
		p, err = mr.NextRawPart()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		b, _ := ioutil.ReadAll(p)
		buffer := bytes.NewBuffer(b)

		parts = append(parts, buffer)
		headers = append(headers, p.Header)
	}
	return
}

func pickAlternativePart(parts []io.Reader, headers []textproto.MIMEHeader) (part io.Reader, h textproto.MIMEHeader, err error) {

	for i, h := range headers {
		mediaType, _, err := getContentType(h)
		if err != nil {
			continue
		}
		if strings.HasPrefix(mediaType, "multipart/") {
			return parts[i], headers[i], nil
		}
	}
	for i, h := range headers {
		mediaType, _, err := getContentType(h)
		if err != nil {
			continue
		}
		if mediaType == "text/html" {
			return parts[i], headers[i], nil
		}
	}
	for i, h := range headers {
		mediaType, _, err := getContentType(h)
		if err != nil {
			continue
		}
		if mediaType == "text/plain" {
			return parts[i], headers[i], nil
		}
	}
	//if we get all the way here, part will be nil
	return
}

// Parse address comment as defined in http://tools.wordtothewise.com/rfc/822
// FIXME: Does not work for address groups
// NOTE: This should be removed for go>1.10 (please check)
func parseAddressComment(raw string) string {
	parsed := []string{}
	for _, item := range regexp.MustCompile("[,;]").Split(raw, -1) {
		re := regexp.MustCompile("[(][^)]*[)]")
		comments := strings.Join(re.FindAllString(item, -1), " ")
		comments = strings.Replace(comments, "(", "", -1)
		comments = strings.Replace(comments, ")", "", -1)
		withoutComments := re.ReplaceAllString(item, "")
		addr, err := mail.ParseAddress(withoutComments)
		if err != nil {
			continue
		}
		if addr.Name == "" {
			addr.Name = comments
		}
		parsed = append(parsed, addr.String())
	}
	return strings.Join(parsed, ", ")
}

func checkHeaders(headers []textproto.MIMEHeader) bool {
	foundAttachment := false

	for i := 0; i < len(headers); i++ {
		h := headers[i]

		mediaType, _, _ := getContentType(h)

		if !strings.HasPrefix(mediaType, "text/") {
			foundAttachment = true
		} else if foundAttachment {
			//this means that there is a text part after the first attachment, so we will have to convert the body from plain->HTML
			return true
		}
	}
	return false
}

func decodePart(partReader io.Reader, header textproto.MIMEHeader) (decodedPart io.Reader) {
	decodedPart = DecodeContentEncoding(partReader, header.Get("Content-Transfer-Encoding"))
	if decodedPart == nil {
		log.Printf("Unsupported Content-Transfer-Encoding '%v'", header.Get("Content-Transfer-Encoding"))
		decodedPart = partReader
	}
	return
}

// assume 'text/plain' if missing
func getContentType(header textproto.MIMEHeader) (mediatype string, params map[string]string, err error) {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	return mime.ParseMediaType(contentType)
}

// ===================== MIME Printer ===================================
// Simply print resulting MIME tree into text form
// TODO to file mime_printer.go
type stack []string

func (s stack) Push(v string) stack {
	return append(s, v)
}
func (s stack) Pop() (stack, string) {
	l := len(s)
	return s[:l-1], s[l-1]
}
func (s stack) Peek() string {
	return s[len(s)-1]
}

type MIMEPrinter struct {
	result        *bytes.Buffer
	boundaryStack stack
}

func NewMIMEPrinter() (pd *MIMEPrinter) {
	return &MIMEPrinter{
		result:        bytes.NewBuffer([]byte("")),
		boundaryStack: stack{},
	}
}

func (pd *MIMEPrinter) Accept(partReader io.Reader, header textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error) {
	if isFirst {
		http.Header(header).Write(pd.result)
		pd.result.Write([]byte("\n"))
		if IsLeaf(header) {
			pd.result.ReadFrom(partReader)
		} else {
			_, params, _ := getContentType(header)
			boundary := params["boundary"]
			pd.boundaryStack = pd.boundaryStack.Push(boundary)
			pd.result.Write([]byte("\nThis is a multi-part message in MIME format.\n--" + boundary + "\n"))
		}
	} else {
		if !isLast {
			pd.result.Write([]byte("\n--" + pd.boundaryStack.Peek() + "\n"))
		} else {
			var boundary string
			pd.boundaryStack, boundary = pd.boundaryStack.Pop()
			pd.result.Write([]byte("\n--" + boundary + "--\n.\n"))
		}
	}
	return nil
}

func (pd *MIMEPrinter) String() string {
	return pd.result.String()
}

// ======================== PlainText Collector  =========================
// Collect contents of all non-attachment text/plain parts and return
// it is a string
// TODO to file collector_plaintext.go

type PlainTextCollector struct {
	target            VisitAcceptor
	plainTextContents *bytes.Buffer
}

func NewPlainTextCollector(targetAccepter VisitAcceptor) *PlainTextCollector {
	return &PlainTextCollector{
		target:            targetAccepter,
		plainTextContents: bytes.NewBuffer([]byte("")),
	}
}

func (ptc *PlainTextCollector) Accept(partReader io.Reader, header textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error) {
	if isFirst {
		if IsLeaf(header) {
			mediaType, params, _ := getContentType(header)
			disp, _, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
			if mediaType == "text/plain" && disp != "attachment" {
				partData, _ := ioutil.ReadAll(partReader)
				decodedPart := decodePart(bytes.NewReader(partData), header)

				if buffer, err := ioutil.ReadAll(decodedPart); err == nil {
					buffer, err = DecodeCharset(buffer, mediaType, params)
					if err != nil {
						log.Println("Decode charset error:", err)
						err = nil // Don't fail parsing on decoding errors, use original
					}
					ptc.plainTextContents.Write(buffer)
				}

				err = ptc.target.Accept(bytes.NewReader(partData), header, hasPlainSibling, isFirst, isLast)
				return
			}
		}
	}
	err = ptc.target.Accept(partReader, header, hasPlainSibling, isFirst, isLast)
	return
}

func (ptc PlainTextCollector) GetPlainText() string {
	return ptc.plainTextContents.String()
}

// ======================== Body Collector  ==============
// Collect contents of all non-attachment parts and return
// it as a string
// TODO to file collector_body.go

type BodyCollector struct {
	target            VisitAcceptor
	htmlBodyBuffer    *bytes.Buffer
	plainBodyBuffer   *bytes.Buffer
	htmlHeaderBuffer  *bytes.Buffer
	plainHeaderBuffer *bytes.Buffer
	hasHtml           bool
}

func NewBodyCollector(targetAccepter VisitAcceptor) *BodyCollector {
	return &BodyCollector{
		target:            targetAccepter,
		htmlBodyBuffer:    bytes.NewBuffer([]byte("")),
		plainBodyBuffer:   bytes.NewBuffer([]byte("")),
		htmlHeaderBuffer:  bytes.NewBuffer([]byte("")),
		plainHeaderBuffer: bytes.NewBuffer([]byte("")),
	}
}

func (bc *BodyCollector) Accept(partReader io.Reader, header textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error) {
	// TODO: collect html and plaintext - if there's html with plain sibling don't include plain/text
	if isFirst {
		if IsLeaf(header) {
			mediaType, params, _ := getContentType(header)
			disp, _, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
			if disp != "attachment" {
				partData, _ := ioutil.ReadAll(partReader)
				decodedPart := decodePart(bytes.NewReader(partData), header)
				if buffer, err := ioutil.ReadAll(decodedPart); err == nil {
					buffer, err = DecodeCharset(buffer, mediaType, params)
					if err != nil {
						log.Println("Decode charset error:", err)
						err = nil // Don't fail parsing on decoding errors, use original
					}
					if mediaType == "text/html" {
						bc.hasHtml = true
						http.Header(header).Write(bc.htmlHeaderBuffer)
						bc.htmlBodyBuffer.Write(buffer)
					} else if mediaType == "text/plain" {
						http.Header(header).Write(bc.plainHeaderBuffer)
						bc.plainBodyBuffer.Write(buffer)
					}
				}

				err = bc.target.Accept(bytes.NewReader(partData), header, hasPlainSibling, isFirst, isLast)
				return
			}
		}
	}
	err = bc.target.Accept(partReader, header, hasPlainSibling, isFirst, isLast)
	return
}

func (bc *BodyCollector) GetBody() (string, string) {
	if bc.hasHtml {
		return bc.htmlBodyBuffer.String(), "text/html"
	} else {
		return bc.plainBodyBuffer.String(), "text/plain"
	}
}

func (bc *BodyCollector) GetHeaders() string {
	if bc.hasHtml {
		return bc.htmlHeaderBuffer.String()
	} else {
		return bc.plainHeaderBuffer.String()
	}
}

// ======================== Attachments Collector  ==============
// Collect contents of all attachment parts and return
// them as a string
// TODO to file collector_attachment.go

type AttachmentsCollector struct {
	target     VisitAcceptor
	attBuffers []string
	attHeaders []string
}

func NewAttachmentsCollector(targetAccepter VisitAcceptor) *AttachmentsCollector {
	return &AttachmentsCollector{
		target:     targetAccepter,
		attBuffers: []string{},
		attHeaders: []string{},
	}
}

func (ac *AttachmentsCollector) Accept(partReader io.Reader, header textproto.MIMEHeader, hasPlainSibling bool, isFirst, isLast bool) (err error) {
	if isFirst {
		if IsLeaf(header) {
			mediaType, params, _ := getContentType(header)
			disp, _, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
			if (mediaType != "text/html" && mediaType != "text/plain") || disp == "attachment" {
				partData, _ := ioutil.ReadAll(partReader)
				decodedPart := decodePart(bytes.NewReader(partData), header)

				if buffer, err := ioutil.ReadAll(decodedPart); err == nil {
					buffer, err = DecodeCharset(buffer, mediaType, params)
					if err != nil {
						log.Println("Decode charset error:", err)
						err = nil // Don't fail parsing on decoding errors, use original
					}
					headerBuf := new(bytes.Buffer)
					http.Header(header).Write(headerBuf)
					ac.attHeaders = append(ac.attHeaders, headerBuf.String())
					ac.attBuffers = append(ac.attBuffers, string(buffer))
				}

				err = ac.target.Accept(bytes.NewReader(partData), header, hasPlainSibling, isFirst, isLast)
				return
			}
		}
	}
	err = ac.target.Accept(partReader, header, hasPlainSibling, isFirst, isLast)
	return
}

func (ac AttachmentsCollector) GetAttachments() []string {
	return ac.attBuffers
}

func (ac AttachmentsCollector) GetAttHeaders() []string {
	return ac.attHeaders
}
