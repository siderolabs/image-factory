// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/frontend/http/authentication"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/version"
)

// RequestMiddleware owns request validation, authentication, response observation and audit.
// It has no access to endpoint services or frontend construction dependencies.
type RequestMiddleware struct {
	logger         *zap.Logger
	contract       *api.Contract
	authentication *authentication.Selector
	auditSink      audit.Sink
	authEnabled    bool
}

// NewRequestMiddleware configures the common request pipeline independently of endpoint composition.
func NewRequestMiddleware(logger *zap.Logger, contract *api.Contract, provider authentication.Provider, verifier authentication.TokenVerifier, sink audit.Sink) *RequestMiddleware {
	return &RequestMiddleware{
		logger:         logger,
		contract:       contract,
		authentication: authentication.New(logger, provider, verifier),
		auditSink:      sink,
		authEnabled:    provider != nil,
	}
}

// Build applies the route's declared access and protocol policies.
func (m *RequestMiddleware) Build(route transport.Route) (httprouter.Handle, error) {
	switch route.Access {
	case transport.AccessPublic, transport.AccessAuthenticated, transport.AccessImageDownload:
		return m.Wrap(route.Handler, route.Access, route.Protocol), nil
	default:
		return nil, fmt.Errorf("route %s %s has unsupported access policy %d", route.Method, route.Path, route.Access)
	}
}

// Wrap applies the request pipeline to an endpoint with explicit access and protocol policies.
func (m *RequestMiddleware) Wrap(h transport.Handler, access transport.AccessPolicy, protocol transport.Protocol) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		requestID := requestIDFrom(r)
		ctx := ctxlog.WithRequestID(r.Context(), requestID)
		logger := ctxlog.Logger(ctx, m.logger)

		var state transport.ResponseState

		sw := transport.ObserveResponse(w, &state)
		sw.Header().Set("Server", version.ServerString())
		sw.Header().Set(RequestIDHeader, requestID)

		handler := m.withContractValidation(m.withAuth(h, access, &state))
		start := time.Now()
		err := handler(ctx, sw, r, p)
		duration := time.Since(start)

		classification := transport.ClassifyError(err)
		if state.Status() == 0 {
			transport.RenderError(sw, r, protocol, classification)
		}

		status := classification.Status
		if state.Status() != 0 {
			status = state.Status()
		} else {
			// Nothing was committed, so net/http sends an implicit 200 that reaches no hook.
			state.ApplyCacheControlPin(sw)
		}

		logger.Log(classification.Level, "request",
			zap.String("method", r.Method), zap.String("path", r.URL.Path),
			zap.Int("status", status), zap.Duration("duration", duration), zap.Error(err),
		)

		// Audit projects the authoritative principal after authentication; this is
		// not a separate identity captured by the authentication middleware.
		if access != transport.AccessPublic {
			username := ""
			if principal, ok := authn.PrincipalFromContext(r.Context()); ok { //nolint:contextcheck // Authentication middleware publishes its derived context to the request.
				username = principal.Username()
			}

			m.audit(ctx, logger, audit.Record{
				Time: start, RequestID: requestID, Username: username,
				ClientIP: r.RemoteAddr, Method: r.Method, Path: r.URL.Path,
				Status: status, Duration: duration, Error: errString(err),
			})
		}
	}
}

func (m *RequestMiddleware) withContractValidation(h transport.Handler) transport.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		if m.contract != nil {
			_, _, err := m.contract.ValidateRequest(ctx, r)
			if errors.Is(err, routers.ErrPathNotFound) {
				return xerrors.NewTagged[transport.RouteNotFoundTag](fmt.Errorf("route not registered in OpenAPI: %w", err))
			}

			if errors.Is(err, routers.ErrMethodNotAllowed) {
				return xerrors.NewTagged[transport.MethodNotAllowedTag](fmt.Errorf("method not registered in OpenAPI: %w", err))
			}

			if err != nil {
				return xerrors.NewTagged[transport.InvalidRequestTag](fmt.Errorf("invalid request: %w", err))
			}
		}

		return h(ctx, w, r, p)
	}
}

func requestIDFrom(r *http.Request) string {
	if id := r.Header.Get(RequestIDHeader); id != "" {
		return id
	}

	return uuid.NewString()
}

func (m *RequestMiddleware) withAuth(h transport.Handler, access transport.AccessPolicy, state *transport.ResponseState) transport.Handler {
	if access == transport.AccessPublic || !m.authEnabled {
		return h
	}

	authenticated := m.authentication.Middleware(access, h)

	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		w.Header().Set("Cache-Control", "no-store")
		state.PinCacheControl(w)

		return authenticated(ctx, w, r, p)
	}
}

// audit failures are logged but never fail the request.
func (m *RequestMiddleware) audit(ctx context.Context, logger *zap.Logger, record audit.Record) {
	if m.auditSink == nil {
		return
	}

	if err := m.auditSink.Log(ctx, record); err != nil {
		logger.Error("failed to write audit record", zap.Error(err))
	}
}

func errString(err error) string {
	if err != nil {
		return err.Error()
	}

	return ""
}
