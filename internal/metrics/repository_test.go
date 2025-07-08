/*
Copyright 2025 Flant JSC

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

package metrics

import (
	"testing"
)

func Test_convertToHTTPS(t *testing.T) {
	type args struct {
		repoURL string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "SSH to HTTPS",
			args: args{
				repoURL: "git@github.com/deckhouse/csi-nfs",
			},
			want: "https://github.com/deckhouse/csi-nfs",
		},
		{
			name: "HTTPS to HTTPS",
			args: args{
				repoURL: "https://github.com/deckhouse/csi-nfs",
			},
			want: "https://github.com/deckhouse/csi-nfs",
		},
		{
			name: "HTTPS with .git",
			args: args{
				repoURL: "https://gitlab-ci-token:token@gitlab.com/deckhouse/flant-integration.git",
			},
			want: "https://gitlab.com/deckhouse/flant-integration",
		},
		{
			name: "Invalid URL",
			args: args{
				repoURL: "https://git@",
			},
			want: "https://",
		},
		{
			name: "Invalid URL 2",
			args: args{
				repoURL: "https://@git",
			},
			want: "https://git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToHTTPS(tt.args.repoURL); got != tt.want {
				t.Errorf("convertToHTTPS() = %v, want %v", got, tt.want)
			}
		})
	}
}
