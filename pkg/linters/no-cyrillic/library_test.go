package no_cyrillic

import (
	"fmt"
	"strings"
	"testing"
)

func Test_found_msg(t *testing.T) {
	// Simple check with one Cyrillic letter.
	in := "fooБfoo"
	expected := `  fooБfoo
  ---^`

	actual, has := checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		t.Errorf("Expect '%s', got '%s'", expected, actual)
	}

	// No Cyrillic letters.
	in = "asdqwe 123456789 !@#$%^&*( ZXCVBNM"
	expected = ""
	actual, has = checkCyrillicLetters(in)

	if has {
		t.Errorf("Should not detect cyrillic letters in string")
	}

	if actual != expected {
		t.Errorf("Expect '%s', got '%s'", expected, actual)
	}

	// Multiple words with Cyrillic letters.
	in = "asdqwe Там на qw q cheсk tеst qwd неведомых qqw"
	expected =
		"  asdqwe Там на qw q cheсk tеst qwd неведомых qqw\n" +
			"  -------^^^-^^---------^---^-------^^^^^^^^^"

	actual, has = checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		fmt.Printf("  %s\n%s\n",
			strings.Repeat("0123456789", len(actual)/2/10+1),
			actual)
		t.Errorf("Expect \n%s\n, got \n%s\n", expected, actual)
	}

	// Multiple messages for string with '\n'.
	in = "Lorem ipsum dolor sit amet,\n consectetur adipiscing elit,\n" +
		"раскрою перед вами всю \nкартину и разъясню," +
		"Ut enim ad minim veniam,"
	expected =
		"  раскрою перед вами всю \n" +
			"  ^^^^^^^-^^^^^-^^^^-^^^\n" +
			"  картину и разъясню,Ut enim ad minim veniam,\n" +
			"  ^^^^^^^-^-^^^^^^^^"

	actual, has = checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		fmt.Printf("  %s\n%s\n",
			strings.Repeat("0123456789", len(actual)/2/10+1),
			actual)
		t.Errorf("Expect \n%s\n, got \n%s\n", expected, actual)
	}

}
