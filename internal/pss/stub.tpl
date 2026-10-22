{{- /*
Replaces pod_security_standard_* defines from admission-policy-engine templates/_helpers.tpl:
instead of a constraint, emit a "kind + parameters" pair.
Bound to the call signature (list $context "<Kind>" $enforcementAction [$parameters]).
D8PrivilegedContainer is called without parameters, hence the len check.
*/ -}}
{{- define "pod_security_standard_baseline" }}
- kind: {{ index . 1 }}
  parameters: {{ if gt (len .) 3 }}{{ index . 3 | toJson }}{{ else }}{}{{ end }}
{{- end }}
{{- define "pod_security_standard_restricted" }}
- kind: {{ index . 1 }}
  parameters: {{ if gt (len .) 3 }}{{ index . 3 | toJson }}{{ else }}{}{{ end }}
{{- end }}
{{- /*
Replaces ape.only_ready_constraints: in the cluster it drops constraints whose template is not
ready yet; dmt checks against all of them, so the rendered pairs pass through as is.
*/ -}}
{{- define "ape.only_ready_constraints" }}
{{- index . 1 }}
{{- end }}
