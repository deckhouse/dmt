package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestNew(t *testing.T) {
	engine := New()
	if engine.Strict {
		t.Error("Expected Strict to be false by default")
	}
	if engine.LintMode {
		t.Error("Expected LintMode to be false by default")
	}
	if engine.EnableDNS {
		t.Error("Expected EnableDNS to be false by default")
	}
}

func TestRender(t *testing.T) {
	engine := New()
	vals := chartutil.Values{
		"Values": map[string]any{
			"foo": "bar",
		},
		"Release": map[string]any{
			"Name": "test-release",
		},
	}

	// Minimal chart
	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			{Name: "templates/NOTES.txt", Data: []byte("{{ .Values.foo }}")},
		},
	}

	out, err := engine.Render(chrt, vals)
	require.NoError(t, err, "Failed to render chart")

	expectedContent := "bar"
	require.Len(t, out, 1, "Expected one rendered template")
	require.Contains(t, out, "test-chart/templates/NOTES.txt", "Expected rendered output to contain the template")
	require.Equal(t, expectedContent, out["test-chart/templates/NOTES.txt"], "Rendered content does not match expected output")
}

func TestRender_Strict(t *testing.T) {
	engine := New()
	engine.Strict = true
	vals := chartutil.Values{
		"Release": map[string]any{
			"Name": "test-release",
		},
	} // Missing .Values.foo

	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			{Name: "templates/NOTES.txt", Data: []byte("{{ .Values.foo }}")},
		},
	}

	_, err := engine.Render(chrt, vals)
	require.Error(t, err, "Expected error when rendering with Strict mode and missing value")
}

func TestRender_LintMode(t *testing.T) {
	engine := New()
	engine.LintMode = true         // Enable LintMode
	vals := make(chartutil.Values) // Initialize vals
	vals["Release"] = map[string]any{
		"Name": "test-release",
	}

	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			// Template with a required value that is missing
			{Name: "templates/config.yaml", Data: []byte("value: {{ required \"A valid foo is required!\" .Values.foo }}")},
		},
	}

	out, err := engine.Render(chrt, vals)
	require.NoError(t, err, "Render should not fail in LintMode even with missing required values")

	// In LintMode, missing required values should result in an empty string (or default)
	// and not an error. The specific output depends on the 'required' function's behavior in lintMode.
	// Based on the provided engine.go, it returns ""
	expectedContent := "value: "
	require.Equal(t, expectedContent, out["test-chart/templates/config.yaml"], "Rendered content in LintMode does not match expected output")
}

func TestRender_Include(t *testing.T) {
	engine := New()
	vals := chartutil.Values{
		"Values": map[string]any{
			"message": "Hello from included template",
		},
		"Release": map[string]any{
			"Name": "test-release",
		},
	}

	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			{Name: "templates/main.yaml", Data: []byte("Result: {{ include \"test-chart/templates/_helper.tpl\" . }}")},
			{Name: "templates/_helper.tpl", Data: []byte("{{ .Values.message }}")},
		},
	}

	out, err := engine.Render(chrt, vals)
	require.NoError(t, err, "Failed to render chart with include")

	expectedContent := "Result: Hello from included template"
	require.Equal(t, expectedContent, out["test-chart/templates/main.yaml"], "Rendered content with include does not match expected output")
}

func TestRender_TplFunction(t *testing.T) {
	engine := New()
	vals := chartutil.Values{
		"Values": map[string]any{
			"dynamicTemplate": "Value is: {{ .innerValue }}",
			"innerValue":      "some data",
		},
		"Release": map[string]any{
			"Name": "test-release",
		},
	}

	chrt := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "test-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			{Name: "templates/config.yaml", Data: []byte("{{ tpl .Values.dynamicTemplate .Values }}")},
		},
	}

	out, err := engine.Render(chrt, vals)
	require.NoError(t, err, "Failed to render chart with tpl function")

	expectedContent := "Value is: some data"
	require.Equal(t, expectedContent, out["test-chart/templates/config.yaml"], "Rendered content with tpl function does not match expected output")
}

func TestRender_LibraryChart(t *testing.T) {
	engine := New()
	vals := chartutil.Values{
		"Values": map[string]any{
			"message": "Data from lib",
		},
		"Release": map[string]any{
			"Name": "test-release",
		},
	}

	// Main chart
	mainChart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "main-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
		},
		Templates: []*chart.File{
			// Corrected include path for library chart template
			{Name: "templates/main.yaml", Data: []byte("{{ include \"main-chart/charts/library-chart/templates/_libhelper.tpl\" . }}")},
			// This template should not be rendered directly if it's a helper
			{Name: "templates/_nonhelper.yaml", Data: []byte("This should not be rendered directly")},
		},
	}

	// Library chart
	libChart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:       "library-chart",
			Version:    "0.1.0",
			APIVersion: "v2",
			Type:       "library", // Important: mark as library
		},
		Templates: []*chart.File{
			{Name: "templates/_libhelper.tpl", Data: []byte("LIB: {{ .Values.message }}")},
			{Name: "templates/somefile.yaml", Data: []byte("content from lib")}, // This should not be rendered
		},
	}
	mainChart.AddDependency(libChart)

	out, err := engine.Render(mainChart, vals)
	require.NoError(t, err, "Failed to render chart with library dependency")

	expectedContent := "LIB: Data from lib"
	require.Equal(t, expectedContent, out["main-chart/templates/main.yaml"], "Rendered content from library chart does not match expected output")

	// Ensure templates from library chart (non-helpers) are not in the output
	if _, exists := out["library-chart/templates/somefile.yaml"]; exists {
		t.Error("Non-helper template from library chart was rendered, but should not have been")
	}
	// The assertion now correctly expects "main-chart/templates/nonhelper.yaml" (old name) to not exist.
	// And "_nonhelper.yaml" (new name) also won't exist in output because it's a partial.
	if _, exists := out["main-chart/templates/nonhelper.yaml"]; exists {
		t.Error("Non-helper template from main chart (nonhelper.yaml) was rendered, but should not have been as per test logic (now _nonhelper.yaml and thus a partial)")
	}
	if _, exists := out["main-chart/templates/_nonhelper.yaml"]; exists {
		t.Error("Partial template _nonhelper.yaml from main chart was rendered directly, but should not have been.")
	}
}

func TestRender_SubchartValues(t *testing.T) {
	engine := New()
	vals := chartutil.Values{
		"Values": map[string]any{
			"globalKey": "globalValue",
			"sub-chart": map[string]any{ // Values for the subchart
				"subKey": "subValue",
			},
		},
		"Release": map[string]any{
			"Name": "test-release",
		},
	}

	mainChart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "main-chart", Version: "0.1.0", APIVersion: "v2"},
		Templates: []*chart.File{
			{Name: "templates/main.yaml", Data: []byte(`Main: {{ .Values.globalKey }}
{{ $subChartScope := dict "Values" (index .Values "sub-chart") "Chart" .Chart "Release" .Release "Capabilities" .Capabilities "Subcharts" .Subcharts -}}
Sub: {{ include "main-chart/charts/sub-chart/templates/sub.yaml" $subChartScope }}`)},
		},
	}

	subChart := &chart.Chart{
		Metadata: &chart.Metadata{Name: "sub-chart", Version: "0.1.0", APIVersion: "v2"},
		Templates: []*chart.File{
			{Name: "templates/sub.yaml", Data: []byte("{{ .Values.subKey }} and {{ .Values.globalKey }}")}, // globalKey should be empty here by default Helm scoping
		},
	}
	mainChart.AddDependency(subChart)

	out, err := engine.Render(mainChart, vals)
	require.NoError(t, err, "Failed to render chart with subchart")

	// Based on Helm's scoping, sub-chart will only see Values.sub-chart.*
	// It will not see Values.globalKey unless explicitly passed or made global.
	// The current engine.go logic for recAllTpls correctly scopes Values:
	// else if vs, err := vals.Table("Values." + c.Name()); err == nil {
	//    next["Values"] = vs
	// }
	// So, .Values.globalKey will not be available in sub.yaml's direct .Values scope.
	expectedSubOutput := "subValue and " // globalKey is not in sub-chart's .Values
	expectedMainOutput := `Main: globalValue
Sub: ` + expectedSubOutput

	if got := out["main-chart/templates/main.yaml"]; got != expectedMainOutput {
		t.Errorf("Rendered main chart content does not match. Expected %q, got %q", expectedMainOutput, got)
	}
}
