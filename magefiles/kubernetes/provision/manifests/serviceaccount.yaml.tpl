apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    test.k8s.gitlab.com/name: {{ .Name }}
