// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// TalosctlSource retrieves talosctl release metadata and files.
type TalosctlSource interface {
	GetTalosctlTuples(ctx context.Context, version string) ([]TalosctlTuple, error)
	GetTalosctlImage(ctx context.Context, version string) (string, error)
}

// TalosctlDownload is an opened talosctl binary.
type TalosctlDownload struct {
	Reader   io.ReadCloser
	Version  string
	Filename string
	Size     int64
}

// TalosctlService owns talosctl release lookup and file access.
type TalosctlService struct {
	source TalosctlSource
}

// NewTalosctlService creates a talosctl application service.
func NewTalosctlService(source TalosctlSource) *TalosctlService {
	return &TalosctlService{source: source}
}

// List returns the normalized version and available talosctl filenames.
func (service *TalosctlService) List(ctx context.Context, version string) (string, []string, error) {
	version = normalizeVersionTag(version)

	tuples, err := service.source.GetTalosctlTuples(ctx, version)
	if err != nil {
		return "", nil, err
	}

	filenames := make([]string, 0, len(tuples))
	for _, tuple := range tuples {
		filenames = append(filenames, fmt.Sprintf("talosctl-%s-%s%s", tuple.OS, tuple.Arch, tuple.Ext))
	}

	slices.Sort(filenames)

	return version, filenames, nil
}

// Open opens one talosctl binary for delivery.
func (service *TalosctlService) Open(ctx context.Context, version, filename string, withBody bool) (TalosctlDownload, error) {
	version = normalizeVersionTag(version)

	directory, err := service.source.GetTalosctlImage(ctx, version)
	if err != nil {
		return TalosctlDownload{}, err
	}

	path := filepath.Join(directory, filename)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return TalosctlDownload{}, err
	}

	download := TalosctlDownload{
		Size:     fileInfo.Size(),
		Version:  version,
		Filename: filename,
	}
	if !withBody {
		return download, nil
	}

	reader, err := os.Open(path)
	if err != nil {
		return TalosctlDownload{}, err
	}

	download.Reader = reader

	return download, nil
}

func normalizeVersionTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}

	return "v" + version
}
