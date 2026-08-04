{{- /*
  dmt image-resolution override.

  Render injects this file into the rendered chart's templates/ so offline
  strict rendering never fails on an image whose digest dmt cannot know ahead of
  time: werf-computed names (e.g. kubeProxy134, built per Kubernetes version) or
  werf-defined aliases (e.g. agentDistroless) that have no matching images/
  directory. These defines shadow the real deckhouse_lib_helm helpers — a parent
  chart's define wins over the subchart's — so any image name resolves to a
  stable placeholder reference. dmt lints rendered manifests and never pulls
  images, so a fake-but-well-formed reference is all it needs.
*/ -}}

{{- define "_dmt_image_digest" -}}
sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc
{{- end -}}

{{- define "_dmt_image_ref" -}}
{{- $base := "registry.example.com/deckhouse" -}}
{{- with .Values.global }}{{- with .modulesImages }}{{- with .registry }}{{- with .base }}{{- $base = . }}{{- end }}{{- end }}{{- end }}{{- end -}}
{{- printf "%s@%s" $base (include "_dmt_image_digest" .) -}}
{{- end -}}

{{- define "helm_lib_module_image" -}}
{{- include "_dmt_image_ref" (index . 0) -}}
{{- end -}}

{{- define "helm_lib_module_image_no_fail" -}}
{{- include "_dmt_image_ref" (index . 0) -}}
{{- end -}}

{{- define "helm_lib_module_common_image" -}}
{{- include "_dmt_image_ref" (index . 0) -}}
{{- end -}}

{{- define "helm_lib_module_common_image_no_fail" -}}
{{- include "_dmt_image_ref" (index . 0) -}}
{{- end -}}

{{- define "helm_lib_module_image_digest" -}}
{{- include "_dmt_image_digest" (index . 0) -}}
{{- end -}}

{{- define "helm_lib_module_image_digest_no_fail" -}}
{{- include "_dmt_image_digest" (index . 0) -}}
{{- end -}}
