// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package refresher_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/internal/remotewrap/internal/refresher"
)

func TestRefresher(t *testing.T) {
	t.Parallel()

	var counter int32

	r := refresher.New(func() (int32, error) {
		counter++

		if counter > 2 {
			return 0, errors.New("too big")
		}

		return counter, nil
	}, time.Second)

	v, err := r.Get()
	require.NoError(t, err)
	assert.Equal(t, int32(1), v)

	v, err = r.Get()
	require.NoError(t, err)
	assert.Equal(t, int32(1), v)

	time.Sleep(time.Second + time.Millisecond)

	v, err = r.Get()
	require.NoError(t, err)
	assert.Equal(t, int32(2), v)

	var eg errgroup.Group

	for range 10 {
		eg.Go(func() error {
			vv, verr := r.Get()
			if verr != nil {
				return verr
			}

			if vv != 2 {
				return errors.New("expected 2, got " + strconv.Itoa(int(vv)))
			}

			return nil
		})
	}

	require.NoError(t, eg.Wait())

	time.Sleep(time.Second + time.Millisecond)

	_, err = r.Get()
	require.Error(t, err)
	require.EqualError(t, err, "too big")
}
