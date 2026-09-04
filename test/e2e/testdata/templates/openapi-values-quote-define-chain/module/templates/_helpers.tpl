{{- /*
  These helpers read module .Values directly and are meant to be quoted by their
  callers. A define renders nothing on its own, so the rule must judge the quoting at
  each call site (configmap.yaml), not inside the define body.
*/ -}}

{{- define "e2e-quote-chain.public_domain" -}}
{{ .Values.e2eQuoteChain.publicDomain }}
{{- end -}}

{{- define "e2e-quote-chain.api_domain" -}}
{{ .Values.e2eQuoteChain.apiDomain }}
{{- end -}}

{{- define "e2e-quote-chain.admin_host" -}}
{{ .Values.e2eQuoteChain.adminHost }}
{{- end -}}

{{- /* chain: the outer define includes an inner define that reads .Values directly */ -}}
{{- define "e2e-quote-chain.web_inner" -}}
{{ .Values.e2eQuoteChain.webDomain }}
{{- end -}}
{{- define "e2e-quote-chain.web_domain" -}}
{{ include "e2e-quote-chain.web_inner" . }}
{{- end -}}

{{- define "e2e-quote-chain.metrics_inner" -}}
{{ .Values.e2eQuoteChain.metricsHost }}
{{- end -}}
{{- define "e2e-quote-chain.metrics_host" -}}
{{ include "e2e-quote-chain.metrics_inner" . }}
{{- end -}}
