package helm

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// Catch warning logs messages and filter them
// https://github.com/helm/helm/issues/7019
type FilteredHelmWriter struct {
	Writer io.Writer
}

var _ io.Writer = (*FilteredHelmWriter)(nil)

func (w *FilteredHelmWriter) Write(p []byte) (n int, err error) {
	builder := strings.Builder{}

	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.Contains(line, "found symbolic link in path") {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	result := strings.TrimSuffix(builder.String(), "\n")
	return w.Writer.Write([]byte(result))
}
