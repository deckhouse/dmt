{{- define "forms.rawScalar" -}}
{{ . }}
{{- end -}}
{{- define "forms.chainA" -}}
{{ include "forms.chainB" . }}
{{- end -}}
{{- define "forms.chainB" -}}
{{ . }}
{{- end -}}
{{- define "forms.listItems" -}}
{{- range .items }}
- {{ . }}
{{- end }}
{{- end -}}
