package acd

import (
	"encoding/json"
	"io"
	"net/http"
)

// ChangesService provides access to incemental changes in the Amazon Cloud Drive API.
//
// See: https://developer.amazon.com/public/apis/experience/cloud-drive/content/changes
type ChangesService struct {
	client *Client
}

// A ChangeSet is collection of node changes as received from the Changes API
type ChangeSet struct {
	Checkpoint string  `json:"checkpoint"`
	Nodes      []*Node `json:"nodes"`
	Reset      bool    `json:"reset"`
	StatusCode int     `json:"statusCode"`
	End        bool    `json:"end"`
}

// ChangesOptions contains all possible arguments for the Changes API
type ChangesOptions struct {
	Checkpoint    string `json:"checkpoint,omitempty"`
	ChunkSize     int    `json:"chunkSize,omitempty"`
	MaxNodes      int    `json:"maxNodes,omitempty"`
	IncludePurged bool   `json:"includePurged,omitempty,string"`
}

// GetChanges returns all the changes since opts.Checkpoint
func (s *ChangesService) GetChanges(opts *ChangesOptions) ([]*ChangeSet, *http.Response, error) {
	var changeSets []*ChangeSet
	resp, err := s.GetChangesFunc(opts, func(cs *ChangeSet, err error) error {
		if err != nil {
			return err
		}
		changeSets = append(changeSets, cs)
		return nil
	})
	return changeSets, resp, err
}

// GetChangesChan gets all the changes since opts.Checkpoint sending each ChangeSet to the channel.
// The provided channel is closed before returning
func (s *ChangesService) GetChangesChan(opts *ChangesOptions, ch chan<- *ChangeSet) (*http.Response, error) {
	defer close(ch)

	return s.GetChangesFunc(opts, func(cs *ChangeSet, err error) error {
		if err != nil {
			return err
		}
		ch <- cs
		return nil
	})
}

// GetChangesFunc gets all the changes since opts.Checkpoint and calls f with the ChangeSet or the error received.
// If f returns a non nil value, GetChangesFunc exits and returns the given error.
func (s *ChangesService) GetChangesFunc(opts *ChangesOptions, f func(*ChangeSet, error) error) (*http.Response, error) {
	req, err := s.client.NewMetadataRequest("POST", "changes", opts)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		changeSet := &ChangeSet{}
		err := decoder.Decode(&changeSet)
		if err == io.EOF {
			return resp, nil
		}
		err = f(changeSet, err)
		if err != nil {
			return resp, err
		}
	}
}
