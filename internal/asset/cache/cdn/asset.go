// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cdn

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

type cdnAsset struct {
	underlying cache.RedirectableAsset
	host       string
	trimPrefix string
}

// Check interface.
var (
	_ cache.BootAsset         = (*cdnAsset)(nil)
	_ cache.RedirectableAsset = (*cdnAsset)(nil)
)

// Size returns the size of the boot asset.
func (a *cdnAsset) Size() int64 {
	return a.underlying.Size()
}

// Reader returns a reader for the boot asset.
func (a *cdnAsset) Reader() (io.ReadCloser, error) {
	return a.underlying.Reader()
}

func (a *cdnAsset) rewriteURL(ctx context.Context, filename string) (string, error) {
	raw, err := a.underlying.Redirect(ctx, filename)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if a.trimPrefix != "" {
		u.Path = strings.TrimPrefix(u.Path, a.trimPrefix)
	}

	if a.host != "" {
		u.Host = a.host
	}

	return u.String(), nil
}

// Redirect returns the URL for the boot asset.
func (a *cdnAsset) Redirect(ctx context.Context, filename string) (string, error) {
	return a.rewriteURL(ctx, filename)
}
