package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bradenaw/juniper/parallel"

	"github.com/Masterminds/semver/v3"
	"github.com/ProtonMail/gluon/async"
	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/stream"
	"github.com/bradenaw/juniper/xslices"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestServer_LoginLogout(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)
			require.Equal(t, "user", user.Name)
			require.Equal(t, "user@"+s.GetDomain(), user.Email)

			// Logout from the test API.
			require.NoError(t, c.AuthDelete(ctx))

			// Future requests should fail.
			require.Error(t, c.AuthDelete(ctx))
		})
	})
}

func TestServerMulti(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		_, _, err := s.CreateUser("user", []byte("pass"))
		require.NoError(t, err)

		// Create one client.
		c1, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
		require.NoError(t, err)
		defer c1.Close()

		// Create another client.
		c2, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
		require.NoError(t, err)
		defer c2.Close()

		// Both clients should be able to make requests.
		must(c1.GetUser(ctx))
		must(c2.GetUser(ctx))

		// Logout the first client; it should no longer be able to make requests.
		require.NoError(t, c1.AuthDelete(ctx))
		require.Panics(t, func() { must(c1.GetUser(ctx)) })

		// The second client should still be able to make requests.
		must(c2.GetUser(ctx))
	})
}

func TestServer_Ping(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, _ *proton.Manager) {
		ctl := proton.NewNetCtl()

		m := proton.New(
			proton.WithHostURL(s.GetHostURL()),
			proton.WithTransport(ctl.NewRoundTripper(&tls.Config{InsecureSkipVerify: true})),
		)

		var status proton.Status

		m.AddStatusObserver(func(s proton.Status) {
			status = s
		})

		// When the network goes down, ping should fail.
		ctl.Disable()
		require.Error(t, m.Ping(ctx))
		require.Equal(t, proton.StatusDown, status)

		// When the network goes up, ping should succeed.
		ctl.Enable()
		require.NoError(t, m.Ping(ctx))
		require.Equal(t, proton.StatusUp, status)

		// When the API is down, ping should still succeed if the API is at least reachable.
		s.SetOffline(true)
		require.NoError(t, m.Ping(ctx))
		require.Equal(t, proton.StatusUp, status)

		// When the API is down, ping should fail if the API cannot be reached.
		ctl.Disable()
		require.Error(t, m.Ping(ctx))
		require.Equal(t, proton.StatusDown, status)

		// When the network goes up, ping should succeed, even if the API is down.
		ctl.Enable()
		require.NoError(t, m.Ping(ctx))
		require.Equal(t, proton.StatusUp, status)

		// When the API comes back alive, ping should succeed.
		s.SetOffline(false)
		require.NoError(t, m.Ping(ctx))
		require.Equal(t, proton.StatusUp, status)
	})
}

func TestServer_Bool(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1, func([]string) {
				metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
				require.NoError(t, err)

				// By default the message is unread.
				require.True(t, bool(must(c.GetMessage(ctx, metadata[0].ID)).Unread))

				// Mark the message as read.
				require.NoError(t, c.MarkMessagesRead(ctx, metadata[0].ID))

				// Now the message is read.
				require.False(t, bool(must(c.GetMessage(ctx, metadata[0].ID)).Unread))
			})
		})
	})
}

func TestServer_Messages(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				// Get the messages.
				metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
				require.NoError(t, err)

				// The messages should be the ones we created.
				require.ElementsMatch(t, messageIDs, xslices.Map(metadata, func(metadata proton.MessageMetadata) string {
					return metadata.ID
				}))

				// The messages should be in All Mail and should be unread.
				for _, message := range metadata {
					require.True(t, bool(message.Unread))
					require.Equal(t, []string{proton.AllMailLabel}, message.LabelIDs)
				}

				// Mark the first three messages as read and put them in archive.
				require.NoError(t, c.MarkMessagesRead(ctx, messageIDs[0], messageIDs[1], messageIDs[2]))
				require.NoError(t, c.LabelMessages(ctx, []string{messageIDs[0], messageIDs[1], messageIDs[2]}, proton.ArchiveLabel))

				// They should now be read.
				require.False(t, bool(must(c.GetMessage(ctx, messageIDs[0])).Unread))
				require.False(t, bool(must(c.GetMessage(ctx, messageIDs[1])).Unread))
				require.False(t, bool(must(c.GetMessage(ctx, messageIDs[2])).Unread))

				// They should now be in archive.
				require.ElementsMatch(t, []string{proton.ArchiveLabel, proton.AllMailLabel}, must(c.GetMessage(ctx, messageIDs[0])).LabelIDs)
				require.ElementsMatch(t, []string{proton.ArchiveLabel, proton.AllMailLabel}, must(c.GetMessage(ctx, messageIDs[1])).LabelIDs)
				require.ElementsMatch(t, []string{proton.ArchiveLabel, proton.AllMailLabel}, must(c.GetMessage(ctx, messageIDs[2])).LabelIDs)

				// Put them in inbox.
				require.NoError(t, c.LabelMessages(ctx, []string{messageIDs[0], messageIDs[1], messageIDs[2]}, proton.ArchiveLabel))
			})
		})
	})
}

func TestServer_GetMessageMetadataPage(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				for _, chunk := range xslices.Chunk(messageIDs, 150) {
					// Get the messages.
					metadata, err := c.GetMessageMetadataPage(ctx, 0, 150, proton.MessageFilter{ID: chunk})
					require.NoError(t, err)

					// The messages should be the ones we created.
					require.ElementsMatch(t, chunk, xslices.Map(metadata, func(metadata proton.MessageMetadata) string {
						return metadata.ID
					}))

				}
			})
		})
	})
}

func TestServer_MessageFilter(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				// Get the messages.
				metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
				require.NoError(t, err)

				// The messages should be the ones we created.
				require.ElementsMatch(t, messageIDs, xslices.Map(metadata, func(metadata proton.MessageMetadata) string {
					return metadata.ID
				}))

				// Get metadata for just the first three messages.
				partial, err := c.GetMessageMetadata(ctx, proton.MessageFilter{
					ID: []string{
						metadata[0].ID,
						metadata[1].ID,
						metadata[2].ID,
					},
				})
				require.NoError(t, err)

				// The messages should be just the first three.
				require.Equal(t, metadata[:3], partial)
			})
		})
	})
}

func TestServer_MessageFilterDesc(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 100, func(messageIDs []string) {
				allMetadata := make([]proton.MessageMetadata, 0, 100)

				// first request.
				{
					metadata, err := c.GetMessageMetadataPage(ctx, 0, 10, proton.MessageFilter{Desc: true})
					require.NoError(t, err)

					allMetadata = append(allMetadata, metadata...)
				}

				for i := 1; i < 11; i++ {
					// Get the messages.
					metadata, err := c.GetMessageMetadataPage(ctx, 0, 10, proton.MessageFilter{Desc: true, EndID: allMetadata[len(allMetadata)-1].ID})
					require.NoError(t, err)
					require.NotEmpty(t, metadata)
					require.Equal(t, metadata[0].ID, allMetadata[len(allMetadata)-1].ID)
					allMetadata = append(allMetadata, metadata[1:]...)
				}

				// Final check. Asking for EndID as last message multiple times will always return the last id.
				metadata, err := c.GetMessageMetadataPage(ctx, 0, 10, proton.MessageFilter{Desc: true, EndID: allMetadata[len(allMetadata)-1].ID})
				require.NoError(t, err)
				require.Len(t, metadata, 1)
				require.Equal(t, metadata[0].ID, allMetadata[len(allMetadata)-1].ID)

				// The messages should be the ones we created.
				require.ElementsMatch(t, messageIDs, xslices.Map(allMetadata, func(metadata proton.MessageMetadata) string {
					return metadata.ID
				}))
			})
		})
	})
}

func TestServer_MessageIDs(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 10000, func(wantMessageIDs []string) {
				allMessageIDs, err := c.GetAllMessageIDs(ctx, "")
				require.NoError(t, err)
				require.ElementsMatch(t, wantMessageIDs, allMessageIDs)

				halfMessageIDs, err := c.GetAllMessageIDs(ctx, allMessageIDs[len(allMessageIDs)/2])
				require.NoError(t, err)
				require.ElementsMatch(t, allMessageIDs[len(allMessageIDs)/2+1:], halfMessageIDs)
			})
		})
	})
}

func TestServer_MessagesDelete(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				// Get the messages.
				metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
				require.NoError(t, err)

				// The messages should be the ones we created.
				require.ElementsMatch(t, messageIDs, xslices.Map(metadata, func(metadata proton.MessageMetadata) string {
					return metadata.ID
				}))

				// Delete half the messages.
				require.NoError(t, c.DeleteMessage(ctx, messageIDs[0:500]...))

				// Get the remaining messages.
				remaining, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
				require.NoError(t, err)

				// The remaining messages should be the ones we didn't delete.
				require.ElementsMatch(t, messageIDs[500:], xslices.Map(remaining, func(metadata proton.MessageMetadata) string {
					return metadata.ID
				}))
			})
		})
	})
}

func TestServer_MessagesDeleteAfterUpdate(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				// Get the initial event ID.
				eventID, err := c.GetLatestEventID(ctx)
				require.NoError(t, err)

				// Put half the messages in archive.
				require.NoError(t, c.LabelMessages(ctx, messageIDs[0:500], proton.ArchiveLabel))

				// Delete half the messages.
				require.NoError(t, c.DeleteMessage(ctx, messageIDs[0:500]...))

				// Get the event reflecting this change.
				event, more, err := c.GetEvent(ctx, eventID)
				require.NoError(t, err)
				require.False(t, more)
				require.Equal(t, 1, len(event))

				// The event should have the correct number of message events.
				require.Len(t, event[0].Messages, 500)

				// All the events should be delete events.
				for _, message := range event[0].Messages {
					require.Equal(t, proton.EventDelete, message.Action)
				}
			})
		})
	})
}

func TestServer_Events(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 3, func(messageIDs []string) {
				// Get the latest event ID to stream from.
				fromEventID, err := c.GetLatestEventID(ctx)
				require.NoError(t, err)

				// Begin collecting events.
				eventCh := c.NewEventStream(ctx, time.Second, 0, fromEventID)

				// Mark a message as read.
				require.NoError(t, c.MarkMessagesRead(ctx, messageIDs[0]))

				// The message should eventually be read.
				require.Eventually(t, func() bool {
					event := <-eventCh

					if len(event.Messages) != 1 {
						return false
					}

					if event.Messages[0].ID != messageIDs[0] {
						return false
					}

					return !bool(event.Messages[0].Message.Unread)
				}, 5*time.Second, time.Millisecond*100)

				// Add another message to archive.
				require.NoError(t, c.LabelMessages(ctx, []string{messageIDs[1]}, proton.ArchiveLabel))

				// The message should eventually be in archive and all mail.
				require.Eventually(t, func() bool {
					event := <-eventCh

					if len(event.Messages) != 1 {
						return false
					}

					if event.Messages[0].ID != messageIDs[1] {
						return false
					}

					return elementsMatch([]string{proton.ArchiveLabel, proton.AllMailLabel}, event.Messages[0].Message.LabelIDs)
				}, 5*time.Second, time.Millisecond*100)

				// Perform a sequence of actions on the same message.
				require.NoError(t, c.LabelMessages(ctx, []string{messageIDs[2]}, proton.TrashLabel))
				require.NoError(t, c.UnlabelMessages(ctx, []string{messageIDs[2]}, proton.TrashLabel))
				require.NoError(t, c.MarkMessagesRead(ctx, messageIDs[2]))
				require.NoError(t, c.MarkMessagesUnread(ctx, messageIDs[2]))

				// The final state of the message should be correct.
				require.Eventually(t, func() bool {
					event := <-eventCh

					if len(event.Messages) != 1 {
						return false
					}

					if event.Messages[0].ID != messageIDs[2] {
						return false
					}

					return bool(event.Messages[0].Message.Unread) && elementsMatch([]string{proton.AllMailLabel}, event.Messages[0].Message.LabelIDs)
				}, 5*time.Second, time.Millisecond*100)

				// No more events should be sent.
				select {
				case <-eventCh:
					t.Fatal("unexpected event")

				default:
					// ....
				}
			})
		})
	})
}

func TestServer_Events_Multi(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		for i := 0; i < 10; i++ {
			withUser(ctx, t, s, m, fmt.Sprintf("user%v", i), "pass", func(c *proton.Client) {
				latest, err := c.GetLatestEventID(ctx)
				require.NoError(t, err)

				// Fetching latest again should return the same event ID.
				latestAgain, err := c.GetLatestEventID(ctx)
				require.NoError(t, err)
				require.Equal(t, latest, latestAgain)

				events, more, err := c.GetEvent(ctx, latest)
				require.NoError(t, err)
				require.False(t, more)

				// The event should be empty.
				require.Equal(t, []proton.Event{{EventID: events[0].EventID}}, events)

				// After fetching an empty event, its ID should still be the latest.
				eventAgain, more, err := c.GetEvent(ctx, events[0].EventID)
				require.NoError(t, err)
				require.False(t, more)
				require.Equal(t, eventAgain[0].EventID, events[0].EventID)
			})
		}
	})
}

func TestServer_Events_Refresh(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			// Get the latest event ID to stream from.
			fromEventID, err := c.GetLatestEventID(ctx)
			require.NoError(t, err)

			// Refresh the user's mail.
			require.NoError(t, s.RefreshUser(user.ID, proton.RefreshMail))

			// Begin collecting events.
			eventCh := c.NewEventStream(ctx, time.Second, 0, fromEventID)

			// The user should eventually be refreshed.
			require.Eventually(t, func() bool {
				return (<-eventCh).Refresh&proton.RefreshMail != 0
			}, 5*time.Second, time.Millisecond*100)
		})
	})
}

func TestServer_Events_UserSettings(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			_, err = s.b.SetUserSettingsTelemetry(user.ID, proton.SettingDisabled)
			require.NoError(t, err)

			// Get the latest event ID to stream from.
			fromEventID, err := c.GetLatestEventID(ctx)
			require.NoError(t, err)

			// Refresh the user's mail.
			_, err = s.b.SetUserSettingsTelemetry(user.ID, proton.SettingEnabled)
			require.NoError(t, err)

			// Begin collecting events.
			eventCh := c.NewEventStream(ctx, time.Second, 0, fromEventID)

			// The user should eventually be refreshed.
			require.Eventually(t, func() bool {
				e := <-eventCh
				return e.UserSettings != nil && e.UserSettings.Telemetry == proton.SettingEnabled
			}, 5*time.Second, time.Millisecond*100)
		})
	})
}

func TestServer_RevokeUser(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)
			require.Equal(t, "user", user.Name)
			require.Equal(t, "user@"+s.GetDomain(), user.Email)

			// Revoke the user's auth.
			require.NoError(t, s.RevokeUser(user.ID))

			// Future requests should fail.
			require.Error(t, c.AuthDelete(ctx))
		})
	})
}

func TestServer_Calls(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			var calls []Call

			// Watch calls that are made.
			s.AddCallWatcher(func(call Call) {
				calls = append(calls, call)
			})

			// Get the user.
			_, err := c.GetUser(ctx)
			require.NoError(t, err)

			// Logout the user.
			require.NoError(t, c.AuthDelete(ctx))

			// The user call should be correct.
			userCall := calls[0]
			require.Equal(t, "/core/v4/users", userCall.URL.Path)
			require.Equal(t, "GET", userCall.Method)
			require.Equal(t, http.StatusOK, userCall.Status)

			// The logout call should be correct.
			logoutCall := calls[1]
			require.Equal(t, "/auth/v4", logoutCall.URL.Path)
			require.Equal(t, "DELETE", logoutCall.Method)
			require.Equal(t, http.StatusOK, logoutCall.Status)
		})
	})
}

func TestServer_Calls_Status(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			var calls []Call

			// Watch calls that are made.
			s.AddCallWatcher(func(call Call) {
				calls = append(calls, call)
			})

			// Make a bad call.
			_, err := c.GetMessage(ctx, "no such message ID")
			require.Error(t, err)

			// The user call should have error status.
			require.Equal(t, http.StatusUnprocessableEntity, calls[0].Status)
		})
	})
}

func TestServer_Calls_Request(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		var calls []Call

		s.AddCallWatcher(func(call Call) {
			calls = append(calls, call)
		})

		withUser(ctx, t, s, m, "user", "pass", func(*proton.Client) {
			require.Equal(
				t,
				calls[0].RequestBody,
				must(json.Marshal(proton.AuthInfoReq{Username: "user"})),
			)
		})
	})
}

func TestServer_Calls_Response(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		var calls []Call

		s.AddCallWatcher(func(call Call) {
			calls = append(calls, call)
		})

		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			salts, err := c.GetSalts(ctx)
			require.NoError(t, err)

			require.Equal(
				t,
				calls[len(calls)-1].ResponseBody,
				must(json.Marshal(struct{ KeySalts []proton.Salt }{salts})),
			)
		})
	})
}

func TestServer_Calls_Cookies(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		var calls []Call

		s.AddCallWatcher(func(call Call) {
			calls = append(calls, call)
		})

		withUser(ctx, t, s, m, "user", "pass", func(*proton.Client) {
			// The header in the first call's response should set the Session-Id cookie.
			resHeader := (&http.Response{Header: calls[len(calls)-2].ResponseHeader})
			require.Len(t, resHeader.Cookies(), 1)
			require.Equal(t, "Session-Id", resHeader.Cookies()[0].Name)
			require.NotEmpty(t, resHeader.Cookies()[0].Value)

			// The cookie should be sent in the next call.
			reqHeader := (&http.Request{Header: calls[len(calls)-1].RequestHeader})
			require.Len(t, reqHeader.Cookies(), 1)
			require.Equal(t, "Session-Id", reqHeader.Cookies()[0].Name)
			require.NotEmpty(t, reqHeader.Cookies()[0].Value)

			// The cookie should be the same.
			require.Equal(t, resHeader.Cookies()[0].Value, reqHeader.Cookies()[0].Value)
		})
	})
}

func TestServer_Calls_Manager(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		var calls []Call

		// Watch calls that are made.
		s.AddCallWatcher(func(call Call) {
			calls = append(calls, call)
		})

		// Make a non-user request.
		require.NoError(t, m.ReportBug(ctx, proton.ReportBugReq{}))

		// The call should be correct.
		reportCall := calls[0]
		require.Equal(t, "/core/v4/reports/bug", reportCall.URL.Path)
		require.Equal(t, "POST", reportCall.Method)
		require.Equal(t, http.StatusOK, reportCall.Status)
	})
}

func TestServer_CreateMessage(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			draft, err := c.CreateDraft(ctx, addrKRs[addr[0].ID], proton.CreateDraftReq{
				Message: proton.DraftTemplate{
					Subject: "My subject",
					Sender:  &mail.Address{Address: addr[0].Email},
					ToList:  []*mail.Address{{Address: "recipient@example.com"}},
				},
			})
			require.NoError(t, err)

			require.Equal(t, addr[0].ID, draft.AddressID)
			require.Equal(t, "My subject", draft.Subject)
			require.Equal(t, &mail.Address{Address: "user@" + s.GetDomain()}, draft.Sender)
			require.Equal(t, []*mail.Address{{Address: "recipient@example.com"}}, draft.ToList)
			require.ElementsMatch(t, []string{proton.AllMailLabel, proton.AllDraftsLabel, proton.DraftsLabel}, draft.LabelIDs)
		})
	})
}

func TestServer_UpdateDraft(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			// Create the draft.
			draft, err := c.CreateDraft(ctx, addrKRs[addr[0].ID], proton.CreateDraftReq{
				Message: proton.DraftTemplate{
					Subject: "My subject",
					Sender:  &mail.Address{Address: addr[0].Email},
					ToList:  []*mail.Address{{Address: "recipient@example.com"}},
				},
			})
			require.NoError(t, err)
			require.Equal(t, addr[0].ID, draft.AddressID)
			require.Equal(t, "My subject", draft.Subject)
			require.Equal(t, &mail.Address{Address: "user@" + s.GetDomain()}, draft.Sender)
			require.Equal(t, []*mail.Address{{Address: "recipient@example.com"}}, draft.ToList)

			// Create an event stream to watch for an update event.
			fromEventID, err := c.GetLatestEventID(ctx)
			require.NoError(t, err)

			eventCh := c.NewEventStream(ctx, time.Second, 0, fromEventID)

			// Update the draft subject/to-list.
			msg, err := c.UpdateDraft(ctx, draft.ID, addrKRs[addr[0].ID], proton.UpdateDraftReq{
				Message: proton.DraftTemplate{
					Subject:  "Edited subject",
					Sender:   &mail.Address{Address: addr[0].Email},
					ToList:   []*mail.Address{{Address: "edited@example.com"}},
					MIMEType: rfc822.TextPlain,
				},
			})
			require.NoError(t, err)
			require.Equal(t, "Edited subject", msg.Subject)

			// We should eventually get an update event.
			require.Eventually(t, func() bool {
				event := <-eventCh

				if len(event.Messages) < 1 {
					return false
				}

				if event.Messages[0].ID != draft.ID {
					return false
				}

				if event.Messages[0].Action != proton.EventUpdate {
					return false
				}

				require.Equal(t, draft.ID, event.Messages[0].ID)
				require.Equal(t, "Edited subject", event.Messages[0].Message.Subject)
				require.Equal(t, []*mail.Address{{Address: "edited@example.com"}}, event.Messages[0].Message.ToList)

				return true
			}, 5*time.Second, time.Millisecond*100)
		})
	})
}

func TestServer_SendMessage(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			draft, err := c.CreateDraft(ctx, addrKRs[addr[0].ID], proton.CreateDraftReq{
				Message: proton.DraftTemplate{
					Subject: "My subject",
					Sender:  &mail.Address{Address: addr[0].Email},
					ToList:  []*mail.Address{{Address: "recipient@example.com"}},
				},
			})
			require.NoError(t, err)

			sent, err := c.SendDraft(ctx, draft.ID, proton.SendDraftReq{})
			require.NoError(t, err)

			require.Equal(t, draft.ID, sent.ID)
			require.Equal(t, addr[0].ID, sent.AddressID)
			require.Equal(t, "My subject", sent.Subject)
			require.Equal(t, []*mail.Address{{Address: "recipient@example.com"}}, sent.ToList)
			require.Contains(t, sent.LabelIDs, proton.SentLabel)
		})
	})
}

func TestServer_AuthDelete(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			require.NoError(t, c.AuthDelete(ctx))
		})
	})
}

func TestServer_ForceUpgrade(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New()
	defer s.Close()

	s.SetMinAppVersion(semver.MustParse("1.0.0"))

	if _, _, err := s.CreateUser("user", []byte("pass")); err != nil {
		t.Fatal(err)
	}

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithAppVersion("proton_0.9.0"),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	var called bool

	m.AddErrorHandler(proton.AppVersionBadCode, func() {
		called = true
	})

	if _, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass")); err == nil {
		t.Fatal(err)
	}

	require.True(t, called)
}

func TestServer_Import(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			res := importMessages(ctx, t, c, addr[0].ID, addrKRs[addr[0].ID], []string{}, proton.MessageFlagReceived, 1)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, proton.SuccessCode, res[0].Code)

			message, err := c.GetMessage(ctx, res[0].MessageID)
			require.NoError(t, err)

			dec, err := message.Decrypt(addrKRs[message.AddressID])
			require.NoError(t, err)
			require.NotEmpty(t, dec)
		})
	})
}

func TestServer_Import_Dedup(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			subjectGenerator := func() string {
				return "my Subject"
			}

			res := importMessagesWithSubjectGenerator(
				ctx,
				t,
				c,
				addr[0].ID,
				addrKRs[addr[0].ID],
				[]string{},
				proton.MessageFlagReceived,
				1,
				subjectGenerator,
			)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, proton.SuccessCode, res[0].Code)

			message, err := c.GetMessage(ctx, res[0].MessageID)
			require.NoError(t, err)

			dec, err := message.Decrypt(addrKRs[message.AddressID])
			require.NoError(t, err)
			require.NotEmpty(t, dec)

			// Import message again should be deduped.
			resDedup := importMessagesWithSubjectGenerator(
				ctx,
				t,
				c,
				addr[0].ID,
				addrKRs[addr[0].ID],
				[]string{},
				proton.MessageFlagReceived,
				1,
				subjectGenerator,
			)
			require.NoError(t, err)
			require.Len(t, resDedup, 1)
			require.Equal(t, proton.SuccessCode, resDedup[0].Code)
			require.Equal(t, res[0].MessageID, resDedup[0].MessageID)
		})
	}, WithMessageDedup())
}

func TestServer_Labels(t *testing.T) {
	type add string
	type rem string

	tests := []struct {
		name         string
		flags        proton.MessageFlag
		actions      []any
		wantLabelIDs []string
		wantError    bool
	}{
		{
			name:         "received flag, no actions",
			flags:        proton.MessageFlagReceived,
			wantLabelIDs: []string{proton.AllMailLabel},
		},
		{
			name:         "sent flag, no actions",
			flags:        proton.MessageFlagSent,
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, no actions",
			flags:        proton.MessageFlagScheduledSend,
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel)},
			wantLabelIDs: []string{proton.InboxLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel)},
			wantLabelIDs: []string{proton.SentLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel)},
			wantLabelIDs: []string{proton.AllScheduledLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox then add archive",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent then add archive",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled then add archive",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox then remove inbox",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), rem(proton.InboxLabel)},
			wantLabelIDs: []string{proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent then remove sent",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel), rem(proton.SentLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled then remove scheduled",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel), rem(proton.AllScheduledLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox then remove archive",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), rem(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.InboxLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent then remove archive",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel), rem(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.SentLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled then remove archive",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel), rem(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.AllScheduledLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox then remove inbox then add archive",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), rem(proton.InboxLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent then remove sent then add archive",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel), rem(proton.SentLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled then remove scheduled then add archive",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel), rem(proton.AllScheduledLabel), add(proton.ArchiveLabel)},
			wantLabelIDs: []string{proton.ArchiveLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add starred",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.StarredLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add starred",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.StarredLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add starred",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.StarredLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox, add starred, remove inbox",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), add(proton.StarredLabel), rem(proton.InboxLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag, add sent, add starred, remove sent",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.SentLabel), add(proton.StarredLabel), rem(proton.SentLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add scheduled, add starred, remove scheduled",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.AllScheduledLabel), add(proton.StarredLabel), rem(proton.AllScheduledLabel)},
			wantLabelIDs: []string{proton.StarredLabel, proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add trash, remove trash",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.TrashLabel), rem(proton.TrashLabel)},
			wantLabelIDs: []string{proton.AllMailLabel},
		},
		{
			name:         "sent flag, add trash, remove trash",
			flags:        proton.MessageFlagSent,
			actions:      []any{add(proton.TrashLabel), rem(proton.TrashLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "scheduled flag, add trash, remove trash",
			flags:        proton.MessageFlagScheduledSend,
			actions:      []any{add(proton.TrashLabel), rem(proton.TrashLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.AllSentLabel},
		},
		{
			name:         "received flag, add inbox, add trash, remove inbox",
			flags:        proton.MessageFlagReceived,
			actions:      []any{add(proton.InboxLabel), add(proton.TrashLabel), rem(proton.InboxLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.TrashLabel},
		},
		{
			name:         "scheduled & sent flags, add scheduled, add sent",
			flags:        proton.MessageFlagScheduledSend | proton.MessageFlagSent,
			actions:      []any{add(proton.AllScheduledLabel), add(proton.SentLabel)},
			wantLabelIDs: []string{proton.AllMailLabel, proton.SentLabel, proton.AllSentLabel},
		},
	}

	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					res := importMessages(ctx, t, c, addr[0].ID, addrKRs[addr[0].ID], []string{}, tt.flags, 1)

					require.True(t, (func() error {
						for _, action := range tt.actions {
							switch action := action.(type) {
							case add:
								if err := c.LabelMessages(ctx, []string{res[0].MessageID}, string(action)); err != nil {
									return err
								}

							case rem:
								if err := c.UnlabelMessages(ctx, []string{res[0].MessageID}, string(action)); err != nil {
									return err
								}
							}
						}

						return nil
					}() != nil) == tt.wantError)

					message, err := c.GetMessage(ctx, res[0].MessageID)
					require.NoError(t, err)

					// The message should be in the correct labels.
					require.ElementsMatch(t, tt.wantLabelIDs, message.LabelIDs)

					// The flags should be preserved after import.
					require.True(t, message.Flags&tt.flags == tt.flags)
				})
			}
		})
	})
}

func TestServer_Import_FlagsAndLabels(t *testing.T) {
	tests := []struct {
		name         string
		labelIDs     []string
		flags        proton.MessageFlag
		wantLabelIDs []string
		wantError    bool
	}{
		{
			name:         "received flag --> no label",
			flags:        proton.MessageFlagReceived,
			wantLabelIDs: []string{proton.AllMailLabel},
		},
		{
			name:         "received flag --> inbox",
			labelIDs:     []string{proton.InboxLabel},
			flags:        proton.MessageFlagReceived,
			wantLabelIDs: []string{proton.InboxLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag --> sent",
			labelIDs:     []string{proton.SentLabel},
			flags:        proton.MessageFlagSent,
			wantLabelIDs: []string{proton.SentLabel, proton.AllSentLabel, proton.AllMailLabel},
		},
		{
			name:         "received flag --> sent",
			labelIDs:     []string{proton.SentLabel},
			flags:        proton.MessageFlagReceived,
			wantLabelIDs: []string{proton.InboxLabel, proton.AllMailLabel},
		},
		{
			name:         "sent flag --> inbox",
			labelIDs:     []string{proton.InboxLabel},
			flags:        proton.MessageFlagSent,
			wantLabelIDs: []string{proton.SentLabel, proton.AllSentLabel, proton.AllMailLabel},
		},
		{
			name:         "no flag --> drafts",
			labelIDs:     []string{proton.DraftsLabel},
			wantLabelIDs: []string{proton.DraftsLabel, proton.AllDraftsLabel, proton.AllMailLabel},
		},
		{
			name:      "forbidden: received flag --> All Mail",
			labelIDs:  []string{proton.AllMailLabel},
			flags:     proton.MessageFlagReceived,
			wantError: true,
		},
		{
			name:      "forbidden: sent flag --> All Mail",
			labelIDs:  []string{proton.AllMailLabel},
			flags:     proton.MessageFlagSent,
			wantError: true,
		},
		{
			name:      "forbidden: received flag --> inbox and all mail",
			labelIDs:  []string{proton.InboxLabel, proton.AllMailLabel},
			flags:     proton.MessageFlagReceived,
			wantError: true,
		},
		{
			name:      "forbidden: sent flag --> sent and all mail",
			labelIDs:  []string{proton.SentLabel, proton.AllMailLabel},
			flags:     proton.MessageFlagSent,
			wantError: true,
		},
		{
			name:      "forbidden: received flag --> inbox and sent",
			labelIDs:  []string{proton.InboxLabel, proton.SentLabel},
			flags:     proton.MessageFlagReceived,
			wantError: true,
		},
		{
			name:      "forbidden: sent flag --> inbox and sent",
			labelIDs:  []string{proton.InboxLabel, proton.SentLabel},
			flags:     proton.MessageFlagSent,
			wantError: true,
		},
		{
			name:      "forbidden: received flag --> inbox and archive",
			labelIDs:  []string{proton.InboxLabel, proton.ArchiveLabel},
			flags:     proton.MessageFlagReceived,
			wantError: true,
		},
	}

	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					str, err := c.ImportMessages(ctx, addrKRs[addr[0].ID], runtime.NumCPU(), runtime.NumCPU(), []proton.ImportReq{{
						Metadata: proton.ImportMetadata{
							AddressID: addr[0].ID,
							Flags:     tt.flags,
							LabelIDs:  tt.labelIDs,
						},
						Message: newMessageLiteral("sender@example.com", "recipient@example.com"),
					}}...)
					require.NoError(t, err)

					res, err := stream.Collect(ctx, str)
					if tt.wantError {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.Equal(t, proton.SuccessCode, res[0].Code)

						message, err := c.GetMessage(ctx, res[0].MessageID)
						require.NoError(t, err)

						// The message should be in the correct labels.
						require.ElementsMatch(t, tt.wantLabelIDs, message.LabelIDs)

						// The flags should be preserved after import.
						require.True(t, message.Flags&tt.flags == tt.flags)
					}
				})
			}
		})
	})
}

func TestServer_PublicKeys(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		if _, _, err := s.CreateUser("other", []byte("pass")); err != nil {
			t.Fatal(err)
		}

		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			intKeys, intType, err := c.GetPublicKeys(ctx, "other@"+s.GetDomain())
			require.NoError(t, err)
			require.Equal(t, proton.RecipientTypeInternal, intType)
			require.Len(t, intKeys, 1)

			extKeys, extType, err := c.GetPublicKeys(ctx, "other@example.com")
			require.NoError(t, err)
			require.Equal(t, proton.RecipientTypeExternal, extType)
			require.Len(t, extKeys, 0)
		})
	})
}

func TestServer_Proxy(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		var calls []Call

		s.AddCallWatcher(func(call Call) {
			calls = append(calls, call)
		})

		withUser(ctx, t, s, m, "user", "pass", func(_ *proton.Client) {
			proxy := New(
				WithProxyOrigin(s.GetHostURL()),
				WithProxyTransport(proton.InsecureTransport()),
			)
			defer proxy.Close()

			m := proton.New(
				proton.WithHostURL(proxy.GetProxyURL()),
				proton.WithTransport(proton.InsecureTransport()),
			)
			defer m.Close()

			// Login -- the call should be proxied to the upstream server.
			c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
			require.NoError(t, err)
			defer c.Close()

			// The results of the call should be correct.
			user, err := c.GetUser(ctx)
			require.NoError(t, err)
			require.Equal(t, "user", user.Name)
		})

		// Assert that the calls were proxied.
		require.Greater(t, len(calls), 0)
	})
}

func TestServer_Proxy_Cache(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(_ *proton.Client) {
			proxy := New(
				WithProxyOrigin(s.GetHostURL()),
				WithProxyTransport(proton.InsecureTransport()),
				WithAuthCacher(NewAuthCache()),
			)
			defer proxy.Close()

			// Need to skip verifying the server proofs for the proxy cache feature to work!
			m := proton.New(
				proton.WithHostURL(proxy.GetProxyURL()),
				proton.WithTransport(proton.InsecureTransport()),
				proton.WithSkipVerifyProofs(),
			)
			defer m.Close()

			// Login 3 times; we should produce 1 unique auth.
			require.Len(t, xslices.Unique(iterator.Collect(iterator.Map(iterator.Counter(3), func(int) string {
				c, auth, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
				require.NoError(t, err)
				defer c.Close()

				return auth.UID
			}))), 1)
		})
	})
}

func TestServer_Proxy_AuthDelete(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(_ *proton.Client) {
			proxy := New(
				WithProxyOrigin(s.GetHostURL()),
				WithProxyTransport(proton.InsecureTransport()),
				WithAuthCacher(NewAuthCache()),
			)
			defer proxy.Close()

			// Need to skip verifying the server proofs for the proxy cache feature to work!
			m := proton.New(
				proton.WithHostURL(proxy.GetProxyURL()),
				proton.WithTransport(proton.InsecureTransport()),
			)
			defer m.Close()

			// Watch for login -- the calls should be proxied.
			var login []Call

			s.AddCallWatcher(func(call Call) {
				login = append(login, call)
			})

			// Login -- the call should be proxied to the upstream server.
			c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
			require.NoError(t, err)
			defer c.Close()

			// Assert that the login was proxied.
			require.NotEmpty(t, len(login))

			// Watch for logout -- logout should not be proxied to the upstream server.
			var logout []Call

			s.AddCallWatcher(func(call Call) {
				logout = append(logout, call)
			})

			// Logout -- the call should not be proxied to the upstream server.
			require.NoError(t, c.AuthDelete(ctx))

			// Assert that the logout was not proxied!
			require.Empty(t, len(logout))
		})
	})
}

func TestServer_RealProxy(t *testing.T) {
	username := os.Getenv("GO_PROTON_API_TEST_USERNAME")
	password := os.Getenv("GO_PROTON_API_TEST_PASSWORD")

	if username == "" || password == "" {
		t.Skip("skipping test, set the username and password to run")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy := New()
	defer proxy.Close()

	m := proton.New(
		proton.WithHostURL(proxy.GetProxyURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Login -- the call should be proxied to the upstream server.
	c, _, err := m.NewClientWithLogin(ctx, username, []byte(password))
	require.NoError(t, err)
	defer c.Close()

	// The results of the call should be correct.
	user, err := c.GetUser(ctx)
	require.NoError(t, err)
	require.Equal(t, username, user.Name)
}

func TestServer_RealProxy_Cache(t *testing.T) {
	username := os.Getenv("GO_PROTON_API_TEST_USERNAME")
	password := os.Getenv("GO_PROTON_API_TEST_PASSWORD")

	if username == "" || password == "" {
		t.Skip("skipping test, set the username and password to run")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy := New(WithAuthCacher(NewAuthCache()))
	defer proxy.Close()

	m := proton.New(
		proton.WithHostURL(proxy.GetProxyURL()),
		proton.WithTransport(proton.InsecureTransport()),
		proton.WithSkipVerifyProofs(),
	)
	defer m.Close()

	// Login 3 times; we should produce 1 unique auth.
	require.Len(t, xslices.Unique(iterator.Collect(iterator.Map(iterator.Counter(3), func(int) string {
		c, auth, err := m.NewClientWithLogin(ctx, username, []byte(password))
		require.NoError(t, err)
		defer c.Close()

		return auth.UID
	}))), 1)
}

func TestServer_Messages_Fetch(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			withMessages(ctx, t, c, "pass", 1000, func(messageIDs []string) {
				ctl := proton.NewNetCtl()

				mm := proton.New(
					proton.WithHostURL(s.GetHostURL()),
					proton.WithTransport(ctl.NewRoundTripper(&tls.Config{InsecureSkipVerify: true})),
				)
				defer mm.Close()

				cc, _, err := mm.NewClientWithLogin(ctx, "user", []byte("pass"))
				require.NoError(t, err)
				defer cc.Close()

				total := countBytesRead(ctl, func() {
					res, err := stream.Collect(ctx, getFullMessages(ctx, cc, runtime.NumCPU(), runtime.NumCPU(), messageIDs...))
					require.NoError(t, err)
					require.NotEmpty(t, res)
				})

				ctl.SetReadLimit(total / 2)

				require.Less(t, countBytesRead(ctl, func() {
					res, err := stream.Collect(ctx, getFullMessages(ctx, cc, runtime.NumCPU(), runtime.NumCPU(), messageIDs...))
					require.Error(t, err)
					require.Empty(t, res)
				}), total)

				ctl.SetReadLimit(0)

				require.Equal(t, countBytesRead(ctl, func() {
					res, err := stream.Collect(ctx, getFullMessages(ctx, cc, runtime.NumCPU(), runtime.NumCPU(), messageIDs...))
					require.NoError(t, err)
					require.NotEmpty(t, res)
				}), total)
			})
		})
	}, WithTLS(false))
}

func TestServer_Status(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(*proton.Client) {
			ctl := proton.NewNetCtl()

			mm := proton.New(
				proton.WithHostURL(s.GetHostURL()),
				proton.WithTransport(ctl.NewRoundTripper(&tls.Config{InsecureSkipVerify: true})),
			)
			defer mm.Close()

			statusCh := make(chan proton.Status, 1)

			mm.AddStatusObserver(func(status proton.Status) {
				statusCh <- status
			})

			cc, _, err := mm.NewClientWithLogin(ctx, "user", []byte("pass"))
			require.NoError(t, err)
			defer cc.Close()

			{
				user, err := cc.GetUser(ctx)
				require.NoError(t, err)
				require.Equal(t, "user", user.Name)
			}

			ctl.SetCanRead(false)

			{
				_, err := cc.GetUser(ctx)
				require.Error(t, err)
			}

			require.Equal(t, proton.StatusDown, <-statusCh)
		})
	}, WithTLS(false))
}

func TestServer_Labels_Duplicates(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			req := proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeLabel,
			}

			label, err := c.CreateLabel(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, req.Name, label.Name)

			_, err = c.CreateLabel(context.Background(), req)
			require.Error(t, err)
		})
	})
}

func TestServer_Labels_Duplicates_Update(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			label1, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeLabel,
			})
			require.NoError(t, err)

			label2, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeLabel,
			})
			require.NoError(t, err)

			// Updating label1 with label2's name should fail.
			_, err = c.UpdateLabel(context.Background(), label1.ID, proton.UpdateLabelReq{
				Name:  label2.Name,
				Color: label1.Color,
			})
			require.Error(t, err)

			// Updating label1's color while preserving its name should succeed.
			_, err = c.UpdateLabel(context.Background(), label1.ID, proton.UpdateLabelReq{
				Name:  label1.Name,
				Color: "#f00",
			})
			require.NoError(t, err)

			// Updating label1 with a new name should succeed.
			_, err = c.UpdateLabel(context.Background(), label1.ID, proton.UpdateLabelReq{
				Name:  uuid.NewString(),
				Color: label1.Color,
			})
			require.NoError(t, err)
		})
	})
}

func TestServer_Labels_Subfolders(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			parent, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeFolder,
			})
			require.NoError(t, err)

			child, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:     uuid.NewString(),
				ParentID: parent.ID,
				Color:    "#f66",
				Type:     proton.LabelTypeFolder,
			})
			require.NoError(t, err)
			require.Equal(t, []string{parent.Name, child.Name}, child.Path)

			child2, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:     uuid.NewString(),
				ParentID: child.ID,
				Color:    "#f66",
				Type:     proton.LabelTypeFolder,
			})
			require.NoError(t, err)
			require.Equal(t, []string{parent.Name, child.Name, child2.Name}, child2.Path)
		})
	})
}

func TestServer_Labels_Subfolders_Reassign(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			parent1, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeFolder,
			})
			require.NoError(t, err)

			parent2, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeFolder,
			})
			require.NoError(t, err)

			// Create a child initially under parent1.
			child, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:     uuid.NewString(),
				ParentID: parent1.ID,
				Color:    "#f66",
				Type:     proton.LabelTypeFolder,
			})
			require.NoError(t, err)
			require.Equal(t, []string{parent1.Name, child.Name}, child.Path)

			// Reassign the child to parent2.
			child2, err := c.UpdateLabel(context.Background(), child.ID, proton.UpdateLabelReq{
				Name:     child.Name,
				Color:    child.Color,
				ParentID: parent2.ID,
			})
			require.NoError(t, err)
			require.Equal(t, []string{parent2.Name, child.Name}, child2.Path)

			// Reassign the child to no parent.
			child3, err := c.UpdateLabel(context.Background(), child.ID, proton.UpdateLabelReq{
				Name:     child2.Name,
				Color:    child2.Color,
				ParentID: "",
			})
			require.NoError(t, err)
			require.Equal(t, []string{child3.Name}, child3.Path)
		})
	})
}

func TestServer_Labels_Subfolders_DeleteParentWithChildren(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			parent, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeFolder,
			})
			require.NoError(t, err)

			child, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:     uuid.NewString(),
				ParentID: parent.ID,
				Color:    "#f66",
				Type:     proton.LabelTypeFolder,
			})
			require.NoError(t, err)
			require.Equal(t, []string{parent.Name, child.Name}, child.Path)

			other, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
				Name:  uuid.NewString(),
				Color: "#f66",
				Type:  proton.LabelTypeFolder,
			})
			require.NoError(t, err)

			// Get labels before.
			before, err := c.GetLabels(context.Background(), proton.LabelTypeFolder)
			require.NoError(t, err)

			// Delete the parent.
			require.NoError(t, c.DeleteLabel(context.Background(), parent.ID))

			// Get labels after.
			after, err := c.GetLabels(context.Background(), proton.LabelTypeFolder)
			require.NoError(t, err)

			// Both parent and child are deleted.
			require.Equal(t, len(before)-2, len(after))

			// The only label left is the other one.
			require.Equal(t, other.ID, after[0].ID)
		})
	})
}

func TestServer_AddressCreateDelete(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(context.Background())
			require.NoError(t, err)

			// Create an address.
			alias, err := s.CreateAddress(user.ID, "alias@example.com", []byte("pass"))
			require.NoError(t, err)

			// The user should have two addresses, both enabled.
			{
				addr, err := c.GetAddresses(context.Background())
				require.NoError(t, err)
				require.Len(t, addr, 2)
				require.Equal(t, addr[0].Status, proton.AddressStatusEnabled)
				require.Equal(t, addr[1].Status, proton.AddressStatusEnabled)
			}

			// Disable the alias.
			require.NoError(t, c.DisableAddress(context.Background(), alias))

			// The user should have two addresses, the primary enabled and the alias disabled.
			{
				addr, err := c.GetAddresses(context.Background())
				require.NoError(t, err)
				require.Len(t, addr, 2)
				require.Equal(t, addr[0].Status, proton.AddressStatusEnabled)
				require.Equal(t, addr[1].Status, proton.AddressStatusDisabled)
			}

			// Delete the alias.
			require.NoError(t, c.DeleteAddress(context.Background(), alias))

			// The user should have one address, the primary enabled.
			{
				addr, err := c.GetAddresses(context.Background())
				require.NoError(t, err)
				require.Len(t, addr, 1)
				require.Equal(t, addr[0].Status, proton.AddressStatusEnabled)
			}
		})
	})
}

func TestServer_AddressOrder(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			user, err := c.GetUser(context.Background())
			require.NoError(t, err)

			primary, err := c.GetAddresses(context.Background())
			require.NoError(t, err)

			// Create 3 additional addresses.
			addr1, err := s.CreateAddress(user.ID, "addr1@example.com", []byte("pass"))
			require.NoError(t, err)

			addr2, err := s.CreateAddress(user.ID, "addr2@example.com", []byte("pass"))
			require.NoError(t, err)

			addr3, err := s.CreateAddress(user.ID, "addr3@example.com", []byte("pass"))
			require.NoError(t, err)

			addresses, err := c.GetAddresses(context.Background())
			require.NoError(t, err)

			// Check the order.
			require.Equal(t, primary[0].ID, addresses[0].ID)
			require.Equal(t, addr1, addresses[1].ID)
			require.Equal(t, addr2, addresses[2].ID)
			require.Equal(t, addr3, addresses[3].ID)

			// Update the order.
			require.NoError(t, c.OrderAddresses(ctx, proton.OrderAddressesReq{
				AddressIDs: []string{addr3, addr2, addr1, primary[0].ID},
			}))

			// Check the order.
			addresses, err = c.GetAddresses(context.Background())
			require.NoError(t, err)

			require.Equal(t, addr3, addresses[0].ID)
			require.Equal(t, addr2, addresses[1].ID)
			require.Equal(t, addr1, addresses[2].ID)
			require.Equal(t, primary[0].ID, addresses[3].ID)
		})
	})
}

func TestServer_MailSettings(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			settings, err := c.GetMailSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, proton.Bool(false), settings.AttachPublicKey)

			updated, err := c.SetAttachPublicKey(context.Background(), proton.SetAttachPublicKeyReq{AttachPublicKey: true})
			require.NoError(t, err)
			require.Equal(t, proton.Bool(true), updated.AttachPublicKey)
		})
	})
}

func TestServer_UserSettings(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			settings, err := c.GetUserSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, proton.SettingEnabled, settings.Telemetry)
			require.Equal(t, proton.SettingEnabled, settings.CrashReports)

			settings, err = c.SetUserSettingsTelemetry(context.Background(), proton.SetTelemetryReq{Telemetry: proton.SettingDisabled})
			require.NoError(t, err)
			require.Equal(t, proton.SettingDisabled, settings.Telemetry)
			require.Equal(t, proton.SettingEnabled, settings.CrashReports)

			settings, err = c.SetUserSettingsCrashReports(context.Background(), proton.SetCrashReportReq{CrashReports: proton.SettingDisabled})
			require.NoError(t, err)
			require.Equal(t, proton.SettingDisabled, settings.Telemetry)
			require.Equal(t, proton.SettingDisabled, settings.CrashReports)

			settings, err = c.SetUserSettingsTelemetry(context.Background(), proton.SetTelemetryReq{Telemetry: 2})
			require.Error(t, err)

			settings, err = c.SetUserSettingsCrashReports(context.Background(), proton.SetCrashReportReq{CrashReports: 2})
			require.Error(t, err)

			settings, err = c.GetUserSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, proton.SettingDisabled, settings.Telemetry)
			require.Equal(t, proton.SettingDisabled, settings.CrashReports)

		})
	})
}

func TestServer_Domains(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		domains, err := m.GetDomains(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{s.GetDomain()}, domains)
	})
}

func TestServer_StatusHooks(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		s.AddStatusHook(func(req *http.Request) (int, bool) {
			if req.URL.Path == "/core/v4/addresses" {
				return http.StatusBadRequest, true
			}

			return 0, false
		})

		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			addr, err := c.GetAddresses(context.Background())
			require.Error(t, err)
			require.Nil(t, addr)

			if apiErr := new(proton.APIError); errors.As(err, &apiErr) {
				require.Equal(t, http.StatusBadRequest, apiErr.Status)
			} else {
				require.Fail(t, "expected APIError")
			}
		})
	})
}

func TestServer_SendDataEvent(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			// Send data event minimal
			err := c.SendDataEvent(context.Background(), proton.SendStatsReq{MeasurementGroup: "proton.any.test"})
			require.NoError(t, err)

			// Send data event Full.
			var req proton.SendStatsReq
			req.MeasurementGroup = "proton.any.test"
			req.Event = "test"
			req.Values = map[string]any{"string": "string", "integer": 42}
			req.Dimensions = map[string]any{"string": "string", "integer": 42}
			err = c.SendDataEvent(context.Background(), req)
			require.NoError(t, err)

			// Send bad data event.
			err = c.SendDataEvent(context.Background(), proton.SendStatsReq{})
			require.Error(t, err)
		})
	})
}

func TestServer_SendDataEventMultiple(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			// Send multiple minimal data event.
			var req proton.SendStatsMultiReq
			req.EventInfo = append(req.EventInfo, proton.SendStatsReq{MeasurementGroup: "proton.any.test"})
			req.EventInfo = append(req.EventInfo, proton.SendStatsReq{MeasurementGroup: "proton.any.test2"})
			err := c.SendDataEventMultiple(context.Background(), req)
			require.NoError(t, err)

			// send empty multiple data event.
			err = c.SendDataEventMultiple(context.Background(), proton.SendStatsMultiReq{})
			require.NoError(t, err)

			// Send bad multiple data event.
			var badReq proton.SendStatsMultiReq
			badReq.EventInfo = append(badReq.EventInfo, proton.SendStatsReq{})
			err = c.SendDataEventMultiple(context.Background(), badReq)
			require.Error(t, err)
		})
	})
}

func TestServer_GetMessageGroupCount(t *testing.T) {
	withServer(t, func(ctx context.Context, s *Server, m *proton.Manager) {
		withUser(ctx, t, s, m, "user", "pass", func(c *proton.Client) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			user, err := c.GetUser(ctx)
			require.NoError(t, err)

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)

			salt, err := c.GetSalts(ctx)
			require.NoError(t, err)

			pass, err := salt.SaltForKey([]byte("pass"), user.Keys.Primary().ID)
			require.NoError(t, err)

			_, addrKRs, err := proton.Unlock(user, addr, pass, async.NoopPanicHandler{})
			require.NoError(t, err)

			expected := []proton.MessageGroupCount{
				{
					LabelID: proton.InboxLabel,
					Total:   10,
					Unread:  4,
				},
				{
					LabelID: proton.SentLabel,
					Total:   4,
					Unread:  0,
				},
				{
					LabelID: proton.ArchiveLabel,
					Total:   3,
					Unread:  0,
				},
				{
					LabelID: proton.TrashLabel,
					Total:   6,
					Unread:  0,
				},
				{
					LabelID: proton.AllMailLabel,
					Total:   23,
					Unread:  4,
				},
			}

			for _, st := range expected {
				if st.LabelID == proton.AllMailLabel {
					continue
				}

				var flags proton.MessageFlag
				if st.LabelID == proton.InboxLabel {
					flags = proton.MessageFlagReceived
				} else if st.LabelID == proton.SentLabel {
					flags = proton.MessageFlagSent
				}

				res := importMessages(ctx, t, c, addr[0].ID, addrKRs[addr[0].ID], []string{}, flags, st.Total)
				msgIDs := xslices.Map(res, func(r proton.ImportRes) string {
					return r.MessageID
				})
				require.NoError(t, c.LabelMessages(ctx, msgIDs, st.LabelID))
				if st.Unread == 0 {
					require.NoError(t, c.MarkMessagesRead(ctx, msgIDs...))
				} else {
					require.NoError(t, c.MarkMessagesRead(ctx, msgIDs[st.Unread:]...))
				}
			}

			counts, err := c.GetGroupedMessageCount(ctx)
			require.NoError(t, err)

			counts = xslices.Filter(counts, func(t proton.MessageGroupCount) bool {
				switch t.LabelID {
				case proton.InboxLabel, proton.TrashLabel, proton.ArchiveLabel, proton.AllMailLabel, proton.SentLabel:
					return true
				default:
					return false
				}
			})
			require.NotEmpty(t, counts)
			require.ElementsMatch(t, expected, counts)

		})
	})
}

func withServer(t *testing.T, fn func(ctx context.Context, s *Server, m *proton.Manager), opts ...Option) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(opts...)
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithCookieJar(newTestCookieJar()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	fn(ctx, s, m)
}

func withUser(ctx context.Context, t *testing.T, s *Server, m *proton.Manager, user, pass string, fn func(c *proton.Client)) {
	_, _, err := s.CreateUser(user, []byte(pass))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, user, []byte(pass))
	require.NoError(t, err)
	defer c.Close()

	fn(c)
}

func withMessages(ctx context.Context, t *testing.T, c *proton.Client, pass string, count int, fn func([]string)) {
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

	fn(xslices.Map(importMessages(ctx, t, c, addr[0].ID, addrKRs[addr[0].ID], []string{}, proton.MessageFlagReceived, count), func(res proton.ImportRes) string {
		return res.MessageID
	}))
}

func importMessagesWithSubjectGenerator(
	ctx context.Context,
	t *testing.T,
	c *proton.Client,
	addrID string,
	addrKR *crypto.KeyRing,
	labelIDs []string,
	flags proton.MessageFlag,
	count int,
	subjectGenerator func() string,
) []proton.ImportRes {
	req := iterator.Collect(iterator.Map(iterator.Counter(count), func(int) proton.ImportReq {
		return proton.ImportReq{
			Metadata: proton.ImportMetadata{
				AddressID: addrID,
				LabelIDs:  labelIDs,
				Flags:     flags,
				Unread:    true,
			},
			Message: newMessageLiteralWithSubject("sender@example.com", "recipient@example.com", subjectGenerator()),
		}
	}))

	str, err := c.ImportMessages(ctx, addrKR, runtime.NumCPU(), runtime.NumCPU(), req...)
	require.NoError(t, err)

	res, err := stream.Collect(ctx, str)
	require.NoError(t, err)

	return res
}

func importMessages(
	ctx context.Context,
	t *testing.T,
	c *proton.Client,
	addrID string,
	addrKR *crypto.KeyRing,
	labelIDs []string,
	flags proton.MessageFlag,
	count int,
) []proton.ImportRes {
	return importMessagesWithSubjectGenerator(ctx, t, c, addrID, addrKR, labelIDs, flags, count, func() string {
		return uuid.NewString()
	})
}

func countBytesRead(ctl *proton.NetCtl, fn func()) uint64 {
	var read uint64

	ctl.OnRead(func(b []byte) {
		atomic.AddUint64(&read, uint64(len(b)))
	})

	fn()

	return read
}

type testCookieJar struct {
	cookies map[string][]*http.Cookie
	lock    sync.RWMutex
}

func newTestCookieJar() *testCookieJar {
	return &testCookieJar{
		cookies: make(map[string][]*http.Cookie),
	}
}

func (j *testCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.lock.Lock()
	defer j.lock.Unlock()

	j.cookies[u.Host] = cookies
}

func (j *testCookieJar) Cookies(u *url.URL) []*http.Cookie {
	j.lock.RLock()
	defer j.lock.RUnlock()

	return j.cookies[u.Host]
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

func elementsMatch[T comparable](want, got []T) bool {
	if len(want) != len(got) {
		return false
	}

	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}

	return true
}

func getFullMessages(ctx context.Context,
	c *proton.Client,
	workers, buffer int,
	messageIDs ...string) stream.Stream[proton.FullMessage] {
	scheduler := proton.NewSequentialScheduler()
	attachmentStorageProvider := proton.NewDefaultAttachmentAllocator()
	return parallel.MapStream(
		ctx,
		stream.FromIterator(iterator.Slice(messageIDs)),
		workers,
		buffer,
		func(ctx context.Context, messageID string) (proton.FullMessage, error) {
			return c.GetFullMessage(ctx, messageID, scheduler, attachmentStorageProvider)
		},
	)
}
