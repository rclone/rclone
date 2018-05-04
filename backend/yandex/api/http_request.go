package src

// HTTPRequest struct
type HTTPRequest struct {
	Method     string
	Path       string
	Parameters map[string]interface{}
	Headers    map[string][]string
}

func createGetRequest(client *Client, path string, params map[string]interface{}) *HTTPRequest {
	return createRequest(client, "GET", path, params)
}

func createRequest(client *Client, method string, path string, parameters map[string]interface{}) *HTTPRequest {
	var headers = make(map[string][]string)
	headers["Authorization"] = []string{"OAuth " + client.token}
	return &HTTPRequest{
		Method:     method,
		Path:       path,
		Parameters: parameters,
		Headers:    headers,
	}
}
