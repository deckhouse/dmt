#!/bin/sh -e

VERSION=$1
if [ -z "$VERSION" ] ; then
    echo "Required version argument!" 1>&2
    echo 1>&2
    echo "Usage: $0 VERSION" 1>&2
    exit 1
fi

export CGO_ENABLED=0

go run github.com/mitchellh/gox@latest -osarch="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64" \
    -output="release-build/$VERSION/{{.OS}}-{{.Arch}}/bin/dmt" \
    -ldflags="-s -w -X main.version=$VERSION" \
        github.com/deckhouse/dmt/cmd/dmt