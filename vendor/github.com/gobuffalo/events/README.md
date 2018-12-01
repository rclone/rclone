<p align="center"><img src="https://github.com/gobuffalo/buffalo/blob/master/logo.svg" width="360"></p>

<p align="center">
<a href="https://godoc.org/github.com/gobuffalo/events"><img src="https://godoc.org/github.com/gobuffalo/events?status.svg" alt="GoDoc" /></a>
<a href="https://goreportcard.com/report/github.com/gobuffalo/events"><img src="https://goreportcard.com/badge/github.com/gobuffalo/events" alt="Go Report Card" /></a>
</p>

# github.com/gobuffalo/events

**Note:** This package was first introduced to Buffalo in this [PR](https://github.com/gobuffalo/buffalo/pull/1305). Assuming the PR is merged Buffalo will not start emitting events until `v0.13.0-beta.2` or greater.

A list of known emitted events can be found at [https://godoc.org/github.com/gobuffalo/events#pkg-constants](https://godoc.org/github.com/gobuffalo/events#pkg-constants)

## Installation

```bash
$ go get -u -v github.com/gobuffalo/events
```

## Listening For Events

To listen for events you need to register an [`events#Listener`](https://godoc.org/github.com/gobuffalo/events#Listener) function first.

```go
func init() {
  // if you want to give your listener a nice name to identify itself
  events.NamedListen("my-listener", func(e events.Event) {
    fmt.Println("### e ->", e)
  })

  // if you don't care about identifying your listener
  events.Listen(func(e events.Event) {
    fmt.Println("### e ->", e)
  })
}
```

## Emitting Events

```go
events.Emit(events.Event{
  Kind:    "my-event",
  Message: // optional message,
  Payload: // optional payload,
  Error:   // optional error,
})
```

There is only one required field when emitting an event, `Kind`.

The `Kind` field is key to how people will interpret your messages, and should be constructed as such: `<namespace>:<event-kind>:<err-optional>`.

In the examples below from [Buffalo](https://gobuffalo.io) you can see it is using the `buffalo:` name space for its events.

```go
// EvtAppStart is emitted when buffalo.App#Serve is called
EvtAppStart = "buffalo:app:start"
// EvtAppStartErr is emitted when an error occurs calling buffalo.App#Serve
EvtAppStartErr = "buffalo:app:start:err"
// EvtAppStop is emitted when buffalo.App#Stop is called
EvtAppStop = "buffalo:app:stop"
// EvtAppStopErr is emitted when an error occurs calling buffalo.App#Stop
EvtAppStopErr = "buffalo:app:stop:err"
```

## Implementing a Manager

By default `events` implements a basic manager for you. Should you want to replace that with your own implementation, perhaps that's backed by a proper message queue, you can implement the [`events#Manager`](https://godoc.org/github.com/gobuffalo/events#Manager) interface.

```go
var _ events.Manager = MyManager{}
events.SetManager(MyManager{})
```

## Listening via Buffalo Plugins

Once Buffalo is actively emitting events, plugins, will be able to listen those events via their CLIs.

To do so you can set the `BuffaloCommand` to `events` when telling Buffalo which plugin in commands are available. Buffalo will create a new listener that says the JSON version of the event to that command in question.

```go
var availableCmd = &cobra.Command{
	Use:   "available",
	Short: "a list of available buffalo plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		plugs := plugins.Commands{
			{Name: "echo", UseCommand: "echo", BuffaloCommand: "events", Description: echoCmd.Short, Aliases: echoCmd.Aliases},
		}
		return json.NewEncoder(os.Stdout).Encode(plugs)
	},
}


events.Emit(events.Event{
  Kind:    "my-event",
})

// buffalo-foo echo "{\"kind\": \"my-event\"}"
```
