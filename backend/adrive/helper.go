package adrive

import (
	"context"
	"net/http"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

// Client contains the info for the Aliyun Drive API
type AdriveClient struct {
	mu           sync.RWMutex // Protecting read/writes
	c            *rest.Client // The REST client
	rootURL      string       // API root URL
	driveID      string       // Drive ID
	errorHandler func(resp *http.Response) error
	pacer        *fs.Pacer // To pace the API calls
}

// NewClient takes an http.Client and makes a new api instance
func NewAdriveClient(c *http.Client, rootURL string) *AdriveClient {
	client := &AdriveClient{
		c:       rest.NewClient(c),
		rootURL: rootURL,
	}
	client.c.SetErrorHandler(errorHandler)
	client.c.SetRoot(rootURL)

	// Create a pacer using rclone's default exponential backoff
	client.pacer = fs.NewPacer(
		context.Background(),
		pacer.NewDefault(
			pacer.MinSleep(minSleep),
			pacer.MaxSleep(maxSleep),
			pacer.DecayConstant(decayConstant),
		),
	)

	return client
}
