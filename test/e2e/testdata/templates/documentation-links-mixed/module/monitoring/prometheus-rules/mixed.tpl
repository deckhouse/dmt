- name: e2e.documentation-links
  rules:
    - alert: E2EDocumentationLinks
      expr: vector(1)
      annotations:
        fallback: "{{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/ok.html{{ end }}"
        after: https://deckhouse.io/modules/e2e/after.html
        then: "{{ if .Values.global.modules.publicDomainTemplate }}https://deckhouse.io/modules/e2e/then.html{{ end }}"
        and: "{{ if and .Values.global.modules.publicDomainTemplate .Values.foo }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/and.html{{ end }}"
