// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Error tags shared by HTTP dispatch and rendering.
type (
	RouteNotFoundTag    struct{}
	MethodNotAllowedTag struct{}
	InvalidRequestTag   struct{}
	InvalidImageTag     = registry.InvalidImageTag
	ProxyUnavailableTag = registry.ProxyUnavailableTag
)

// ErrorClassification describes logging and response behavior for a handler error.
type ErrorClassification struct {
	Message string
	Status  int
	Level   zapcore.Level
	Render  bool
}

// ClassifyError maps existing domain error tags to their HTTP and logging contract.
func ClassifyError(err error) ErrorClassification {
	classification := ErrorClassification{Status: http.StatusOK, Level: zapcore.InfoLevel}

	switch {
	case err == nil:
	case xerrors.TagIs[enterrors.RespondedTag](err):
		classification.Level = zapcore.WarnLevel
	case xerrors.TagIs[enterrors.NotEnabledTag](err):
		classification = rendered(err.Error(), http.StatusPaymentRequired)
	case xerrors.TagIs[enterrors.NotReadyTag](err):
		classification = rendered("service temporarily unavailable", http.StatusServiceUnavailable)
	case xerrors.TagIs[ProxyUnavailableTag](err):
		classification = rendered(err.Error(), http.StatusServiceUnavailable)
	case xerrors.TagIs[storage.ErrNotFoundTag](err),
		xerrors.TagIs[artifacts.ErrNotFoundTag](err),
		xerrors.TagIs[schematicpkg.NotFoundTag](err),
		xerrors.TagIs[registry.NotFoundTag](err),
		xerrors.TagIs[RouteNotFoundTag](err):
		classification = rendered(err.Error(), http.StatusNotFound)
	case xerrors.TagIs[MethodNotAllowedTag](err):
		classification = rendered(err.Error(), http.StatusMethodNotAllowed)
	case xerrors.TagIs[profile.InvalidErrorTag](err),
		xerrors.TagIs[schematicpkg.InvalidErrorTag](err),
		xerrors.TagIs[enterrors.InvalidErrorTag](err),
		xerrors.TagIs[InvalidImageTag](err),
		xerrors.TagIs[InvalidRequestTag](err):
		classification = rendered(err.Error(), http.StatusBadRequest)
	case xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err):
		classification = rendered("authentication required to access this schematic", http.StatusUnauthorized)
	case xerrors.TagIs[schematicpkg.ForbiddenTag](err):
		classification = rendered("access denied", http.StatusForbidden)
	case errors.Is(err, context.Canceled):
		classification.Status = 499
	default:
		classification = ErrorClassification{
			Message: "internal server error",
			Status:  http.StatusInternalServerError,
			Level:   zapcore.ErrorLevel,
			Render:  true,
		}
	}

	return classification
}

func rendered(message string, status int) ErrorClassification {
	return ErrorClassification{Message: message, Status: status, Level: zapcore.WarnLevel, Render: true}
}

// RenderError writes a classified error without replacing handler-owned responses.
func RenderError(writer http.ResponseWriter, request *http.Request, protocol Protocol, classification ErrorClassification) {
	if !classification.Render {
		return
	}

	if classification.Status == http.StatusUnauthorized && protocol != ProtocolBrowserAuth &&
		writer.Header().Get("WWW-Authenticate") == "" && writer.Header().Get("Hx-Redirect") == "" {
		writer.Header().Set("WWW-Authenticate", `Basic realm="Image Factory Enterprise", charset="UTF-8"`)
	}

	if request.Method == http.MethodHead {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Content-Length", strconv.Itoa(len(classification.Message)+1))
		writer.WriteHeader(classification.Status)

		return
	}

	http.Error(writer, classification.Message, classification.Status)
}
