/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package module

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestIsBinaryManifest(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "clean yaml",
			data: []byte("apiVersion: v1\nkind: Secret\ndata:\n  cni: Y2lsaXVt\n"),
			want: false,
		},
		{
			name: "multibyte utf-8 is allowed",
			data: []byte("metadata:\n  name: тест\n"),
			want: false,
		},
		{
			name: "invalid leading utf-8 octet",
			// 0xf1 starts a 4-byte sequence but is followed by an invalid
			// continuation byte: this is exactly what b64dec of a random
			// alphanumeric string produces.
			data: []byte{'d', 'a', 't', 'a', ':', '\n', ' ', ' ', 0x68, 0x1d, 0xf1, 0x63, 0xdc, 0xca},
			want: true,
		},
		{
			name: "disallowed control character",
			data: []byte("data:\n  value\x00here\n"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isBinaryManifest(tt.data))
		})
	}
}

func TestSanitizeBinaryManifest(t *testing.T) {
	t.Run("preserves clean content", func(t *testing.T) {
		in := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: тест\n")
		assert.Equal(t, in, sanitizeBinaryManifest(in))
	})

	t.Run("replaces invalid utf-8 and keeps layout", func(t *testing.T) {
		// Mimics a rendered Secret whose data value came from b64dec of a
		// synthetic placeholder, yielding binary bytes.
		in := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: d8-cni-configuration\ndata:\n  ")
		in = append(in, 0x68, 0x1d, 0xf1, 0x63, 0xdc, 0xca)
		in = append(in, '\n')

		out := sanitizeBinaryManifest(in)

		require.True(t, utf8.Valid(out), "sanitized manifest must be valid UTF-8")

		node := map[string]any{}
		require.NoError(t, yaml.UnmarshalStrict(out, &node))
		assert.Equal(t, "Secret", node["kind"])
	})

	t.Run("replaces control characters", func(t *testing.T) {
		in := []byte("data:\n  value\x01\x02\x03\n")
		out := sanitizeBinaryManifest(in)

		require.True(t, utf8.Valid(out))

		node := map[string]any{}
		require.NoError(t, yaml.UnmarshalStrict(out, &node))
	})
}
