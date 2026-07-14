package remotelint

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	regclient "github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type RemoteLintOptions struct {
	// Login is the username to use for the registry e.g. license-token
	Login string
	// Password is the password to use for the registry
	Password string
}

// RunRemoteLint runs the remote linting for the given registry image and options
// RegistryPath is the path to the image e.g. registry.example.com/deckhouse/deckhouse:latest
func RunRemoteLint(ctx context.Context, imagePath string, opts *RemoteLintOptions) error {
	registryPath, tag, err := cutTagFromImagePath(imagePath)
	if err != nil {
		return fmt.Errorf("failed to cut tag from image path: %w", err)
	}

	client := initRegistryClient(registryPath, opts.Login, opts.Password)

	image, err := client.GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "dmt-"+imagePath)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	log.Info("extracting image to temp directory", slog.String("tempDir", tempDir))

	rc := image.Extract()
	defer rc.Close()

	err = Extract(ctx, rc, tempDir)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}

	return nil
}

// returns repository path and tag (or digest) from the image path
// turns strings like "registry.example.com/my-module:v0.0.1" into "registry.example.com/my-module" and "v0.0.1" (or sha256:aaa)
func cutTagFromImagePath(imagePath string) (string, string, error) {
	if parts := strings.Split(imagePath, "@"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	if parts := strings.Split(imagePath, ":"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("tag not found in image path: %s", imagePath)
}

func initRegistryClient(registryHost string, login, password string) *regclient.Client {
	auth := registryAuth(registryHost, login, password)

	return regclient.New(registryHost, regclient.WithAuth(auth))
}

// registryAuth resolves credentials for the source registry, mirroring the
// pre-#386 priority: explicit login/password, then license token, then the
// Docker config, then anonymous.
func registryAuth(registryHost string, login, password string) authn.Authenticator {
	if login != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: login,
			Password: password,
		})
	}

	if auth, ok := dockerConfigAuth(registryHost); ok {
		return auth
	}

	log.Debug("using anonymous access for the source registry", slog.String("registry", registryHost))

	return authn.Anonymous
}

// dockerConfigAuth resolves credentials for registryHost from the Docker config
// (~/.docker/config.json, written by `d8 dk cr login`). ok is false when the
// config holds no usable entry for the host.
func dockerConfigAuth(registryHost string) (authn.Authenticator, bool) {
	ref, err := name.ParseReference(registryHost)
	if err != nil {
		return nil, false
	}

	log.Warn("debug dockerConfigAuth", slog.String(
		"registryHost", registryHost),
		slog.String("ref", ref.String()),
		slog.String("registry", ref.Context().RegistryStr()),
	)

	reg, err := name.NewRegistry(ref.Context().RegistryStr())
	if err != nil {
		return nil, false
	}

	auth, err := authn.DefaultKeychain.Resolve(reg)
	if err != nil || auth == authn.Anonymous {
		return nil, false
	}

	cfg, err := auth.Authorization()
	if err != nil {
		return nil, false
	}

	if cfg.Username == "" && cfg.Password == "" && cfg.Auth == "" && cfg.IdentityToken == "" {
		return nil, false
	}

	log.Debug("using Docker config credentials", slog.String("registry", reg.String()))

	return auth, true
}
