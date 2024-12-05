{{ .Revision }}
ubuntu-{{ .Revision }}
alpine3.16-{{ .Revision }}
alpine3.17-{{ .Revision }}
alpine3.18-{{ .Revision }}
alpine3.19-{{ .Revision }}
alpine3.21-{{ .Revision }}
alpine-{{ .Revision }}
ubi-fips-{{ .Revision }}

{{ if .IsLatest }}
ubuntu
alpine3.16
alpine3.17
alpine3.18
alpine3.19
alpine3.21
alpine
ubi-fips
latest
{{ end }}

