// Package enterprise provide glue to Enterprise code.
package enterprise

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// AuthProvider defines an authentication provider.
type AuthProvider interface {
	Middleware(func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error
}
