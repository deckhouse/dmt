package docs

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
	"github.com/mitchellh/go-wordwrap"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/docs/rules"
)

// LinterID is the stable identifier used to reference this linter in configuration and diagnostics.
const LinterID = "docs"

// Linter runs documentation rules against a package directory.
type Linter struct {
	config    Config
	errorList *errors.LintRuleErrorsList
}

// Config holds the path and settings required to construct a Linter.
type Config struct {
	Path string
}

// NewLinter constructs a Linter from cfg, scoping its diagnostics to this linter and capping severity at the configured level.
func NewLinter(cfg Config, errorList *errors.LintRuleErrorsList) *Linter {
	return &Linter{
		config:    cfg,
		errorList: errorList.WithLinterID(LinterID),
	}
}

// Lint executes all documentation rules against the configured package path.
func (l *Linter) Lint(ctx context.Context) {
	if !hasDocsDir(l.config.Path) {
		l.errorList.WithFilePath(l.config.Path).Warn("docs folder not found in package root")
		return
	}

	rules.NewReadmeRule(l.config.Path, l.errorList).Check(ctx)
	// rules.NewBilingualRule(l.config.Path).Check(ctx)
	// rules.NewCyrillicInEnglishRule(l.config.Path).Check(ctx)
}

// hasDocsDir reports whether docs/ exists as a directory in the package root.
func hasDocsDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "docs"))
	return err == nil && info.IsDir()
}

func PrintResult(errorList *errors.LintRuleErrorsList) {
	errs := errorList.GetErrors()

	if len(errs) == 0 {
		return
	}

	slices.SortFunc(errs, func(a, b pkg.LinterError) int {
		return cmp.Or(
			cmp.Compare(a.Level, b.Level),
			cmp.Compare(a.LinterID, b.LinterID),
			cmp.Compare(a.RuleID, b.RuleID),
		)
	})

	w := new(tabwriter.Writer)

	const minWidth = 5

	buf := bytes.NewBuffer([]byte{})
	w.Init(buf, minWidth, 0, 0, ' ', 0)

	for idx := range errs {
		err := errs[idx]

		msgColor := color.FgRed

		if err.Level == pkg.Ignored {
			// TODO: make it not global
			if !flags.ShowIgnored {
				continue
			}

			msgColor = color.FgWhite
		}

		if err.Level == pkg.Warn {
			// TODO: make it not global
			if flags.HideWarnings {
				continue
			}

			msgColor = color.FgHiYellow
		}

		// header
		fmt.Fprint(w, emoji.Sprintf(":monkey:"))
		fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()("["))

		if err.RuleID != "" {
			fmt.Fprint(w, color.New(color.FgHiBlue).SprintFunc()(err.RuleID+" "))
		}

		fmt.Fprintf(w, "%s\n", color.New(color.FgHiBlue).SprintfFunc()("(#%s)]", err.LinterID))

		// body
		fmt.Fprintf(w, "\t%s\t\t%s\n", "Message:", color.New(msgColor).SprintfFunc()(prepareString(err.Text)))

		if err.ObjectID != "" && err.ObjectID != err.ModuleID {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "Object:", err.ObjectID)
		}

		if err.ObjectValue != nil {
			value := fmt.Sprintf("%v", err.ObjectValue)

			fmt.Fprintf(w, "\t%s\t\t%s\n", "Value:", prepareString(value))
		}

		if err.FilePath != "" {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "FilePath:", strings.TrimSpace(err.FilePath))
		}

		if err.LineNumber != 0 {
			fmt.Fprintf(w, "\t%s\t\t%d\n", "LineNumber:", err.LineNumber)
		}

		if err.FixError != nil {
			fmt.Fprintf(w, "\t%s\t\t%s\n", "AutofixError:", color.New(color.FgHiYellow).Sprint(err.FixError.Error()))
		}

		// if flags.ShowDocumentation {
		// 	docURL := generateDocumentationURL(err.LinterID, err.RuleID)
		// 	if docURL != "" {
		// 		fmt.Fprintf(w, "\t%s\t\t%s\n", "Documentation:", docURL)
		// 	}
		// }

		fmt.Fprintln(w)

		w.Flush()
	}

	fmt.Println(buf.String())
}

// prepareString handle ussual string and prepare it for tablewriter
func prepareString(input string) string {
	// magic wrap const
	const wrapLen = 100

	w := &strings.Builder{}

	// split wraps for tablewrite
	split := strings.Split(wordwrap.WrapString(input, wrapLen), "\n")

	// first string must be pure for correct handling
	fmt.Fprint(w, strings.TrimSpace(split[0]))

	for i := 1; i < len(split); i++ {
		fmt.Fprintf(w, "\n\t\t\t%s", strings.TrimSpace(split[i]))
	}

	return w.String()
}
