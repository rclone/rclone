package src

//ResourceInfoResponse struct is returned by the API for metedata requests.
type ResourceInfoResponse struct {
	PublicKey        string                 `json:"public_key"`
	Name             string                 `json:"name"`
	Created          string                 `json:"created"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
	Preview          string                 `json:"preview"`
	PublicURL        string                 `json:"public_url"`
	OriginPath       string                 `json:"origin_path"`
	Modified         string                 `json:"modified"`
	Path             string                 `json:"path"`
	Md5              string                 `json:"md5"`
	ResourceType     string                 `json:"type"`
	MimeType         string                 `json:"mime_type"`
	Size             uint64                 `json:"size"`
	Embedded         *ResourceListResponse  `json:"_embedded"`
}
