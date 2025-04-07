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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToHTTPS(tt.args.repoURL); got != tt.want {
				t.Errorf("convertToHTTPS() = %v, want %v", got, tt.want)
			}
		})
	}
}
