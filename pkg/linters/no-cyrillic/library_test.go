package nocyrillic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_found_msg(t *testing.T) {
	// Simple check with one Cyrillic letter.
	in := "fooБfoo"
	expected := `fooБfoo
---^`

	actual, has := checkCyrillicLetters(in)

	require.True(t, has, "Should detect cyrillic letters in string")
	require.Equal(t, expected, actual)

	// No Cyrillic letters.
	in = "asdqwe 123456789 !@#$%^&*( ZXCVBNM"
	expected = ""
	actual, has = checkCyrillicLetters(in)

	require.False(t, has, "Should not detect cyrillic letters in string")
	require.Equal(t, expected, actual)

	// Multiple words with Cyrillic letters.
	in = "asdqwe Там на qw q cheсk tеst qwd неведомых qqw"
	expected = `asdqwe Там на qw q cheсk tеst qwd неведомых qqw
-------^^^-^^---------^---^-------^^^^^^^^^`

	actual, has = checkCyrillicLetters(in)
	require.True(t, has, "Should detect cyrillic letters in string")
	require.Equal(t, expected, actual)

	// Multiple messages for string with '\n'.
	in = "Lorem ipsum dolor sit amet,\n consectetur adipiscing elit,\n" +
		"раскрою перед вами всю \nкартину и разъясню," +
		"Ut enim ad minim veniam,"
	expected = `раскрою перед вами всю
^^^^^^^-^^^^^-^^^^-^^^
картину и разъясню,Ut enim ad minim veniam,
^^^^^^^-^-^^^^^^^^`

	actual, has = checkCyrillicLetters(in)
	require.True(t, has, "Should detect cyrillic letters in string")
	require.Equal(t, expected, actual)
}
