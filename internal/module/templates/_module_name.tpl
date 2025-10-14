{{- define "helm_lib_module_camelcase_name" -}}
{{- $chartName := .Chart.Name | required "Chart.Name is required" | lower -}}
{{- if eq $chartName "" -}}
  {{- "module" -}}  {{/* Fallback if empty */}}
{{- else if not (contains "-" $chartName) -}}
  {{- $chartName -}}
{{- else -}}
  {{- $spaced := replace $chartName "-" " " -}}
  {{- $titled := title $spaced -}}
  {{- $joined := replace $titled " " "" -}}
  {{- if le (len $joined) 1 -}}
    {{- lower $joined -}}
  {{- else -}}
    {{- $first := lower (substr (int 0) (int 1) $joined) -}}
    {{- $rest := substr (int 1) (int (sub (len $joined) 1)) $joined -}}
    {{- printf "%s%s" $first $rest -}}
  {{- end -}}
{{- end -}}
{{- end -}}