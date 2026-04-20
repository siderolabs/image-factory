// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadHTPasswd loads an htpasswd file and returns a map of username to bcrypt password hashes.
//
// The htpasswd file format is one entry per line: `username:bcrypt_hash`.
// Multiple lines with the same username are allowed, enabling multiple API keys per user.
// Blank lines and lines starting with `#` are ignored.
// Only bcrypt hashes ($2y$, $2a$, $2b$) are supported.
func LoadHTPasswd(filePath string) (map[string][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	users := make(map[string][]string)

	scanner := bufio.NewScanner(f)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		username, hash, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("htpasswd line %d: missing ':' separator", lineNo)
		}

		username = strings.TrimSpace(username)
		hash = strings.TrimSpace(hash)

		if username == "" {
			return nil, fmt.Errorf("htpasswd line %d: empty username", lineNo)
		}

		if !strings.HasPrefix(hash, "$2y$") && !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			return nil, fmt.Errorf("htpasswd line %d: unsupported hash format (only bcrypt $2y$/$2a$/$2b$ is supported)", lineNo)
		}

		users[username] = append(users[username], hash)
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
