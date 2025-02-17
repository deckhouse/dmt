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

package rules

import (
	"regexp"
	"strings"
)

var cyrRe = regexp.MustCompile(`[А-Яа-яЁё]+`)
var cyrPointerRe = regexp.MustCompile(`[А-Яа-яЁё]`)
var cyrFillerRe = regexp.MustCompile(`[^А-Яа-яЁё]`)

func checkCyrillicLetters(in string) (string, bool) {
	if strings.Contains(in, "\n") {
		return checkCyrillicLettersInArray(strings.Split(in, "\n"))
	}
	return checkCyrillicLettersInString(in)
}

// checkCyrillicLettersInString returns a fancy message if input string contains Cyrillic letters.
func checkCyrillicLettersInString(line string) (string, bool) {
	if !cyrRe.MatchString(line) {
		return "", false
	}

	// Replace trim all spaces, because we do not need formatting here
	line = strings.TrimSpace(line)

	// Make string with pointers to Cyrillic letters so user can detect hidden letters.
	cursor := cyrFillerRe.ReplaceAllString(line, "-")
	cursor = cyrPointerRe.ReplaceAllString(cursor, "^")
	cursor = strings.TrimRight(cursor, "-")

	return line + "\n" + cursor, true
}

// checkCyrillicLettersInArray returns a fancy message for each string in array that contains Cyrillic letters.
func checkCyrillicLettersInArray(lines []string) (string, bool) {
	res := make([]string, 0)

	hasCyr := false
	for _, line := range lines {
		msg, has := checkCyrillicLettersInString(line)
		if has {
			hasCyr = true
			res = append(res, msg)
		}
	}

	return strings.Join(res, "\n"), hasCyr
}
