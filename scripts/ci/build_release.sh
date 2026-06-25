#!/bin/sh -e

VERSION=$1
if [ -z "$VERSION" ] ; then
    echo "Required version argument!" 1>&2
    echo 1>&2
    echo "Usage: $0 VERSION" 1>&2
    exit 1
fi

export CGO_ENABLED=0

# Mark the checkout as a safe git directory so `git` works inside the build
# container (trdl mounts the repo with a different owner).
git config --global --add safe.directory "$(pwd)" 2>/dev/null || true

COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG="github.com/deckhouse/dmt/internal/version"

go run github.com/mitchellh/gox@latest -osarch="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64" \
    -output="release-build/$VERSION/{{.OS}}-{{.Arch}}/bin/dmt" \
    -ldflags="-s -w -X ${VERSION_PKG}.Version=$VERSION -X ${VERSION_PKG}.Commit=$COMMIT -X ${VERSION_PKG}.Date=$DATE" \
        github.com/deckhouse/dmt/cmd/dmt