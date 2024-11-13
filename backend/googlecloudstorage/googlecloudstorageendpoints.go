package googlecloudstorage

import (
	"fmt"

	"github.com/rclone/rclone/fs"
)

type endpointRecord struct {
	endpointURL, Location, Description string
}

var endpoints = []endpointRecord{
	{"https://storage.europe-west3.rep.googleapis.com/storage/v1/", "europe-west3", "Frankfurt"},
	{"https://storage.europe-west8.rep.googleapis.com/storage/v1/", "europe-west8", "Milan"},
	{"https://storage.europe-west9.rep.googleapis.com/storage/v1/", "europe-west9", "Paris"},
	{"https://storage.me-central2.rep.googleapis.com/storage/v1/", "me-central2", "Doha"},
	{"https://storage.us-central1.rep.googleapis.com/storage/v1/", "us-central1", "Iowa"},
	{"https://storage.us-east1.rep.googleapis.com/storage/v1/", "us-east1", "South Carolina"},
	{"https://storage.us-east4.rep.googleapis.com/storage/v1/", "us-east4", "Northern Virgina"},
	{"https://storage.us-east5.rep.googleapis.com/storage/v1/", "us-east5", "Columbus"},
	{"https://storage.us-south1.rep.googleapis.com/storage/v1/", "us-south1", "Dallas"},
	{"https://storage.us-west1.rep.googleapis.com/storage/v1/", "us-west1", "Oregon"},
	{"https://storage.us-west2.rep.googleapis.com/storage/v1/", "us-west2", "Los Angeles"},
	{"https://storage.us-west3.rep.googleapis.com/storage/v1/", "us-west3", "Salt Lake City"},
	{"https://storage.us-west4.rep.googleapis.com/storage/v1/", "us-west4", "Las Vegas"},
}

func getHelpForEndpoints() (r []fs.OptionExample) {
	r = make([]fs.OptionExample, 0, len(endpoints))
	for _, ep := range endpoints {
		r = append(r, fs.OptionExample{Value: ep.endpointURL, Help: fmt.Sprintf("%s : %s", ep.Location, ep.Description)})
	}
	return r
}
