{
  "title": "Templated",
  "links": [{"title": "Docs", "url": "{{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_documentation_uri" (list . "/modules/e2e/faq.html") }}{{ else }}https://deckhouse.io/modules/e2e/tpl.html{{ end }}"}],
  "panels": []
}
