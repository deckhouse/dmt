package tls

// This file uses the same bogus strings/options but does NOT import the
// tls_certificate library, so the rule must skip it entirely.

func WithGroups(string) string { return "" }

func GenerateSelfSignedCert(args ...any) string { return "" }

var usages = []string{"requestheader-client"}

var skipped = GenerateSelfSignedCert("leaf", WithGroups("Deckhouse"))
