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

package module

import (
	"regexp"
	"strings"
)

var numberSequence = regexp.MustCompile(`([a-zA-Z])(\d+)([a-zA-Z]?)`)
var numberReplacement = []byte(`$1 $2 $3`)

func addWordBoundariesToNumbers(s string) string {
	b := []byte(s)
	b = numberSequence.ReplaceAll(b, numberReplacement)
	return string(b)
}

// toCamelInitCase converts a string to CamelCase with specified initial case
func toCamelInitCase(s string, initCase bool) string {
	s = addWordBoundariesToNumbers(s)
	s = strings.Trim(s, " ")
	n := ""
	capNext := initCase
	for _, v := range s {
		if v >= 'A' && v <= 'Z' {
			n += string(v)
		}
		if v >= '0' && v <= '9' {
			n += string(v)
		}
		if v >= 'a' && v <= 'z' {
			if capNext {
				n += strings.ToUpper(string(v))
			} else {
				n += string(v)
			}
		}
		if v == '_' || v == ' ' || v == '-' || v == '.' {
			capNext = true
		} else {
			capNext = false
		}
	}
	return n
}

// processCamelCase processes a string for camel case conversion with acronym handling
func processCamelCase(s string, initCase bool) string {
	if uppercaseAcronym[s] {
		s = strings.ToLower(s)
	}

	if !initCase && s != "" {
		if uppercaseAcronym[s] {
			s = strings.ToLower(s)
		}
		if r := rune(s[0]); r >= 'A' && r <= 'Z' {
			s = strings.ToLower(string(r)) + s[1:]
		}
	}

	return toCamelInitCase(s, initCase)
}

var uppercaseAcronym = map[string]bool{
	"ID": true,
}

// ToCamel converts a string to CamelCase
func ToCamel(s string) string {
	return processCamelCase(s, true)
}

// ToLowerCamel converts a string to lowerCamelCase
func ToLowerCamel(s string) string {
	if s == "" {
		return s
	}
	return processCamelCase(s, false)
}
