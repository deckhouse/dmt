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

package license

import (
	"errors"
	"fmt"
	"os"
	"regexp"
)

var CELicenseRe = regexp.MustCompile(`(?s)[/#{!-]*(\s)*Copyright 202[1-9] Flant JSC[-!}\n#/]*
[/#{!-]*(\s)*Licensed under the Apache License, Version 2\.0 \(the "License"\);[-!}\n]*
[/#{!-]*(\s)*you may not use this file except in compliance with the License\.[-!}\n]*
[/#{!-]*(\s)*You may obtain a copy of the License at[-!}\n#/]*
[/#{!-]*(\s)*http://www\.apache\.org/licenses/LICENSE-2\.0[-!}\n#/]*
[/#{!-]*(\s)*Unless required by applicable law or agreed to in writing, software[-!}\n]*
[/#{!-]*(\s)*distributed under the License is distributed on an "AS IS" BASIS,[-!}\n]*
[/#{!-]*(\s)*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied\.[-!}\n]*
[/#{!-]*(\s)*See the License for the specific language governing permissions and[-!}\n]*
[/#{!-]*(\s)*limitations under the License\.[-!}\n]*`)

var fileToCheckRe = regexp.MustCompile(
	`\.go$|/[^.]+$|\.sh$|\.lua$|\.py$|^\.github/(scripts|workflows|workflow_templates)/.+\.(js|yml|yaml|sh)$`,
)
var fileToSkipRe = regexp.MustCompile(
	`geohash.lua$|\.github/CODEOWNERS|Dockerfile$|Makefile$|/docs/documentation/|/docs/site/|bashrc$|inputrc$` +
		`|modules_menu_skip$|LICENSE$|tools/spelling/.+|/lib/python/|charts/helm_lib`,
)

var copyrightOrAutogenRe = regexp.MustCompile(`Copyright The|autogenerated|DO NOT EDIT`)
var copyrightRe = regexp.MustCompile(`Copyright`)
var flantRe = regexp.MustCompile(`Flant|Deckhouse`)

const bufSize int = 1024

// checkFileCopyright returns true if file is readable and has no copyright information in it.
func checkFileCopyright(fName string) (bool, error) {
	// Original script 'validate_copyright.sh' used 'head -n 10'.
	// Here we just read first 1024 bytes.
	headBuf, err := readFileHead(fName, bufSize)
	if err != nil {
		return false, err
	}

	// Skip autogenerated file or file already has other than Flant copyright
	if copyrightOrAutogenRe.Match(headBuf) {
		return true, errors.New("generated code or other license")
	}

	// Check Flant license if file contains keywords.
	if flantRe.Match(headBuf) {
		return true, nil
	}

	// Skip file with some other copyright
	if copyrightRe.Match(headBuf) {
		return true, errors.New("contains other license")
	}

	return false, errors.New("no copyright or license information")
}

func readFileHead(fName string, size int) ([]byte, error) {
	file, err := os.Open(fName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("directory")
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlink")
	}

	headBuf := make([]byte, size)
	_, err = file.Read(headBuf)
	if err != nil {
		return nil, err
	}

	return headBuf, nil
}
