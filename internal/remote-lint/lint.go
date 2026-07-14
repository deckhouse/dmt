package remotelint

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

	"github.com/deckhouse/deckhouse/pkg/registry/client"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/bundle/docs"
	releaseDocs "github.com/deckhouse/dmt/pkg/remote-linters/release/docs"
)

type RemoteLintOptions struct {
	// Login is the username to use for the registry e.g. license-token
	Login string
	// Password is the password to use for the registry
	Password string
}

// RunRemoteLint runs the remote linting for the given registry image and options
// RegistryPath is the path to the image e.g. registry.example.com/deckhouse/deckhouse:latest
func RunRemoteLint(ctx context.Context, imagePath string, opts *RemoteLintOptions) error {
	registryPath, tag, err := cutTagFromImagePath(imagePath)
	if err != nil {
		return fmt.Errorf("failed to cut tag from image path: %w", err)
	}

	client := initRegistryClient(registryPath, opts.Login, opts.Password)

	errorList := errors.NewLintRuleErrorsList() // .WithMaxLevel()

	err = lintBundle(ctx, client, tag, errorList)
	if err != nil {
		return fmt.Errorf("failed to lint bundle: %w", err)
	}

	err = lintRelease(ctx, client, tag, errorList)
	if err != nil {
		return fmt.Errorf("failed to lint release: %w", err)
	}

	PrintResult(errorList)

	return nil
}

func lintBundle(ctx context.Context, client *client.Client, tag string, errorList *errors.LintRuleErrorsList) error {
	image, err := client.GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	tempDir, err := ExtractImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}
	defer os.RemoveAll(tempDir)

	linters := buildBundleLinters(tempDir, errorList.WithObjectID("bundle"))

	os.Remove(filepath.Join(tempDir, "docs", "README.md")) // debug: delete this line

	for _, linter := range linters {
		linter.Lint(ctx)
	}

	return nil
}

func lintRelease(ctx context.Context, client *client.Client, tag string, errorList *errors.LintRuleErrorsList) error {
	releaseImage, err := client.WithSegment("release").GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get release image: %w", err)
	}

	tempReleaseDir, err := ExtractImage(ctx, releaseImage)
	if err != nil {
		return fmt.Errorf("failed to extract release image: %w", err)
	}
	defer os.RemoveAll(tempReleaseDir)

	os.Remove(filepath.Join(tempReleaseDir, "changelog.yaml")) // debug: delete this line

	linters := buildReleaseLinters(tempReleaseDir, errorList.WithObjectID("release"))

	for _, linter := range linters {
		linter.Lint(ctx)
	}

	return nil
}

// returns repository path and tag (or digest) from the image path
// turns strings like "registry.example.com/my-module:v0.0.1" into "registry.example.com/my-module" and "v0.0.1" (or sha256:aaa)
func cutTagFromImagePath(imagePath string) (string, string, error) {
	if parts := strings.Split(imagePath, "@"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	if parts := strings.Split(imagePath, ":"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("tag not found in image path: %s", imagePath)
}

type Linter interface {
	Lint(ctx context.Context)
}

func buildBundleLinters(path string, errorList *errors.LintRuleErrorsList) []Linter {
	docsLinter := docs.NewLinter(docs.Config{Path: path}, errorList)

	return []Linter{
		docsLinter,
	}
}

func buildReleaseLinters(path string, errorList *errors.LintRuleErrorsList) []Linter {
	docsLinter := releaseDocs.NewLinter(releaseDocs.Config{Path: path}, errorList)

	return []Linter{
		docsLinter,
	}
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
