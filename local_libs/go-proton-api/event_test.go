package proton_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/stretchr/testify/require"
)

func TestEventStreamer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.New()
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	createTestMessages(t, c, "pass", 10)

	latestEventID, err := c.GetLatestEventID(ctx)
	require.NoError(t, err)

	eventCh := make(chan proton.Event)

	go func() {
		for event := range c.NewEventStream(ctx, time.Second, 0, latestEventID) {
			eventCh <- event
		}
	}()

	// Perform some action to generate an event.
	metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
	require.NoError(t, err)
	require.NoError(t, c.LabelMessages(ctx, []string{metadata[0].ID}, proton.TrashLabel))

	// Wait for the first event.
	<-eventCh

	// Close the client; this should stop the client's event streamer.
	c.Close()

	// Create a new client and perform some actions with it to generate more events.
	cc, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)
	defer cc.Close()

	require.NoError(t, cc.LabelMessages(ctx, []string{metadata[1].ID}, proton.TrashLabel))

	// We should not receive any more events from the original client.
	select {
	case <-eventCh:
		require.Fail(t, "received unexpected event")

	default:
		// ...
	}
}

func TestMaxEventMerge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.New()
	defer s.Close()

	s.SetMaxUpdatesPerEvent(1)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	latestID, err := c.GetLatestEventID(ctx)
	require.NoError(t, err)

	label, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
		Name:  uuid.NewString(),
		Color: "#f66",
		Type:  proton.LabelTypeFolder,
	})
	require.NoError(t, err)

	for i := 0; i < 75; i++ {
		_, err := c.UpdateLabel(ctx, label.ID, proton.UpdateLabelReq{Name: uuid.NewString()})
		require.NoError(t, err)
	}

	events, more, err := c.GetEvent(ctx, latestID)
	require.NoError(t, err)
	require.True(t, more)
	require.Equal(t, 50, len(events))

	events2, more, err := c.GetEvent(ctx, events[len(events)-1].EventID)
	require.NotEqual(t, events, events2)
	require.NoError(t, err)
	require.False(t, more)
	require.Equal(t, 26, len(events2))
}
