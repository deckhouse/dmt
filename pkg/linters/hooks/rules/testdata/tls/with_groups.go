package tls

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var leaf, _ = tls_certificate.GenerateSelfSignedCert(
	"leaf",
	nil,
	tls_certificate.WithGroups("Deckhouse"),
)
