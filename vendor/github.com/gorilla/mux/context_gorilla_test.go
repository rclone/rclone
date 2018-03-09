// +build !go1.7

package mux

import (
	"net/http"
	"testing"

	"github.com/gorilla/context"
)

// Tests that the context is cleared or not cleared properly depending on
// the configuration of the router
func TestKeepContext(t *testing.T) {
	func1 := func(w http.ResponseWriter, r *http.Request) {}

	r := NewRouter()
	r.HandleFunc("/", func1).Name("func1")

	req, _ := http.NewRequest("GET", "http://localhost/", nil)
	context.Set(req, "t", 1)

	res := new(http.ResponseWriter)
	r.ServeHTTP(*res, req)

	if _, ok := context.GetOk(req, "t"); ok {
		t.Error("Context should have been cleared at end of request")
	}

	r.KeepContext = true

	req, _ = http.NewRequest("GET", "http://localhost/", nil)
	context.Set(req, "t", 1)

	r.ServeHTTP(*res, req)
	if _, ok := context.GetOk(req, "t"); !ok {
		t.Error("Context should NOT have been cleared at end of request")
	}

}
