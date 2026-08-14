// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestOptionsConfigurationShape(t *testing.T) {
	t.Parallel()

	actual := configurationShape(reflect.TypeOf(cmd.Options{}), "")
	sort.Strings(actual)
	actualShape := strings.Join(actual, "\n") + "\n"

	if os.Getenv("UPDATE_GOLDEN") == "true" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile("testdata/options-shape.txt", []byte(actualShape), 0o644))
	}

	expected, err := os.ReadFile("testdata/options-shape.txt")
	require.NoError(t, err)
	require.Equal(t, string(expected), actualShape)
}

func configurationShape(typ reflect.Type, prefix string) []string {
	var result []string

	for fieldIndex := range typ.NumField() {
		field := typ.Field(fieldIndex)
		key := field.Tag.Get("koanf")
		if key == "" || key == "-" {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		switch {
		case fieldType == reflect.TypeFor[time.Duration]():
			result = append(result, path+" time.Duration")
		case fieldType.Kind() == reflect.Struct:
			result = append(result, path+" object")
			result = append(result, configurationShape(fieldType, path)...)
		case fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct:
			result = append(result, path+" []"+fieldType.Elem().Name())
			result = append(result, configurationShape(fieldType.Elem(), path+"[]")...)
		default:
			result = append(result, path+" "+field.Type.String())
		}
	}

	return result
}
