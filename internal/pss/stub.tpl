{{- /*
Replaces the helpers templates/policies/ takes from admission-policy-engine templates/_helpers.tpl
and from helm_lib, neither of which is synced. They shape the selectors and labels of a constraint;
dmt reads only its kind and parameters.
*/ -}}
{{- define "system_namespaces" }}
- "d8-*"
- "kube-*"
{{- end }}
{{- define "system_namespaces_action" -}}
warn
{{- end }}
{{- define "helm_lib_module_labels" -}}
labels: {{ index . 1 | toJson }}
{{- end }}
{{- /*
Replaces ape.only_ready_constraints: in the cluster it drops constraints whose template is not
ready yet; dmt checks against all of them, so the rendered constraints pass through as is.
*/ -}}
{{- define "ape.only_ready_constraints" }}
{{- index . 1 }}
{{- end }}
