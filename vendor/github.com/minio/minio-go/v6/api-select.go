/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * (C) 2018-2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v6/pkg/encrypt"
	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// CSVFileHeaderInfo - is the parameter for whether to utilize headers.
type CSVFileHeaderInfo string

// Constants for file header info.
const (
	CSVFileHeaderInfoNone   CSVFileHeaderInfo = "NONE"
	CSVFileHeaderInfoIgnore                   = "IGNORE"
	CSVFileHeaderInfoUse                      = "USE"
)

// SelectCompressionType - is the parameter for what type of compression is
// present
type SelectCompressionType string

// Constants for compression types under select API.
const (
	SelectCompressionNONE SelectCompressionType = "NONE"
	SelectCompressionGZIP                       = "GZIP"
	SelectCompressionBZIP                       = "BZIP2"
)

// CSVQuoteFields - is the parameter for how CSV fields are quoted.
type CSVQuoteFields string

// Constants for csv quote styles.
const (
	CSVQuoteFieldsAlways   CSVQuoteFields = "Always"
	CSVQuoteFieldsAsNeeded                = "AsNeeded"
)

// QueryExpressionType - is of what syntax the expression is, this should only
// be SQL
type QueryExpressionType string

// Constants for expression type.
const (
	QueryExpressionTypeSQL QueryExpressionType = "SQL"
)

// JSONType determines json input serialization type.
type JSONType string

// Constants for JSONTypes.
const (
	JSONDocumentType JSONType = "DOCUMENT"
	JSONLinesType             = "LINES"
)

// ParquetInputOptions parquet input specific options
type ParquetInputOptions struct{}

// CSVInputOptions csv input specific options
type CSVInputOptions struct {
	FileHeaderInfo    CSVFileHeaderInfo
	fileHeaderInfoSet bool

	RecordDelimiter    string
	recordDelimiterSet bool

	FieldDelimiter    string
	fieldDelimiterSet bool

	QuoteCharacter    string
	quoteCharacterSet bool

	QuoteEscapeCharacter    string
	quoteEscapeCharacterSet bool

	Comments    string
	commentsSet bool
}

// SetFileHeaderInfo sets the file header info in the CSV input options
func (c *CSVInputOptions) SetFileHeaderInfo(val CSVFileHeaderInfo) {
	c.FileHeaderInfo = val
	c.fileHeaderInfoSet = true
}

// SetRecordDelimiter sets the record delimiter in the CSV input options
func (c *CSVInputOptions) SetRecordDelimiter(val string) {
	c.RecordDelimiter = val
	c.recordDelimiterSet = true
}

// SetFieldDelimiter sets the field delimiter in the CSV input options
func (c *CSVInputOptions) SetFieldDelimiter(val string) {
	c.FieldDelimiter = val
	c.fieldDelimiterSet = true
}

// SetQuoteCharacter sets the quote character in the CSV input options
func (c *CSVInputOptions) SetQuoteCharacter(val string) {
	c.QuoteCharacter = val
	c.quoteCharacterSet = true
}

// SetQuoteEscapeCharacter sets the quote escape character in the CSV input options
func (c *CSVInputOptions) SetQuoteEscapeCharacter(val string) {
	c.QuoteEscapeCharacter = val
	c.quoteEscapeCharacterSet = true
}

// SetComments sets the comments character in the CSV input options
func (c *CSVInputOptions) SetComments(val string) {
	c.Comments = val
	c.commentsSet = true
}

// MarshalXML - produces the xml representation of the CSV input options struct
func (c CSVInputOptions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if c.FileHeaderInfo != "" || c.fileHeaderInfoSet {
		if err := e.EncodeElement(c.FileHeaderInfo, xml.StartElement{Name: xml.Name{Local: "FileHeaderInfo"}}); err != nil {
			return err
		}
	}

	if c.RecordDelimiter != "" || c.recordDelimiterSet {
		if err := e.EncodeElement(c.RecordDelimiter, xml.StartElement{Name: xml.Name{Local: "RecordDelimiter"}}); err != nil {
			return err
		}
	}

	if c.FieldDelimiter != "" || c.fieldDelimiterSet {
		if err := e.EncodeElement(c.FieldDelimiter, xml.StartElement{Name: xml.Name{Local: "FieldDelimiter"}}); err != nil {
			return err
		}
	}

	if c.QuoteCharacter != "" || c.quoteCharacterSet {
		if err := e.EncodeElement(c.QuoteCharacter, xml.StartElement{Name: xml.Name{Local: "QuoteCharacter"}}); err != nil {
			return err
		}
	}

	if c.QuoteEscapeCharacter != "" || c.quoteEscapeCharacterSet {
		if err := e.EncodeElement(c.QuoteEscapeCharacter, xml.StartElement{Name: xml.Name{Local: "QuoteEscapeCharacter"}}); err != nil {
			return err
		}
	}

	if c.Comments != "" || c.commentsSet {
		if err := e.EncodeElement(c.Comments, xml.StartElement{Name: xml.Name{Local: "Comments"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// CSVOutputOptions csv output specific options
type CSVOutputOptions struct {
	QuoteFields    CSVQuoteFields
	quoteFieldsSet bool

	RecordDelimiter    string
	recordDelimiterSet bool

	FieldDelimiter    string
	fieldDelimiterSet bool

	QuoteCharacter    string
	quoteCharacterSet bool

	QuoteEscapeCharacter    string
	quoteEscapeCharacterSet bool
}

// SetQuoteFields sets the quote field parameter in the CSV output options
func (c *CSVOutputOptions) SetQuoteFields(val CSVQuoteFields) {
	c.QuoteFields = val
	c.quoteFieldsSet = true
}

// SetRecordDelimiter sets the record delimiter character in the CSV output options
func (c *CSVOutputOptions) SetRecordDelimiter(val string) {
	c.RecordDelimiter = val
	c.recordDelimiterSet = true
}

// SetFieldDelimiter sets the field delimiter character in the CSV output options
func (c *CSVOutputOptions) SetFieldDelimiter(val string) {
	c.FieldDelimiter = val
	c.fieldDelimiterSet = true
}

// SetQuoteCharacter sets the quote character in the CSV output options
func (c *CSVOutputOptions) SetQuoteCharacter(val string) {
	c.QuoteCharacter = val
	c.quoteCharacterSet = true
}

// SetQuoteEscapeCharacter sets the quote escape character in the CSV output options
func (c *CSVOutputOptions) SetQuoteEscapeCharacter(val string) {
	c.QuoteEscapeCharacter = val
	c.quoteEscapeCharacterSet = true
}

// MarshalXML - produces the xml representation of the CSVOutputOptions struct
func (c CSVOutputOptions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if c.QuoteFields != "" || c.quoteFieldsSet {
		if err := e.EncodeElement(c.QuoteFields, xml.StartElement{Name: xml.Name{Local: "QuoteFields"}}); err != nil {
			return err
		}
	}

	if c.RecordDelimiter != "" || c.recordDelimiterSet {
		if err := e.EncodeElement(c.RecordDelimiter, xml.StartElement{Name: xml.Name{Local: "RecordDelimiter"}}); err != nil {
			return err
		}
	}

	if c.FieldDelimiter != "" || c.fieldDelimiterSet {
		if err := e.EncodeElement(c.FieldDelimiter, xml.StartElement{Name: xml.Name{Local: "FieldDelimiter"}}); err != nil {
			return err
		}
	}

	if c.QuoteCharacter != "" || c.quoteCharacterSet {
		if err := e.EncodeElement(c.QuoteCharacter, xml.StartElement{Name: xml.Name{Local: "QuoteCharacter"}}); err != nil {
			return err
		}
	}

	if c.QuoteEscapeCharacter != "" || c.quoteEscapeCharacterSet {
		if err := e.EncodeElement(c.QuoteEscapeCharacter, xml.StartElement{Name: xml.Name{Local: "QuoteEscapeCharacter"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// JSONInputOptions json input specific options
type JSONInputOptions struct {
	Type    JSONType
	typeSet bool
}

// SetType sets the JSON type in the JSON input options
func (j *JSONInputOptions) SetType(typ JSONType) {
	j.Type = typ
	j.typeSet = true
}

// MarshalXML - produces the xml representation of the JSONInputOptions struct
func (j JSONInputOptions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if j.Type != "" || j.typeSet {
		if err := e.EncodeElement(j.Type, xml.StartElement{Name: xml.Name{Local: "Type"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// JSONOutputOptions - json output specific options
type JSONOutputOptions struct {
	RecordDelimiter    string
	recordDelimiterSet bool
}

// SetRecordDelimiter sets the record delimiter in the JSON output options
func (j *JSONOutputOptions) SetRecordDelimiter(val string) {
	j.RecordDelimiter = val
	j.recordDelimiterSet = true
}

// MarshalXML - produces the xml representation of the JSONOutputOptions struct
func (j JSONOutputOptions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if j.RecordDelimiter != "" || j.recordDelimiterSet {
		if err := e.EncodeElement(j.RecordDelimiter, xml.StartElement{Name: xml.Name{Local: "RecordDelimiter"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// SelectObjectInputSerialization - input serialization parameters
type SelectObjectInputSerialization struct {
	CompressionType SelectCompressionType
	Parquet         *ParquetInputOptions `xml:"Parquet,omitempty"`
	CSV             *CSVInputOptions     `xml:"CSV,omitempty"`
	JSON            *JSONInputOptions    `xml:"JSON,omitempty"`
}

// SelectObjectOutputSerialization - output serialization parameters.
type SelectObjectOutputSerialization struct {
	CSV  *CSVOutputOptions  `xml:"CSV,omitempty"`
	JSON *JSONOutputOptions `xml:"JSON,omitempty"`
}

// SelectObjectOptions - represents the input select body
type SelectObjectOptions struct {
	XMLName              xml.Name           `xml:"SelectObjectContentRequest" json:"-"`
	ServerSideEncryption encrypt.ServerSide `xml:"-"`
	Expression           string
	ExpressionType       QueryExpressionType
	InputSerialization   SelectObjectInputSerialization
	OutputSerialization  SelectObjectOutputSerialization
	RequestProgress      struct {
		Enabled bool
	}
}

// Header returns the http.Header representation of the SelectObject options.
func (o SelectObjectOptions) Header() http.Header {
	headers := make(http.Header)
	if o.ServerSideEncryption != nil && o.ServerSideEncryption.Type() == encrypt.SSEC {
		o.ServerSideEncryption.Marshal(headers)
	}
	return headers
}

// SelectObjectType - is the parameter which defines what type of object the
// operation is being performed on.
type SelectObjectType string

// Constants for input data types.
const (
	SelectObjectTypeCSV     SelectObjectType = "CSV"
	SelectObjectTypeJSON                     = "JSON"
	SelectObjectTypeParquet                  = "Parquet"
)

// preludeInfo is used for keeping track of necessary information from the
// prelude.
type preludeInfo struct {
	totalLen  uint32
	headerLen uint32
}

// SelectResults is used for the streaming responses from the server.
type SelectResults struct {
	pipeReader *io.PipeReader
	resp       *http.Response
	stats      *StatsMessage
	progress   *ProgressMessage
}

// ProgressMessage is a struct for progress xml message.
type ProgressMessage struct {
	XMLName xml.Name `xml:"Progress" json:"-"`
	StatsMessage
}

// StatsMessage is a struct for stat xml message.
type StatsMessage struct {
	XMLName        xml.Name `xml:"Stats" json:"-"`
	BytesScanned   int64
	BytesProcessed int64
	BytesReturned  int64
}

// messageType represents the type of message.
type messageType string

const (
	errorMsg  messageType = "error"
	commonMsg             = "event"
)

// eventType represents the type of event.
type eventType string

// list of event-types returned by Select API.
const (
	endEvent      eventType = "End"
	recordsEvent            = "Records"
	progressEvent           = "Progress"
	statsEvent              = "Stats"
)

// contentType represents content type of event.
type contentType string

const (
	xmlContent contentType = "text/xml"
)

// SelectObjectContent is a implementation of http://docs.aws.amazon.com/AmazonS3/latest/API/RESTObjectSELECTContent.html AWS S3 API.
func (c Client) SelectObjectContent(ctx context.Context, bucketName, objectName string, opts SelectObjectOptions) (*SelectResults, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return nil, err
	}
	if err := s3utils.CheckValidObjectName(objectName); err != nil {
		return nil, err
	}

	selectReqBytes, err := xml.Marshal(opts)
	if err != nil {
		return nil, err
	}

	urlValues := make(url.Values)
	urlValues.Set("select", "")
	urlValues.Set("select-type", "2")

	// Execute POST on bucket/object.
	resp, err := c.executeMethod(ctx, "POST", requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		queryValues:      urlValues,
		customHeader:     opts.Header(),
		contentMD5Base64: sumMD5Base64(selectReqBytes),
		contentSHA256Hex: sum256Hex(selectReqBytes),
		contentBody:      bytes.NewReader(selectReqBytes),
		contentLength:    int64(len(selectReqBytes)),
	})
	if err != nil {
		return nil, err
	}

	return NewSelectResults(resp, bucketName)
}

// NewSelectResults creates a Select Result parser that parses the response
// and returns a Reader that will return parsed and assembled select output.
func NewSelectResults(resp *http.Response, bucketName string) (*SelectResults, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp, bucketName, "")
	}

	pipeReader, pipeWriter := io.Pipe()
	streamer := &SelectResults{
		resp:       resp,
		stats:      &StatsMessage{},
		progress:   &ProgressMessage{},
		pipeReader: pipeReader,
	}
	streamer.start(pipeWriter)
	return streamer, nil
}

// Close - closes the underlying response body and the stream reader.
func (s *SelectResults) Close() error {
	defer closeResponse(s.resp)
	return s.pipeReader.Close()
}

// Read - is a reader compatible implementation for SelectObjectContent records.
func (s *SelectResults) Read(b []byte) (n int, err error) {
	return s.pipeReader.Read(b)
}

// Stats - information about a request's stats when processing is complete.
func (s *SelectResults) Stats() *StatsMessage {
	return s.stats
}

// Progress - information about the progress of a request.
func (s *SelectResults) Progress() *ProgressMessage {
	return s.progress
}

// start is the main function that decodes the large byte array into
// several events that are sent through the eventstream.
func (s *SelectResults) start(pipeWriter *io.PipeWriter) {
	go func() {
		for {
			var prelude preludeInfo
			var headers = make(http.Header)
			var err error

			// Create CRC code
			crc := crc32.New(crc32.IEEETable)
			crcReader := io.TeeReader(s.resp.Body, crc)

			// Extract the prelude(12 bytes) into a struct to extract relevant information.
			prelude, err = processPrelude(crcReader, crc)
			if err != nil {
				pipeWriter.CloseWithError(err)
				closeResponse(s.resp)
				return
			}

			// Extract the headers(variable bytes) into a struct to extract relevant information
			if prelude.headerLen > 0 {
				if err = extractHeader(io.LimitReader(crcReader, int64(prelude.headerLen)), headers); err != nil {
					pipeWriter.CloseWithError(err)
					closeResponse(s.resp)
					return
				}
			}

			// Get the actual payload length so that the appropriate amount of
			// bytes can be read or parsed.
			payloadLen := prelude.PayloadLen()

			m := messageType(headers.Get("message-type"))

			switch m {
			case errorMsg:
				pipeWriter.CloseWithError(errors.New(headers.Get("error-code") + ":\"" + headers.Get("error-message") + "\""))
				closeResponse(s.resp)
				return
			case commonMsg:
				// Get content-type of the payload.
				c := contentType(headers.Get("content-type"))

				// Get event type of the payload.
				e := eventType(headers.Get("event-type"))

				// Handle all supported events.
				switch e {
				case endEvent:
					pipeWriter.Close()
					closeResponse(s.resp)
					return
				case recordsEvent:
					if _, err = io.Copy(pipeWriter, io.LimitReader(crcReader, payloadLen)); err != nil {
						pipeWriter.CloseWithError(err)
						closeResponse(s.resp)
						return
					}
				case progressEvent:
					switch c {
					case xmlContent:
						if err = xmlDecoder(io.LimitReader(crcReader, payloadLen), s.progress); err != nil {
							pipeWriter.CloseWithError(err)
							closeResponse(s.resp)
							return
						}
					default:
						pipeWriter.CloseWithError(fmt.Errorf("Unexpected content-type %s sent for event-type %s", c, progressEvent))
						closeResponse(s.resp)
						return
					}
				case statsEvent:
					switch c {
					case xmlContent:
						if err = xmlDecoder(io.LimitReader(crcReader, payloadLen), s.stats); err != nil {
							pipeWriter.CloseWithError(err)
							closeResponse(s.resp)
							return
						}
					default:
						pipeWriter.CloseWithError(fmt.Errorf("Unexpected content-type %s sent for event-type %s", c, statsEvent))
						closeResponse(s.resp)
						return
					}
				}
			}

			// Ensures that the full message's CRC is correct and
			// that the message is not corrupted
			if err := checkCRC(s.resp.Body, crc.Sum32()); err != nil {
				pipeWriter.CloseWithError(err)
				closeResponse(s.resp)
				return
			}

		}
	}()
}

// PayloadLen is a function that calculates the length of the payload.
func (p preludeInfo) PayloadLen() int64 {
	return int64(p.totalLen - p.headerLen - 16)
}

// processPrelude is the function that reads the 12 bytes of the prelude and
// ensures the CRC is correct while also extracting relevant information into
// the struct,
func processPrelude(prelude io.Reader, crc hash.Hash32) (preludeInfo, error) {
	var err error
	var pInfo = preludeInfo{}

	// reads total length of the message (first 4 bytes)
	pInfo.totalLen, err = extractUint32(prelude)
	if err != nil {
		return pInfo, err
	}

	// reads total header length of the message (2nd 4 bytes)
	pInfo.headerLen, err = extractUint32(prelude)
	if err != nil {
		return pInfo, err
	}

	// checks that the CRC is correct (3rd 4 bytes)
	preCRC := crc.Sum32()
	if err := checkCRC(prelude, preCRC); err != nil {
		return pInfo, err
	}

	return pInfo, nil
}

// extracts the relevant information from the Headers.
func extractHeader(body io.Reader, myHeaders http.Header) error {
	for {
		// extracts the first part of the header,
		headerTypeName, err := extractHeaderType(body)
		if err != nil {
			// Since end of file, we have read all of our headers
			if err == io.EOF {
				break
			}
			return err
		}

		// reads the 7 present in the header and ignores it.
		extractUint8(body)

		headerValueName, err := extractHeaderValue(body)
		if err != nil {
			return err
		}

		myHeaders.Set(headerTypeName, headerValueName)

	}
	return nil
}

// extractHeaderType extracts the first half of the header message, the header type.
func extractHeaderType(body io.Reader) (string, error) {
	// extracts 2 bit integer
	headerNameLen, err := extractUint8(body)
	if err != nil {
		return "", err
	}
	// extracts the string with the appropriate number of bytes
	headerName, err := extractString(body, int(headerNameLen))
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(headerName, ":"), nil
}

// extractsHeaderValue extracts the second half of the header message, the
// header value
func extractHeaderValue(body io.Reader) (string, error) {
	bodyLen, err := extractUint16(body)
	if err != nil {
		return "", err
	}
	bodyName, err := extractString(body, int(bodyLen))
	if err != nil {
		return "", err
	}
	return bodyName, nil
}

// extracts a string from byte array of a particular number of bytes.
func extractString(source io.Reader, lenBytes int) (string, error) {
	myVal := make([]byte, lenBytes)
	_, err := source.Read(myVal)
	if err != nil {
		return "", err
	}
	return string(myVal), nil
}

// extractUint32 extracts a 4 byte integer from the byte array.
func extractUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}

// extractUint16 extracts a 2 byte integer from the byte array.
func extractUint16(r io.Reader) (uint16, error) {
	buf := make([]byte, 2)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf), nil
}

// extractUint8 extracts a 1 byte integer from the byte array.
func extractUint8(r io.Reader) (uint8, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

// checkCRC ensures that the CRC matches with the one from the reader.
func checkCRC(r io.Reader, expect uint32) error {
	msgCRC, err := extractUint32(r)
	if err != nil {
		return err
	}

	if msgCRC != expect {
		return fmt.Errorf("Checksum Mismatch, MessageCRC of 0x%X does not equal expected CRC of 0x%X", msgCRC, expect)

	}
	return nil
}
