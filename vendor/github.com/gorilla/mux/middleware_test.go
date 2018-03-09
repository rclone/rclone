package mux

import (
	"bytes"
	"net/http"
	"testing"
)

type testMiddleware struct {
	timesCalled uint
}

func (tm *testMiddleware) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tm.timesCalled++
		h.ServeHTTP(w, r)
	})
}

func dummyHandler(w http.ResponseWriter, r *http.Request) {}

func TestMiddlewareAdd(t *testing.T) {
	router := NewRouter()
	router.HandleFunc("/", dummyHandler).Methods("GET")

	mw := &testMiddleware{}

	router.useInterface(mw)
	if len(router.middlewares) != 1 || router.middlewares[0] != mw {
		t.Fatal("Middleware was not added correctly")
	}

	router.Use(mw.Middleware)
	if len(router.middlewares) != 2 {
		t.Fatal("MiddlewareFunc method was not added correctly")
	}

	banalMw := func(handler http.Handler) http.Handler {
		return handler
	}
	router.Use(banalMw)
	if len(router.middlewares) != 3 {
		t.Fatal("MiddlewareFunc method was not added correctly")
	}
}

func TestMiddleware(t *testing.T) {
	router := NewRouter()
	router.HandleFunc("/", dummyHandler).Methods("GET")

	mw := &testMiddleware{}
	router.useInterface(mw)

	rw := NewRecorder()
	req := newRequest("GET", "/")

	// Test regular middleware call
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 1 {
		t.Fatalf("Expected %d calls, but got only %d", 1, mw.timesCalled)
	}

	// Middleware should not be called for 404
	req = newRequest("GET", "/not/found")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 1 {
		t.Fatalf("Expected %d calls, but got only %d", 1, mw.timesCalled)
	}

	// Middleware should not be called if there is a method mismatch
	req = newRequest("POST", "/")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 1 {
		t.Fatalf("Expected %d calls, but got only %d", 1, mw.timesCalled)
	}

	// Add the middleware again as function
	router.Use(mw.Middleware)
	req = newRequest("GET", "/")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 3 {
		t.Fatalf("Expected %d calls, but got only %d", 3, mw.timesCalled)
	}

}

func TestMiddlewareSubrouter(t *testing.T) {
	router := NewRouter()
	router.HandleFunc("/", dummyHandler).Methods("GET")

	subrouter := router.PathPrefix("/sub").Subrouter()
	subrouter.HandleFunc("/x", dummyHandler).Methods("GET")

	mw := &testMiddleware{}
	subrouter.useInterface(mw)

	rw := NewRecorder()
	req := newRequest("GET", "/")

	router.ServeHTTP(rw, req)
	if mw.timesCalled != 0 {
		t.Fatalf("Expected %d calls, but got only %d", 0, mw.timesCalled)
	}

	req = newRequest("GET", "/sub/")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 0 {
		t.Fatalf("Expected %d calls, but got only %d", 0, mw.timesCalled)
	}

	req = newRequest("GET", "/sub/x")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 1 {
		t.Fatalf("Expected %d calls, but got only %d", 1, mw.timesCalled)
	}

	req = newRequest("GET", "/sub/not/found")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 1 {
		t.Fatalf("Expected %d calls, but got only %d", 1, mw.timesCalled)
	}

	router.useInterface(mw)

	req = newRequest("GET", "/")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 2 {
		t.Fatalf("Expected %d calls, but got only %d", 2, mw.timesCalled)
	}

	req = newRequest("GET", "/sub/x")
	router.ServeHTTP(rw, req)
	if mw.timesCalled != 4 {
		t.Fatalf("Expected %d calls, but got only %d", 4, mw.timesCalled)
	}
}

func TestMiddlewareExecution(t *testing.T) {
	mwStr := []byte("Middleware\n")
	handlerStr := []byte("Logic\n")

	router := NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	})

	rw := NewRecorder()
	req := newRequest("GET", "/")

	// Test handler-only call
	router.ServeHTTP(rw, req)

	if bytes.Compare(rw.Body.Bytes(), handlerStr) != 0 {
		t.Fatal("Handler response is not what it should be")
	}

	// Test middleware call
	rw = NewRecorder()

	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(mwStr)
			h.ServeHTTP(w, r)
		})
	})

	router.ServeHTTP(rw, req)
	if bytes.Compare(rw.Body.Bytes(), append(mwStr, handlerStr...)) != 0 {
		t.Fatal("Middleware + handler response is not what it should be")
	}
}

func TestMiddlewareNotFound(t *testing.T) {
	mwStr := []byte("Middleware\n")
	handlerStr := []byte("Logic\n")

	router := NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	})
	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(mwStr)
			h.ServeHTTP(w, r)
		})
	})

	// Test not found call with default handler
	rw := NewRecorder()
	req := newRequest("GET", "/notfound")

	router.ServeHTTP(rw, req)
	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a 404")
	}

	// Test not found call with custom handler
	rw = NewRecorder()
	req = newRequest("GET", "/notfound")

	router.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("Custom 404 handler"))
	})
	router.ServeHTTP(rw, req)

	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a custom 404")
	}
}

func TestMiddlewareMethodMismatch(t *testing.T) {
	mwStr := []byte("Middleware\n")
	handlerStr := []byte("Logic\n")

	router := NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	}).Methods("GET")

	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(mwStr)
			h.ServeHTTP(w, r)
		})
	})

	// Test method mismatch
	rw := NewRecorder()
	req := newRequest("POST", "/")

	router.ServeHTTP(rw, req)
	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a method mismatch")
	}

	// Test not found call
	rw = NewRecorder()
	req = newRequest("POST", "/")

	router.MethodNotAllowedHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("Method not allowed"))
	})
	router.ServeHTTP(rw, req)

	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a method mismatch")
	}
}

func TestMiddlewareNotFoundSubrouter(t *testing.T) {
	mwStr := []byte("Middleware\n")
	handlerStr := []byte("Logic\n")

	router := NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	})

	subrouter := router.PathPrefix("/sub/").Subrouter()
	subrouter.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	})

	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(mwStr)
			h.ServeHTTP(w, r)
		})
	})

	// Test not found call for default handler
	rw := NewRecorder()
	req := newRequest("GET", "/sub/notfound")

	router.ServeHTTP(rw, req)
	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a 404")
	}

	// Test not found call with custom handler
	rw = NewRecorder()
	req = newRequest("GET", "/sub/notfound")

	subrouter.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("Custom 404 handler"))
	})
	router.ServeHTTP(rw, req)

	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a custom 404")
	}
}

func TestMiddlewareMethodMismatchSubrouter(t *testing.T) {
	mwStr := []byte("Middleware\n")
	handlerStr := []byte("Logic\n")

	router := NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	})

	subrouter := router.PathPrefix("/sub/").Subrouter()
	subrouter.HandleFunc("/", func(w http.ResponseWriter, e *http.Request) {
		w.Write(handlerStr)
	}).Methods("GET")

	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(mwStr)
			h.ServeHTTP(w, r)
		})
	})

	// Test method mismatch without custom handler
	rw := NewRecorder()
	req := newRequest("POST", "/sub/")

	router.ServeHTTP(rw, req)
	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a method mismatch")
	}

	// Test method mismatch with custom handler
	rw = NewRecorder()
	req = newRequest("POST", "/sub/")

	router.MethodNotAllowedHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("Method not allowed"))
	})
	router.ServeHTTP(rw, req)

	if bytes.Contains(rw.Body.Bytes(), mwStr) {
		t.Fatal("Middleware was called for a method mismatch")
	}
}
