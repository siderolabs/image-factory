// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talosversion_test

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/talosversion"
)

func TestFeaturesFor(t *testing.T) {
	t.Parallel()

	old := talosversion.FeaturesFor(semver.MustParse("1.4.0"))
	assert.False(t, old.Has(talosversion.FeatureSecureBoot))
	assert.False(t, old.Has(talosversion.FeatureSBOM))

	current := talosversion.FeaturesFor(semver.MustParse("1.13.0"))
	assert.True(t, current.Has(talosversion.FeatureSecureBoot))
	assert.True(t, current.Has(talosversion.FeatureBootloaderSelection))
	assert.True(t, current.Has(talosversion.FeatureLocalTestCluster))
	assert.True(t, current.Has(talosversion.FeatureSBOM))
	assert.True(t, current.Has(talosversion.FeatureVEX))

	assert.True(t, talosversion.FeaturesFor(semver.MustParse("1.12.0-alpha.2")).Has(talosversion.FeatureBootloaderSelection))
	assert.False(t, talosversion.FeaturesFor(semver.MustParse("1.12.0-alpha.1")).Has(talosversion.FeatureBootloaderSelection))
}

func TestFeaturesJSON(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(talosversion.FeaturesFor(semver.MustParse("1.13.0")))
	require.NoError(t, err)
	assert.JSONEq(t,
		`["SecureBoot","BootloaderSelection","EmbeddedConfig","TalosctlDownload","LocalTestCluster","SBOM","VEX","ProductionGuide","TroubleshootingGuide"]`,
		string(encoded),
	)

	// A version with none of the features lists none, rather than null.
	encoded, err = json.Marshal(talosversion.FeaturesFor(semver.MustParse("0.1.0")))
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(encoded))
}

func TestPlatformsFilteredByVersion(t *testing.T) {
	t.Parallel()

	assert.Less(t, len(talosversion.CloudPlatforms(semver.MustParse("1.0.0"))), len(talosversion.CloudPlatforms(semver.MustParse("1.13.0"))))
	assert.Less(t, len(talosversion.Boards(semver.MustParse("1.7.0"))), len(talosversion.Boards(semver.MustParse("1.13.0"))))
}
