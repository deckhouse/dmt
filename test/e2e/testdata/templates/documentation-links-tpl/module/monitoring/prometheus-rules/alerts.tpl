- name: e2e.documentation-links
  rules:
    - alert: E2EDocumentationLinks
      expr: vector(1)
      annotations:
        summary: The {{`{{ $labels.node }}`}} node needs attention.
        description: See https://deckhouse.ru/products/kubernetes-platform/documentation/v1/.
