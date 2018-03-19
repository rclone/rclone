package src

type apiRequest interface {
	Request() *HTTPRequest
}
