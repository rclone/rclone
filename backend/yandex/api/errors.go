package src

import "encoding/json"

//DiskClientError struct
type DiskClientError struct {
	Description string `json:"Description"`
	Code        string `json:"Error"`
}

func (e DiskClientError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}
