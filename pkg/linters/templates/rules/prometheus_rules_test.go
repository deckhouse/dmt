package rules

import "testing"

func TestMe(_ *testing.T) {
	aaa := `
{{ $namespace := "d8-monitoring" }}

{{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/extended-monitoring") }}
{{- if .Values.extendedMonitoring.imageAvailability.exporterEnabled }}
  {{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/image-availability") }}
{{- end }}
{{- if .Values.extendedMonitoring.certificates.exporterEnabled }}
  {{- include "helm_lib_prometheus_rules_recursion" (list . $namespace "monitoring/prometheus-rules/certificates") }}
{{- end }}
`

	isContentMatching([]byte(aaa), "include \"helm_lib_prometheus_rules")
}
