package errors

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
)

type LintRuleError struct {
	Text     string
	ID       string
	ObjectID string
	Value    any
	Module   string
}

// EqualsTo checks if two LintRuleError objects are equal.
// It returns true if both objects have the same ID, Text and ObjectID.
func (l *LintRuleError) EqualsTo(candidate *LintRuleError) bool {
	return l.ID == candidate.ID && l.Text == candidate.Text && l.ObjectID == candidate.ObjectID
}

func NewLintRuleError(id, objectID, module string, value any, template string, a ...any) *LintRuleError {
	return &LintRuleError{
		ID:       strings.ToLower(id),
		ObjectID: objectID,
		Module:   module,
		Value:    value,
		Text:     fmt.Sprintf(template, a...),
	}
}

type LintRuleErrorsList struct {
	data []*LintRuleError
}

// Add adds new error to the list if it doesn't exist yet.
// It first checks if error is empty (i.e. all its fields are empty strings)
// and then checks if error with the same ID, ObjectId and Text already exists in the list.
// If the error is already in the list, it doesn't add it again.
// It returns the list itself to allow chaining.
func (l *LintRuleErrorsList) Add(e *LintRuleError) *LintRuleErrorsList {
	if e == nil {
		return l
	}
	if slices.ContainsFunc(l.data, e.EqualsTo) {
		return l
	}
	l.data = append(l.data, e)

	return l
}

// Merge merges another LintRuleErrorsList into current one, removing all duplicate errors.
// It returns the list itself to allow chaining.
func (l *LintRuleErrorsList) Merge(e *LintRuleErrorsList) *LintRuleErrorsList {
	for _, el := range e.data {
		l.Add(el)
	}

	return l
}

// ConvertToError converts LintRuleErrorsList to a single error.
// It returns an error that contains all errors from the list with a nice formatting.
// If the list is empty, it returns nil.
func (l *LintRuleErrorsList) ConvertToError() error {
	if len(l.data) == 0 {
		return nil
	}
	slices.SortFunc(l.data, func(a, b *LintRuleError) int {
		return cmp.Or(
			cmp.Compare(a.Module, b.Module),
			cmp.Compare(a.ObjectID, b.ObjectID),
		)
	})

	warningOnlyLinters := map[string]struct{}{}
	for _, wo := range WarningsOnly {
		warningOnlyLinters[wo] = struct{}{}
	}

	builder := strings.Builder{}
	for _, err := range l.data {
		msgColor := color.FgRed
		if _, ok := warningOnlyLinters[err.ID]; ok {
			msgColor = color.FgHiYellow
		}

		builder.WriteString(fmt.Sprintf(
			"%s%s\n\tMessage\t- %s\n\tObject\t- %s\n\tModule\t- %s\n",
			emoji.Sprintf(":monkey:"),
			color.New(color.FgHiBlue).SprintfFunc()("[#%s]", err.ID),
			color.New(msgColor).SprintfFunc()(err.Text),
			err.ObjectID,
			err.Module,
		))

		if err.Value != nil {
			value := fmt.Sprintf("%v", err.Value)
			builder.WriteString(fmt.Sprintf("\tValue\t- %s\n", value))
		}
		builder.WriteString("\n")
	}

	return errors.New(builder.String())
}

var WarningsOnly []string

func (l *LintRuleErrorsList) Critical() bool {
	for _, err := range l.data {
		if !slices.Contains(WarningsOnly, err.ID) {
			return true
		}
	}

	return false
}
