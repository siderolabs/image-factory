// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"io"

	"github.com/google/go-containerregistry/pkg/name"
)

// ExtractExtensionList exposes extractExtensionList for testing.
func ExtractExtensionList(r io.Reader, registry name.Registry, namespace string) ([]ExtensionRef, error) {
	return extractExtensionList(r, registryWithNamespace{registry: registry, namespace: namespace})
}

// PullReference exposes the extension pull reference for testing.
func (e ExtensionRef) PullReference() name.Tag {
	return e.pullReference
}

// RepoWithNamespace exposes registryWithNamespace.Repo for testing.
func RepoWithNamespace(registry name.Registry, namespace, repoPath string) name.Repository {
	return registryWithNamespace{registry: registry, namespace: namespace}.Repo(repoPath)
}
