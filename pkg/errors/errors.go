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
	ID          string
	Module      string
	ObjectID    string
	ObjectValue any
	Text        string
}

func (l *LintRuleError) EqualsTo(candidate *LintRuleError) bool {
	return l.ID == candidate.ID && l.Text == candidate.Text && l.ObjectID == candidate.ObjectID
}

func NewLintRuleError(id, objectID, module string, value any, template string, a ...any) *LintRuleError {
	return &LintRuleError{
		ObjectID:    objectID,
		ObjectValue: value,
		Text:        fmt.Sprintf(template, a...),
		ID:          strings.ToLower(id),
		Module:      module,
	}
}

func NewLinterRuleList() *LintRuleErrorsList {
	return &LintRuleErrorsList{
		storage: new(errStorage),
	}
}

type LintRuleErrorsList struct {
	data []*LintRuleError

	storage *errStorage

	linterID string
	moduleID string
	objectID string
}

type errStorage struct {
	data []*LintRuleError
}

var (
	ErrLinterIDIsEmpty = errors.New("linter ID is empty")
	ErrModuleIDIsEmpty = errors.New("module ID is empty")
	ErrObjectIDIsEmpty = errors.New("object ID is empty")
)

func (l *LintRuleErrorsList) Validate() error {
	var errs error

	if l.linterID == "" {
		errs = errors.Join(errs, ErrLinterIDIsEmpty)
	}

	if l.moduleID == "" {
		errs = errors.Join(errs, ErrModuleIDIsEmpty)
	}

	if l.objectID == "" {
		errs = errors.Join(errs, ErrObjectIDIsEmpty)
	}

	return errs
}

func (l *LintRuleErrorsList) DumpFromStorage() {
	l.data = l.storage.data
}

// if you change linter ID - all settings must be reset
func (l *LintRuleErrorsList) WithLinterID(id string) *LintRuleErrorsList {
	return &LintRuleErrorsList{
		storage:  l.storage,
		linterID: id,
	}
}

// if you change module ID - all settings except linter ID must be reset
func (l *LintRuleErrorsList) WithModuleID(module string) *LintRuleErrorsList {
	return &LintRuleErrorsList{
		storage:  l.storage,
		linterID: l.linterID,
		moduleID: module,
	}
}

// if you change module ID - all settings except linter and module ID must be reset
func (l *LintRuleErrorsList) WithObjectID(objectID string) *LintRuleErrorsList {
	return &LintRuleErrorsList{
		storage:  l.storage,
		linterID: l.linterID,
		moduleID: l.moduleID,
		objectID: objectID,
	}
}

func (l *LintRuleErrorsList) AddWithValue(value any, template string, a ...any) *LintRuleErrorsList {
	return l.add(value, fmt.Sprintf(template, a...))
}

func (l *LintRuleErrorsList) AddF(template string, a ...any) *LintRuleErrorsList {
	return l.add(nil, fmt.Sprintf(template, a...))
}

func (l *LintRuleErrorsList) Addln(str string) *LintRuleErrorsList {
	return l.add(nil, str)
}

func (l *LintRuleErrorsList) AddErr(err error) *LintRuleErrorsList {
	return l.add(nil, err.Error())
}

func (l *LintRuleErrorsList) add(value any, str string) *LintRuleErrorsList {
	if err := l.Validate(); err != nil {
		panic(err)
	}

	e := &LintRuleError{
		ID:          strings.ToLower(l.linterID),
		Module:      l.moduleID,
		ObjectID:    l.objectID,
		ObjectValue: value,
		Text:        str,
	}

	if slices.ContainsFunc(l.data, e.EqualsTo) {
		return l
	}

	l.storage.data = append(l.storage.data, e)

	return l
}

// Add adds new error to the list if it doesn't exist yet.
// It first checks if error is empty (i.e. all its fields are empty strings)
// and then checks if error with the same ID, ObjectId and Text already exists in the list.
func (l *LintRuleErrorsList) Add(e *LintRuleError) {
	if e == nil {
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

		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)
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
