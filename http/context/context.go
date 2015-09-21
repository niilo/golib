package context

import (
	"net/http"

	"golang.org/x/net/context"
)

// The Handler interface is implemented by HTTP Servers
// that cary also context.Context
type Handler interface {
	ServeHTTPContext(http.ResponseWriter, *http.Request, context.Context)
}

// The HandlerFunc type is an context.Context enabled adapter to allow the use of
// ordinary functions as context enabled HTTP handlers.  If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler object that calls f.
type HandlerFunc func(http.ResponseWriter, *http.Request, context.Context)

// ServeHTTPContext calls f(w, r, ctx)
func (f HandlerFunc) ServeHTTPContext(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	f(w, r, ctx)
}

// The Adapter type is an adapter to adapt Handler
type Adapter func(Handler) Handler

// Adapt calls all Adapters with Handler
// last Adapter is called first
func Adapt(ch Handler, adapters ...Adapter) Handler {
	for _, adapter := range adapters {
		ch = adapter(ch)
	}
	return ch
}

// A HandlerContext holds context.Context with Handler
type ContextHandler struct {
	Context context.Context
	Handler
}

// ServeHTTP implements http.HandlerFunc for HandlerContext
func (c *ContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.Handler.ServeHTTPContext(w, r, c.Context)
}
