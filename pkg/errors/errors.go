package errors

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"

	"github.com/deckhouse/dmt/pkg"
)

type lintRuleError struct {
	ID          string
	Module      string
	ObjectID    string
	ObjectValue any
	Text        string
	FilePath    string
	LineNumber  int
	Level       pkg.Level
}

func (l *lintRuleError) EqualsTo(candidate lintRuleError) bool { //nolint:gocritic // it's a simple method
	return l.ID == candidate.ID &&
		l.Text == candidate.Text &&
		l.ObjectID == candidate.ObjectID &&
		l.Module == candidate.Module
}

type errStorage []lintRuleError

type LintRuleErrorsList struct {
	storage *errStorage

	linterID   string
	moduleID   string
	objectID   string
	value      any
	filePath   string
	lineNumber int

	maxLevel *pkg.Level
}

func NewLintRuleErrorsList() *LintRuleErrorsList {
	lvl := pkg.Error
	return &LintRuleErrorsList{
		storage:  &errStorage{},
		maxLevel: &lvl,
	}
}

func NewLinterRuleList(linterID string, module ...string) *LintRuleErrorsList {
	l := &LintRuleErrorsList{
		storage:  &errStorage{},
		linterID: linterID,
	}
	if len(module) > 0 {
		l.moduleID = module[0]
	}

	return l
}

func (l *LintRuleErrorsList) WithMaxLevel(level pkg.Level) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		objectID:   l.objectID,
		value:      l.value,
		filePath:   l.filePath,
		lineNumber: l.lineNumber,
		maxLevel:   &level,
	}
}

func (l *LintRuleErrorsList) CorrespondToMaxLevel() {
	if l.storage == nil {
		return
	}

	for idx, err := range *l.storage {
		if l.maxLevel != nil && *l.maxLevel < err.Level {
			st := *l.storage
			st[idx].Level = *l.maxLevel
		}
	}
}

func (l *LintRuleErrorsList) WithLinterID(linterID string) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	return &LintRuleErrorsList{
		storage:  l.storage,
		linterID: linterID,
		maxLevel: l.maxLevel,
	}
}

func (l *LintRuleErrorsList) WithObjectID(objectID string) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}
	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		objectID:   objectID,
		value:      l.value,
		filePath:   l.filePath,
		lineNumber: l.lineNumber,
		maxLevel:   l.maxLevel,
	}
}

func (l *LintRuleErrorsList) WithModule(moduleID string) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}
	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   moduleID,
		objectID:   l.objectID,
		value:      l.value,
		filePath:   l.filePath,
		lineNumber: l.lineNumber,
		maxLevel:   l.maxLevel,
	}
}

func (l *LintRuleErrorsList) WithValue(value any) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}
	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		objectID:   l.objectID,
		value:      value,
		filePath:   l.filePath,
		lineNumber: l.lineNumber,
		maxLevel:   l.maxLevel,
	}
}

func (l *LintRuleErrorsList) WithFilePath(filePath string) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}
	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		objectID:   l.objectID,
		value:      l.value,
		filePath:   filePath,
		lineNumber: l.lineNumber,
		maxLevel:   l.maxLevel,
	}
}

func (l *LintRuleErrorsList) WithLineNumber(lineNumber int) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}
	return &LintRuleErrorsList{
		storage:    l.storage,
		linterID:   l.linterID,
		moduleID:   l.moduleID,
		objectID:   l.objectID,
		value:      l.value,
		filePath:   l.filePath,
		lineNumber: lineNumber,
		maxLevel:   l.maxLevel,
	}
}

func (l *LintRuleErrorsList) Warn(str string) *LintRuleErrorsList {
	return l.add(str, pkg.Warn)
}

func (l *LintRuleErrorsList) Warnf(template string, a ...any) *LintRuleErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Warn)
}

func (l *LintRuleErrorsList) Error(str string) *LintRuleErrorsList {
	return l.add(str, pkg.Error)
}

func (l *LintRuleErrorsList) Errorf(template string, a ...any) *LintRuleErrorsList {
	return l.add(fmt.Sprintf(template, a...), pkg.Error)
}

// Deprecated: use Errir or Errorf instead
func (l *LintRuleErrorsList) Add(templateOrString string, a ...any) *LintRuleErrorsList {
	if len(a) != 0 {
		templateOrString = fmt.Sprintf(templateOrString, a...)
	}

	return l.add(templateOrString, pkg.Error)
}

func (l *LintRuleErrorsList) add(str string, level pkg.Level) *LintRuleErrorsList {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	if l.maxLevel != nil && *l.maxLevel < level {
		level = *l.maxLevel
	}

	e := lintRuleError{
		ID:          strings.ToLower(l.linterID),
		Module:      l.moduleID,
		ObjectID:    l.objectID,
		ObjectValue: l.value,
		FilePath:    l.filePath,
		LineNumber:  l.lineNumber,
		Text:        str,
		Level:       level,
	}

	if slices.ContainsFunc(*l.storage, e.EqualsTo) {
		return l
	}

	*l.storage = append(*l.storage, e)

	return l
}

// Merge merges another LintRuleErrorsList into current one, removing all duplicate errors.
func (l *LintRuleErrorsList) Merge(e *LintRuleErrorsList) {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	if e == nil {
		return
	}

	for _, el := range *e.storage {
		if slices.ContainsFunc(*l.storage, el.EqualsTo) {
			continue
		}
		if el.Text == "" {
			continue
		}

		*l.storage = append(*l.storage, el)
	}
}

// ConvertToError converts LintRuleErrorsList to a single error.
// It returns an error that contains all errors from the list with a nice formatting.
// If the list is empty, it returns nil.
func (l *LintRuleErrorsList) ConvertToError() error {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	if len(*l.storage) == 0 {
		return nil
	}

	slices.SortFunc(*l.storage, func(a, b lintRuleError) int {
		return cmp.Or(
			cmp.Compare(a.Module, b.Module),
			cmp.Compare(a.ObjectID, b.ObjectID),
		)
	})

	builder := strings.Builder{}
	for _, err := range *l.storage {
		msgColor := color.FgRed

		if err.Level == pkg.Warn {
			msgColor = color.FgHiYellow
		}

		builder.WriteString(fmt.Sprintf(
			"%s%s\n\t%-12s %s\n\t%-12s %s\n",
			emoji.Sprintf(":monkey:"),
			color.New(color.FgHiBlue).SprintfFunc()("[#%s]", err.ID),
			"Message:", color.New(msgColor).SprintfFunc()(err.Text),
			"Module:", err.Module,
		))
		if err.ObjectID != "" && err.ObjectID != err.Module {
			builder.WriteString(fmt.Sprintf("\t%-12s %s\n", "Object:", err.ObjectID))
		}
		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)
			builder.WriteString(fmt.Sprintf("\t%-12s %s\n", "Value:", value))
		}
		if err.FilePath != "" {
			builder.WriteString(fmt.Sprintf("\t%-12s %s\n", "FilePath:", strings.TrimSpace(err.FilePath)))
		}
		if err.LineNumber != 0 {
			builder.WriteString(fmt.Sprintf("\t%-12s %d\n", "LineNumber:", err.LineNumber))
		}
		builder.WriteString("\n")
	}
	return errors.New(builder.String())
}

func (l *LintRuleErrorsList) ContainsCritical() bool {
	if l.storage == nil {
		l.storage = &errStorage{}
	}

	for _, err := range *l.storage {
		if err.Level == pkg.Error {
			return true
		}
	}

	return false
}
