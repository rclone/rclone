package proton_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/ProtonMail/gluon/async"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/stream"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/require"
)

func createTestMessages(t *testing.T, c *proton.Client, pass string, count int) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	user, err := c.GetUser(ctx)
	require.NoError(t, err)

	addr, err := c.GetAddresses(ctx)
	require.NoError(t, err)

	salt, err := c.GetSalts(ctx)
	require.NoError(t, err)

	keyPass, err := salt.SaltForKey([]byte(pass), user.Keys.Primary().ID)
	require.NoError(t, err)

	_, addrKRs, err := proton.Unlock(user, addr, keyPass, async.NoopPanicHandler{})
	require.NoError(t, err)

	req := iterator.Collect(iterator.Map(iterator.Counter(count), func(i int) proton.ImportReq {
		return proton.ImportReq{
			Metadata: proton.ImportMetadata{
				AddressID: addr[0].ID,
				Flags:     proton.MessageFlagReceived,
				Unread:    true,
			},
			Message: []byte(fmt.Sprintf("From: sender@example.com\r\nReceiver: recipient@example.com\r\nSubject: %v\r\n\r\nHello World!", uuid.New())),
		}
	}))

	str, err := c.ImportMessages(ctx, addrKRs[addr[0].ID], runtime.NumCPU(), runtime.NumCPU(), req...)
	require.NoError(t, err)

	res, err := stream.Collect(ctx, str)
	require.NoError(t, err)

	for _, res := range res {
		require.Equal(t, proton.SuccessCode, res.Code)
	}
}
