apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  {{- if .Labels }}
  labels:
    {{- range $key, $value := .Labels }}
      {{ $key }}: "{{ $value }}"
    {{- end }}
  {{- end }}
rules:
{{- range $resource, $verbs := .Rules }}
- apiGroups: [""]
  resources: ["{{ $resource }}"]
  verbs:
  {{- range $verb := $verbs }}
  {{- if $verb.ConfigFlags }}
  - "{{ $verb.Verb }}" # Required when {{ joinConfigFlags $verb.ConfigFlags }}
  {{- else }}
  - "{{ $verb.Verb }}"
  {{- end }}
  {{- end }}
{{- end }}
