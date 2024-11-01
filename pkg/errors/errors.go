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

func (l *LintRuleError) EqualsTo(candidate *LintRuleError) bool {
	return l.ID == candidate.ID && l.Text == candidate.Text && l.ObjectID == candidate.ObjectID
}

func (l *LintRuleError) IsEmpty() bool {
	return l.ID == "" && l.Text == "" && l.ObjectID == ""
}

func NewLintRuleError(id, objectID, module string, value any, template string, a ...any) *LintRuleError {
	return &LintRuleError{
		ObjectID: objectID,
		Value:    value,
		Text:     fmt.Sprintf(template, a...),
		ID:       id,
		Module:   module,
	}
}

var EmptyRuleError = &LintRuleError{Text: "", ID: "", ObjectID: ""}

type LintRuleErrorsList struct {
	data []*LintRuleError
}

// Add adds new error to the list if it doesn't exist yet.
// It first checks if error is empty (i.e. all its fields are empty strings)
// and then checks if error with the same ID, ObjectId and Text already exists in the list.
func (l *LintRuleErrorsList) Add(e *LintRuleError) {
	if e.IsEmpty() {
		return
	}
	if slices.ContainsFunc(l.data, e.EqualsTo) {
		return
	}
	l.data = append(l.data, e)
}

// Merge merges another LintRuleErrorsList into current one, removing all duplicate errors.
func (l *LintRuleErrorsList) Merge(e LintRuleErrorsList) {
	for _, el := range e.data {
		l.Add(el)
	}
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
	builder := strings.Builder{}
	for _, err := range l.data {
		builder.WriteString(fmt.Sprintf(
			"%s%s\n\tMessage\t- %s\n\tObject\t- %s\n\tModule\t- %s\n",
			emoji.Sprintf(":monkey:"),
			color.New(color.FgHiBlue).SprintfFunc()("[#%s]", err.ID),
			color.New(color.FgRed).SprintfFunc()(err.Text),
			err.ObjectID,
			err.Module,
		))

		if err.Value != nil {
			builder.WriteString(fmt.Sprintf("\tValue\t- %s\n", err.Value))
		}
		builder.WriteString("\n")
	}

	return errors.New(builder.String())
}

var WarningsOnly []string

func (l *LintRuleErrorsList) Critical() bool {
	for _, err := range l.data {
		if slices.Contains(WarningsOnly, err.ID) {
			continue
		}
		return true
	}

	return false
}
