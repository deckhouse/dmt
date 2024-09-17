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

	// Replace all tabs with spaces to prevent shifted cursor.
	line = strings.ReplaceAll(line, "\t", "    ")

	// Make string with pointers to Cyrillic letters so user can detect hidden letters.
	cursor := cyrFillerRe.ReplaceAllString(line, "-")
	cursor = cyrPointerRe.ReplaceAllString(cursor, "^")
	cursor = strings.TrimRight(cursor, "-")

	const formatPrefix = "  "

	return "\n" + formatPrefix + line + "\n" + formatPrefix + cursor, true
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

func addPrefix(lines []string, prefix string) string {
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(prefix)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}
