// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package schematic_test

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	schematicfactory "github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	pkgschematic "github.com/siderolabs/image-factory/pkg/schematic"
)

type blockingHeadStorage struct {
	headStarted chan struct{}
	getStarted  chan struct{}
	releaseHead chan struct{}
	data        []byte
}

func (s *blockingHeadStorage) Head(ctx context.Context, _ string) error {
	close(s.headStarted)

	select {
	case <-s.releaseHead:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *blockingHeadStorage) Get(context.Context, string) ([]byte, error) {
	close(s.getStarted)

	return s.data, nil
}

func (s *blockingHeadStorage) Put(context.Context, string, []byte) error {
	return nil
}

func (s *blockingHeadStorage) Describe(chan<- *prometheus.Desc) {}

func (s *blockingHeadStorage) Collect(chan<- prometheus.Metric) {}

type blockingMissingGetStorage struct {
	getStarted chan struct{}
	releaseGet chan struct{}
	putStarted chan struct{}
}

func (s *blockingMissingGetStorage) Head(context.Context, string) error {
	return xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic not found")
}

func (s *blockingMissingGetStorage) Get(ctx context.Context, _ string) ([]byte, error) {
	close(s.getStarted)

	select {
	case <-s.releaseGet:
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic not found")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *blockingMissingGetStorage) Put(context.Context, string, []byte) error {
	close(s.putStarted)

	return nil
}

func (s *blockingMissingGetStorage) Describe(chan<- *prometheus.Desc) {}

func (s *blockingMissingGetStorage) Collect(chan<- prometheus.Metric) {}

func TestGetWaitsForConcurrentPut(t *testing.T) {
	t.Parallel()

	cfg := &pkgschematic.Schematic{
		Customization: pkgschematic.Customization{
			ExtraKernelArgs: []string{"console=ttyS0"},
		},
	}

	data, err := cfg.Marshal()
	require.NoError(t, err)

	storage := &blockingHeadStorage{
		data:        data,
		headStarted: make(chan struct{}),
		getStarted:  make(chan struct{}),
		releaseHead: make(chan struct{}),
	}

	factory := schematicfactory.NewFactory(zaptest.NewLogger(t), storage, schematicfactory.Options{})

	putDone := make(chan error, 1)

	go func() {
		_, putErr := factory.Put(t.Context(), cfg)
		putDone <- putErr
	}()

	select {
	case <-storage.headStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Put to reach storage.Head")
	}

	type getResult struct {
		schematic *pkgschematic.Schematic
		err       error
	}

	getDone := make(chan getResult, 1)
	getCalling := make(chan struct{})

	go func() {
		close(getCalling)

		schematic, getErr := factory.Get(t.Context(), mustSchematicID(t, cfg), nil)
		getDone <- getResult{schematic: schematic, err: getErr}
	}()

	<-getCalling

	select {
	case <-storage.getStarted:
		t.Fatal("Get reached storage before the in-flight Put completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(storage.releaseHead)

	require.NoError(t, <-putDone)

	result := <-getDone

	require.NoError(t, result.err)
	require.Equal(t, cfg, result.schematic)
}

func TestPutRetriesAfterConcurrentGetMiss(t *testing.T) {
	t.Parallel()

	cfg := &pkgschematic.Schematic{
		Customization: pkgschematic.Customization{
			ExtraKernelArgs: []string{"console=ttyS0"},
		},
	}

	storage := &blockingMissingGetStorage{
		getStarted: make(chan struct{}),
		releaseGet: make(chan struct{}),
		putStarted: make(chan struct{}),
	}

	factory := schematicfactory.NewFactory(zaptest.NewLogger(t), storage, schematicfactory.Options{})
	id := mustSchematicID(t, cfg)

	getDone := make(chan error, 1)

	go func() {
		_, err := factory.Get(t.Context(), id, nil)
		getDone <- err
	}()

	select {
	case <-storage.getStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Get to reach storage")
	}

	putDone := make(chan error, 1)

	go func() {
		_, err := factory.Put(t.Context(), cfg)
		putDone <- err
	}()

	select {
	case <-storage.putStarted:
		t.Fatal("Put reached storage before the in-flight Get completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(storage.releaseGet)

	require.Error(t, <-getDone)
	require.NoError(t, <-putDone)

	select {
	case <-storage.putStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Put to retry after the Get miss")
	}
}

func mustSchematicID(t *testing.T, cfg *pkgschematic.Schematic) string {
	t.Helper()

	id, err := cfg.ID()
	require.NoError(t, err)

	return id
}
