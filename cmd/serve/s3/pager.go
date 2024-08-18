// Package s3 implements a fake s3 server for rclone
package s3

import (
	"sort"

	"github.com/rclone/gofakes3"
)

// pager splits the object list into smulitply pages.
func (db *s3Backend) pager(list *gofakes3.ObjectList, page gofakes3.ListBucketPage) (*gofakes3.ObjectList, error) {
	// sort by alphabet
	sort.Slice(list.CommonPrefixes, func(i, j int) bool {
		return list.CommonPrefixes[i].Prefix < list.CommonPrefixes[j].Prefix
	})
	// sort by modtime
	sort.Slice(list.Contents, func(i, j int) bool {
		return list.Contents[i].LastModified.Before(list.Contents[j].LastModified.Time)
	})
	tokens := page.MaxKeys
	if tokens == 0 {
		tokens = 1000
	}
	if page.HasMarker {
		for i, obj := range list.Contents {
			if obj.Key == page.Marker {
				list.Contents = list.Contents[i+1:]
				break
			}
		}
		for i, obj := range list.CommonPrefixes {
			if obj.Prefix == page.Marker {
				list.CommonPrefixes = list.CommonPrefixes[i+1:]
				break
			}
		}
	}

	response := gofakes3.NewObjectList()
	for _, obj := range list.CommonPrefixes {
		if tokens <= 0 {
			break
		}
		response.AddPrefix(obj.Prefix)
		tokens--
	}

	for _, obj := range list.Contents {
		if tokens <= 0 {
			break
		}
		response.Add(obj)
		tokens--
	}

	if len(list.CommonPrefixes)+len(list.Contents) > int(page.MaxKeys) {
		response.IsTruncated = true
		if len(response.Contents) > 0 {
			response.NextMarker = response.Contents[len(response.Contents)-1].Key
		} else {
			response.NextMarker = response.CommonPrefixes[len(response.CommonPrefixes)-1].Prefix
		}
	}

	return response, nil
}
