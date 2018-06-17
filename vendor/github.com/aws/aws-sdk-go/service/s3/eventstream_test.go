package s3

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamtest"
	"github.com/aws/aws-sdk-go/private/protocol/restxml"
)

func mockSelectObjectContentEventStreamsOutputEvents() ([]SelectObjectContentEventStreamEvent, []eventstream.Message) {

	expectEvents := []SelectObjectContentEventStreamEvent{
		&ContinuationEvent{},
		&EndEvent{},
		&ProgressEvent{
			Details: &Progress{
				BytesProcessed: aws.Int64(123),
				BytesReturned:  aws.Int64(456),
				BytesScanned:   aws.Int64(789),
			},
		},
		&RecordsEvent{
			Payload: []byte("abc123"),
		},
		&StatsEvent{
			Details: &Stats{
				BytesProcessed: aws.Int64(123),
				BytesReturned:  aws.Int64(456),
				BytesScanned:   aws.Int64(789),
			},
		},
	}

	var marshalers request.HandlerList
	marshalers.PushBackNamed(restxml.BuildHandler)
	payloadMarshaler := protocol.HandlerPayloadMarshal{
		Marshalers: marshalers,
	}

	eventMsgs := []eventstream.Message{
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Cont"),
				},
			},
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("End"),
				},
			},
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Progress"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[2]),
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Records"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[3]),
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Stats"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[4]),
		},
	}

	return expectEvents, eventMsgs
}

func TestSelectObjectContentEventStream(t *testing.T) {
	expectEvents, eventMsgs := mockSelectObjectContentEventStreamsOutputEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		false,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)

	params := &SelectObjectContentInput{}
	resp, err := svc.SelectObjectContent(params)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}
	defer resp.EventStream.Close()

	var i int
	for event := range resp.EventStream.Events() {
		if event == nil {
			t.Errorf("%d, expect event, got nil", i)
		}
		if e, a := expectEvents[i], event; !reflect.DeepEqual(e, a) {
			t.Errorf("%d, expect %T %v, got %T %v", i, e, e, a, a)
		}
		i++
	}

	if err := resp.EventStream.Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func TestSelectObjectContentEventStream_Close(t *testing.T) {
	_, eventMsgs := mockSelectObjectContentEventStreamsOutputEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		false,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)

	params := &SelectObjectContentInput{}
	resp, err := svc.SelectObjectContent(params)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}

	resp.EventStream.Close()

	if err := resp.EventStream.Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func ExampleSelectObjectContentEventStream() {
	sess := session.Must(session.NewSession())
	svc := New(sess)

	/*
	   Example myObjectKey CSV content:

	   name,number
	   gopher,0
	   ᵷodɥǝɹ,1
	*/

	// Make the Select Object Content API request using the object uploaded.
	resp, err := svc.SelectObjectContent(&SelectObjectContentInput{
		Bucket:         aws.String("myBucket"),
		Key:            aws.String("myObjectKey"),
		Expression:     aws.String("SELECT name FROM S3Object WHERE cast(number as int) < 1"),
		ExpressionType: aws.String(ExpressionTypeSql),
		InputSerialization: &InputSerialization{
			CSV: &CSVInput{
				FileHeaderInfo: aws.String(FileHeaderInfoUse),
			},
		},
		OutputSerialization: &OutputSerialization{
			CSV: &CSVOutput{},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed making API request, %v\n", err)
		return
	}
	defer resp.EventStream.Close()

	results, resultWriter := io.Pipe()
	go func() {
		defer resultWriter.Close()
		for event := range resp.EventStream.Events() {
			switch e := event.(type) {
			case *RecordsEvent:
				resultWriter.Write(e.Payload)
			case *StatsEvent:
				fmt.Printf("Processed %d bytes\n", e.Details.BytesProcessed)
			}
		}
	}()

	// Printout the results
	resReader := csv.NewReader(results)
	for {
		record, err := resReader.Read()
		if err == io.EOF {
			break
		}
		fmt.Println(record)
	}

	if err := resp.EventStream.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading from event stream failed, %v\n", err)
	}
}

func BenchmarkSelectObjectContentEventStream(b *testing.B) {
	_, eventMsgs := mockSelectObjectContentEventStreamsOutputEvents()
	var buf bytes.Buffer
	encoder := eventstream.NewEncoder(&buf)
	for _, msg := range eventMsgs {
		if err := encoder.Encode(msg); err != nil {
			b.Fatalf("failed to encode message, %v", err)
		}
	}
	stream := &loopReader{source: bytes.NewReader(buf.Bytes())}

	sess := unit.Session
	svc := New(sess, &aws.Config{
		Endpoint:               aws.String("https://example.com"),
		DisableParamValidation: aws.Bool(true),
	})
	svc.Handlers.Send.Swap(corehandlers.SendHandler.Name,
		request.NamedHandler{Name: "mockSend",
			Fn: func(r *request.Request) {
				r.HTTPResponse = &http.Response{
					Status:     "200 OK",
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(stream),
				}
			},
		},
	)

	params := &SelectObjectContentInput{}
	resp, err := svc.SelectObjectContent(params)
	if err != nil {
		b.Fatalf("failed to create request, %v", err)
	}
	defer resp.EventStream.Close()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = resp.EventStream.Err(); err != nil {
			b.Fatalf("expect no error, got %v", err)
		}
		event := <-resp.EventStream.Events()
		if event == nil {
			b.Fatalf("expect event, got nil, %v, %d", resp.EventStream.Err(), i)
		}
	}
}

type loopReader struct {
	source *bytes.Reader
}

func (c *loopReader) Read(p []byte) (int, error) {
	if c.source.Len() == 0 {
		c.source.Seek(0, 0)
	}

	return c.source.Read(p)
}
