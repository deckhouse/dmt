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

type Error struct {
	ID          string
	Module      string
	ObjectID    string
	ObjectValue any
	Text        string
}

type ErrorList []Error

var result = &ErrorList{}

func NewError(linterID string, module ...string) *Error {
	l := &Error{
		ID: linterID,
	}
	if len(module) > 0 {
		l.Module = module[0]
	}

	return l
}

func GetErrors() *ErrorList {
	return result
}

func (l *Error) WithLinterID(id string) *Error {
	return &Error{
		ID:          id,
		Module:      l.Module,
		ObjectID:    l.ObjectID,
		ObjectValue: l.ObjectValue,
	}
}

func (l *Error) WithObjectID(objectID string) *Error {
	return &Error{
		ID:          l.ID,
		Module:      l.Module,
		ObjectID:    objectID,
		ObjectValue: l.ObjectValue,
	}
}

func (l *Error) WithModule(moduleID string) *Error {
	return &Error{
		ID:          l.ID,
		Module:      moduleID,
		ObjectID:    l.ObjectID,
		ObjectValue: l.ObjectValue,
	}
}

func (l *Error) WithValue(value any) *Error {
	return &Error{
		ID:          l.ID,
		Module:      l.Module,
		ObjectID:    l.ObjectID,
		ObjectValue: value,
	}
}

func (l *Error) Add(template string, a ...any) {
	if len(a) > 0 {
		template = fmt.Sprintf(template, a...)
	}

	e := Error{
		ID:          strings.ToLower(l.ID),
		Module:      l.Module,
		ObjectID:    l.ObjectID,
		ObjectValue: l.ObjectValue,
		Text:        template,
	}

	*result = append(*result, e)
}

// ConvertToError converts LintRuleErrorsList to a single error.
// It returns an error that contains all errors from the list with a nice formatting.
// If the list is empty, it returns nil.
func (l *ErrorList) ConvertToError() error {
	if l == nil || len(*l) == 0 {
		return nil
	}
	slices.SortFunc(*l, func(a, b Error) int {
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
	for _, err := range *l {
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

		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)
			builder.WriteString(fmt.Sprintf("\tValue\t- %s\n", value))
		}
		builder.WriteString("\n")
	}

	return errors.New(builder.String())
}

var WarningsOnly []string

func (l *ErrorList) Critical() bool {
	if l == nil {
		return false
	}

	for _, err := range *l {
		if !slices.Contains(WarningsOnly, err.ID) {
			return true
		}
	}

	return false
}
