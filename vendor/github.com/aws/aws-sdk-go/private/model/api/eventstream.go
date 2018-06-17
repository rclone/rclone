// +build codegen

package api

import (
	"bytes"
	"fmt"
	"io"
	"text/template"
)

// EventStreamAPI provides details about the event stream async API and
// associated EventStream shapes.
type EventStreamAPI struct {
	Name      string
	Operation *Operation
	Shape     *Shape
	Inbound   *EventStream
	Outbound  *EventStream
}

// EventStream represents a single eventstream group (input/output) and the
// modeled events that are known for the stream.
type EventStream struct {
	Name   string
	Shape  *Shape
	Events []*Event
}

// Event is a single EventStream event that can be sent or received in an
// EventStream.
type Event struct {
	Name  string
	Shape *Shape
	For   *EventStream
}

// ShapeDoc returns the docstring for the EventStream API.
func (esAPI *EventStreamAPI) ShapeDoc() string {
	tmpl := template.Must(template.New("eventStreamShapeDoc").Parse(`
{{- $.Name }} provides handling of EventStreams for
the {{ $.Operation.ExportedName }} API. 
{{- if $.Inbound }}

Use this type to receive {{ $.Inbound.Name }} events. The events
can be read from the Events channel member.

The events that can be received are:
{{ range $_, $event := $.Inbound.Events }}
    * {{ $event.Shape.ShapeName }}
{{- end }}

{{- end }}

{{- if $.Outbound }}

Use this type to send {{ $.Outbound.Name }} events. The events 
can be sent with the Send method.

The events that can be sent are:
{{ range $_, $event := $.Outbound.Events -}}
    * {{ $event.Shape.ShapeName }}
{{- end }}

{{- end }}`))

	var w bytes.Buffer
	if err := tmpl.Execute(&w, esAPI); err != nil {
		panic(fmt.Sprintf("failed to generate eventstream shape template for %v, %v", esAPI.Name, err))
	}

	return commentify(w.String())
}

func eventStreamAPIShapeRefDoc(refName string) string {
	return commentify(fmt.Sprintf("Use %s to use the API's stream.", refName))
}

func (a *API) setupEventStreams() {
	const eventStreamMemberName = "EventStream"

	for _, op := range a.Operations {
		outbound := setupEventStream(op.InputRef.Shape)
		inbound := setupEventStream(op.OutputRef.Shape)

		if outbound == nil && inbound == nil {
			continue
		}

		if outbound != nil {
			panic(fmt.Sprintf("Outbound stream support not implemented, %s, %s",
				outbound.Name, outbound.Shape.ShapeName))
		}

		switch a.Metadata.Protocol {
		case `rest-json`, `rest-xml`, `json`:
		default:
			panic(fmt.Sprintf("EventStream not supported for protocol %v",
				a.Metadata.Protocol))
		}

		eventStreamAPI := &EventStreamAPI{
			Name:      op.ExportedName + eventStreamMemberName,
			Operation: op,
			Outbound:  outbound,
			Inbound:   inbound,
		}

		streamShape := &Shape{
			API:            a,
			ShapeName:      eventStreamAPI.Name,
			Documentation:  eventStreamAPI.ShapeDoc(),
			Type:           "structure",
			EventStreamAPI: eventStreamAPI,
		}
		streamShapeRef := &ShapeRef{
			API:           a,
			ShapeName:     streamShape.ShapeName,
			Shape:         streamShape,
			Documentation: eventStreamAPIShapeRefDoc(eventStreamMemberName),
		}
		streamShape.refs = []*ShapeRef{streamShapeRef}
		eventStreamAPI.Shape = streamShape

		if _, ok := op.OutputRef.Shape.MemberRefs[eventStreamMemberName]; ok {
			panic(fmt.Sprintf("shape ref already exists, %s.%s",
				op.OutputRef.Shape.ShapeName, eventStreamMemberName))
		}
		op.OutputRef.Shape.MemberRefs[eventStreamMemberName] = streamShapeRef
		op.OutputRef.Shape.EventStreamsMemberName = eventStreamMemberName
		if _, ok := a.Shapes[streamShape.ShapeName]; ok {
			panic("shape already exists, " + streamShape.ShapeName)
		}
		a.Shapes[streamShape.ShapeName] = streamShape

		a.HasEventStream = true
	}
}

func setupEventStream(topShape *Shape) *EventStream {
	var eventStream *EventStream
	for refName, ref := range topShape.MemberRefs {
		if !ref.Shape.IsEventStream {
			continue
		}
		if eventStream != nil {
			panic(fmt.Sprintf("multiple shape ref eventstreams, %s, prev: %s",
				refName, eventStream.Name))
		}

		eventStream = &EventStream{
			Name:  ref.Shape.ShapeName,
			Shape: ref.Shape,
		}
		for _, eventRefName := range ref.Shape.MemberNames() {
			eventRef := ref.Shape.MemberRefs[eventRefName]
			if !eventRef.Shape.IsEvent {
				panic(fmt.Sprintf("unexpected non-event member reference %s.%s",
					ref.Shape.ShapeName, eventRefName))
			}

			updateEventPayloadRef(eventRef.Shape)

			eventRef.Shape.EventFor = append(eventRef.Shape.EventFor, eventStream)
			eventStream.Events = append(eventStream.Events, &Event{
				Name:  eventRefName,
				Shape: eventRef.Shape,
				For:   eventStream,
			})
		}

		// Remove the eventstream references as they will be added elsewhere.
		ref.Shape.removeRef(ref)
		delete(topShape.MemberRefs, refName)
		delete(topShape.API.Shapes, ref.Shape.ShapeName)
	}

	return eventStream
}

func updateEventPayloadRef(parent *Shape) {
	refName := parent.PayloadRefName()
	if len(refName) == 0 {
		return
	}

	payloadRef := parent.MemberRefs[refName]

	if payloadRef.Shape.Type == "blob" {
		return
	}

	if len(payloadRef.LocationName) != 0 {
		return
	}

	payloadRef.LocationName = refName
}

func renderEventStreamAPIShape(w io.Writer, s *Shape) error {
	// Imports needed by the EventStream APIs.
	s.API.imports["bytes"] = true
	s.API.imports["io"] = true
	s.API.imports["sync"] = true
	s.API.imports["sync/atomic"] = true
	s.API.imports["github.com/aws/aws-sdk-go/private/protocol/eventstream"] = true
	s.API.imports["github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamapi"] = true

	return eventStreamAPIShapeTmpl.Execute(w, s)
}

// EventStreamReaderInterfaceName returns the interface name for the
// EventStream's reader interface.
func EventStreamReaderInterfaceName(s *Shape) string {
	return s.ShapeName + "Reader"
}

// Template for an EventStream API Shape that will provide read/writing events
// across the EventStream. This is a special shape that's only public members
// are the Events channel and a Close and Err method.
//
// Executed in the context of a Shape.
var eventStreamAPIShapeTmpl = func() *template.Template {
	t := template.Must(
		template.New("eventStreamAPIShapeTmpl").
			Funcs(template.FuncMap{}).
			Parse(eventStreamAPITmplDef),
	)

	template.Must(
		t.AddParseTree(
			"eventStreamAPIReaderTmpl", eventStreamAPIReaderTmpl.Tree),
	)

	return t
}()

const eventStreamAPITmplDef = `
{{ $.Documentation }}
type {{ $.ShapeName }} struct {
	{{- if $.EventStreamAPI.Inbound }}
		// Reader is the EventStream reader for the {{ $.EventStreamAPI.Inbound.Name }}
		// events. This value is automatically set by the SDK when the API call is made
		// Use this member when unit testing your code with the SDK to mock out the
		// EventStream Reader.
		//
		// Must not be nil.
		Reader {{ $.ShapeName }}Reader

	{{ end -}}

	{{- if $.EventStreamAPI.Outbound }}
		// Writer is the EventStream reader for the {{ $.EventStreamAPI.Inbound.Name }}
		// events. This value is automatically set by the SDK when the API call is made
		// Use this member when unit testing your code with the SDK to mock out the
		// EventStream Writer.
		//
		// Must not be nil.
		Writer *{{ $.ShapeName }}Writer

	{{ end -}}

	// StreamCloser is the io.Closer for the EventStream connection. For HTTP
	// EventStream this is the response Body. The stream will be closed when
	// the Close method of the EventStream is called.
	StreamCloser io.Closer
}

// Close closes the EventStream. This will also cause the Events channel to be
// closed. You can use the closing of the Events channel to terminate your
// application's read from the API's EventStream.
{{- if $.EventStreamAPI.Inbound }}
//
// Will close the underlying EventStream reader. For EventStream over HTTP
// connection this will also close the HTTP connection.
{{ end -}}
//
// Close must be called when done using the EventStream API. Not calling Close
// may result in resource leaks.
func (es *{{ $.ShapeName }}) Close() (err error) { 
	{{- if $.EventStreamAPI.Inbound }}
		es.Reader.Close()
	{{ end -}}
	{{- if $.EventStreamAPI.Outbound }}
		es.Writer.Close()
	{{ end -}}

	return es.Err()
}

// Err returns any error that occurred while reading EventStream Events from
// the service API's response. Returns nil if there were no errors.
func (es *{{ $.ShapeName }}) Err() error { 
	{{- if $.EventStreamAPI.Outbound }}
		if err := es.Writer.Err(); err != nil {
			return err
		}
	{{ end -}}

	{{- if $.EventStreamAPI.Inbound }}
		if err := es.Reader.Err(); err != nil {
			return err
		}
	{{ end -}}

	es.StreamCloser.Close()

	return nil
}

{{ if $.EventStreamAPI.Inbound }}
	// Events returns a channel to read EventStream Events from the 
	// {{ $.EventStreamAPI.Operation.ExportedName }} API.
	//
	// These events are:
	// {{ range $_, $event := $.EventStreamAPI.Inbound.Events }}
	//     * {{ $event.Shape.ShapeName }}
	{{- end }}
	func (es *{{ $.ShapeName }}) Events() <-chan {{ $.EventStreamAPI.Inbound.Name }}Event {
		return es.Reader.Events()
	}

	{{ template "eventStreamAPIReaderTmpl" $ }}
{{ end }}

{{ if $.EventStreamAPI.Outbound }}
	// TODO writer helper method.
{{ end }}

`

var eventStreamAPIReaderTmpl = template.Must(template.New("eventStreamAPIReaderTmpl").
	Funcs(template.FuncMap{}).
	Parse(`
// {{ $.EventStreamAPI.Inbound.Name }}Event groups together all EventStream
// events read from the {{ $.EventStreamAPI.Operation.ExportedName }} API.
//
// These events are:
// {{ range $_, $event := $.EventStreamAPI.Inbound.Events }}
//     * {{ $event.Shape.ShapeName }}
{{- end }}
type {{ $.EventStreamAPI.Inbound.Name }}Event interface {
	event{{ $.EventStreamAPI.Name }}()
}

// {{ $.ShapeName }}Reader provides the interface for reading EventStream
// Events from the {{ $.EventStreamAPI.Operation.ExportedName }} API. The
// default implementation for this interface will be {{ $.ShapeName }}.
//
// The reader's Close method must allow multiple concurrent calls.
//
// These events are:
// {{ range $_, $event := $.EventStreamAPI.Inbound.Events }}
//     * {{ $event.Shape.ShapeName }}
{{- end }}
type {{ $.ShapeName }}Reader interface {
	// Returns a channel of events as they are read from the event stream.
	Events() <-chan {{ $.EventStreamAPI.Inbound.Name }}Event

	// Close will close the underlying event stream reader. For event stream over
	// HTTP this will also close the HTTP connection.
	Close() error

	// Returns any error that has occured while reading from the event stream.
	Err() error
}

type read{{ $.ShapeName }} struct {
	eventReader *eventstreamapi.EventReader
	stream chan {{ $.EventStreamAPI.Inbound.Name }}Event
	errVal atomic.Value

	done      chan struct{}
	closeOnce sync.Once
}

func newRead{{ $.ShapeName }}(
	reader io.ReadCloser,
	unmarshalers request.HandlerList,
	logger aws.Logger,
	logLevel aws.LogLevelType,
) *read{{ $.ShapeName }} {
	r := &read{{ $.ShapeName }}{
		stream: make(chan {{ $.EventStreamAPI.Inbound.Name }}Event),
		done: make(chan struct{}),
	}

	r.eventReader = eventstreamapi.NewEventReader(
		reader,
		protocol.HandlerPayloadUnmarshal{
			Unmarshalers: unmarshalers,
		},
		r.unmarshalerForEventType,
	)
	r.eventReader.UseLogger(logger, logLevel)

	return r
}

// Close will close the underlying event stream reader. For EventStream over
// HTTP this will also close the HTTP connection.
func (r *read{{ $.ShapeName }}) Close() error {
	r.closeOnce.Do(r.safeClose)

	return r.Err()
}

func (r *read{{ $.ShapeName }}) safeClose() {
	close(r.done)
	err := r.eventReader.Close()
	if err != nil {
		r.errVal.Store(err)
	}
}

func (r *read{{ $.ShapeName }}) Err() error {
	if v := r.errVal.Load(); v != nil {
		return v.(error)
	}

	return nil
}

func (r *read{{ $.ShapeName }}) Events() <-chan {{ $.EventStreamAPI.Inbound.Name }}Event {
	return r.stream
}

func (r *read{{ $.ShapeName }}) readEventStream() {
	defer close(r.stream)

	for {
		event, err := r.eventReader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				return
			}
			select {
			case <-r.done:
				// If closed already ignore the error
				return
			default:
			}
			r.errVal.Store(err)
			return
		}

		select {
		case r.stream <- event.({{ $.EventStreamAPI.Inbound.Name }}Event):
		case <-r.done:
			return
		}
	}
}

func (r *read{{ $.ShapeName }}) unmarshalerForEventType(
	eventType string,
) (eventstreamapi.Unmarshaler, error) {
	switch eventType {
		{{- range $_, $event := $.EventStreamAPI.Inbound.Events }}
			case {{ printf "%q" $event.Name }}:
				return &{{ $event.Shape.ShapeName }}{}, nil
		{{ end -}}
	default:
		return nil, fmt.Errorf(
			"unknown event type name, %s, for {{ $.ShapeName }}", eventType)
	}
}
`))

// Template for the EventStream API Output shape that contains the EventStream
// member.
//
// Executed in the context of a Shape.
var eventStreamAPILoopMethodTmpl = template.Must(
	template.New("eventStreamAPILoopMethodTmpl").Parse(`
func (s *{{ $.ShapeName }}) runEventStreamLoop(r *request.Request) {
	if r.Error != nil {
		return
	}

	{{- $esMemberRef := index $.MemberRefs $.EventStreamsMemberName }}
	{{- if $esMemberRef.Shape.EventStreamAPI.Inbound }}
		reader := newRead{{ $esMemberRef.ShapeName }}(
			r.HTTPResponse.Body,
			r.Handlers.UnmarshalStream,
			r.Config.Logger,
			r.Config.LogLevel.Value(),
		)
		go reader.readEventStream()

		eventStream := &{{ $esMemberRef.ShapeName }} {
			StreamCloser: r.HTTPResponse.Body,
			Reader: reader,
		}
	{{ end -}}

	s.{{ $.EventStreamsMemberName }} = eventStream
}
`))

// Template for an EventStream Event shape. This is a normal API shape that is
// decorated as an EventStream Event.
//
// Executed in the context of a Shape.
var eventStreamEventShapeTmpl = template.Must(template.New("eventStreamEventShapeTmpl").Parse(`
{{ range $_, $eventstream := $.EventFor }}
	// The {{ $.ShapeName }} is and event in the {{ $eventstream.Name }} group of events.
	func (s *{{ $.ShapeName }}) event{{ $eventstream.Name }}() {}
{{ end }}

// UnmarshalEvent unmarshals the EventStream Message into the {{ $.ShapeName }} value.
// This method is only used internally within the SDK's EventStream handling.
func (s *{{ $.ShapeName }}) UnmarshalEvent(
	payloadUnmarshaler protocol.PayloadUnmarshaler,
	msg eventstream.Message,
) error {
	{{- range $fieldIdx, $fieldName := $.MemberNames }}
		{{- $fieldRef := index $.MemberRefs $fieldName -}}
		{{ if $fieldRef.IsEventHeader }}
			// TODO handle event header, {{ $fieldName }}
		{{- else if (and ($fieldRef.IsEventPayload) (eq $fieldRef.Shape.Type "blob")) }}
			s.{{ $fieldName }} = make([]byte, len(msg.Payload))
			copy(s.{{ $fieldName }}, msg.Payload)
		{{- else }}
			if err := payloadUnmarshaler.UnmarshalPayload(
				bytes.NewReader(msg.Payload), s,
			); err != nil {
				return fmt.Errorf("failed to unmarshal payload, %v", err)
			}
		{{- end }}
	{{- end }}
	return nil
}
`))

var eventStreamTestTmpl = template.Must(template.New("eventStreamTestTmpl").Parse(`
`))
