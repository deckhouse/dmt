- name: e2e.documentation-links
  rules:
    - alert: E2EDocumentationLinks
      expr: vector(1)
      annotations:
        helper: {{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}
        inline: "[docs]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/e2e/a.html)"
    {{- if .Values.global.modules.publicDomainTemplate }}
    - alert: E2EBlock
      annotations:
        summary: The {{`{{ $labels.node }}`}} node, see {{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}.
    {{- else }}
    - alert: E2EBlock
      annotations:
        summary: The {{`{{ $labels.node }}`}} node, see https://deckhouse.io/modules/e2e/b.html.
    {{- end }}
    - alert: E2EModuleCheck
      annotations:
        a: "{{ if has "documentation" .Values.global.enabledModules }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/c.html{{ end }}"
        b: "{{ if .Values.global.enabledModules | has "documentation" }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/d.html{{ end }}"
        c: "{{ if not (has "documentation" .Values.global.enabledModules) }}https://deckhouse.io/modules/e2e/e.html{{ end }}"
        d: "{{ if empty .Values.global.modules.publicDomainTemplate }}https://deckhouse.io/modules/e2e/f.html{{ end }}"
        e: "{{ if and .Values.global.modules.publicDomainTemplate (has "documentation" .Values.global.enabledModules) }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/g.html{{ end }}"
        f: "{{ if or (not .Values.global.modules.publicDomainTemplate) (not (has "documentation" .Values.global.enabledModules)) }}https://deckhouse.io/modules/e2e/h.html{{ end }}"
        g: "{{ if .Values.foo.enabled }}x{{ else if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/i.html{{ end }}"
        h: "{{ if .Values.a }}{{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}{{ range .Values.b }}x{{ end }}https://deckhouse.io/modules/e2e/j.html{{ end }}{{ end }}"
