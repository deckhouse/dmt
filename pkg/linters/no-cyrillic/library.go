package nocyrillic

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
