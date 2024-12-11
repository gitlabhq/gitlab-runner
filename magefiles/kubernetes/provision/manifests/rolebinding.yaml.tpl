apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    test.k8s.gitlab.com/name: {{ .Name }}
subjects:
- kind: ServiceAccount
  name: {{ .Name }}
  namespace: {{ .Namespace }}
roleRef:
  kind: Role
  name: {{ .Name }}
  apiGroup: rbac.authorization.k8s.io
