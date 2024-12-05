ubuntu-x86_64-{{ .Revision }}-pwsh
ubuntu-ppc64le-{{ .Revision }}
ubuntu-s390x-{{ .Revision }}
ubuntu-arm64-{{ .Revision }}
ubuntu-arm-{{ .Revision }}
ubuntu-x86_64-{{ .Revision }}
alpine3.16-x86_64-{{ .Revision }}-pwsh
alpine3.17-x86_64-{{ .Revision }}-pwsh
alpine3.18-x86_64-{{ .Revision }}-pwsh
alpine3.21-x86_64-{{ .Revision }}-pwsh
alpine3.16-ppc64le-{{ .Revision }}
alpine3.16-s390x-{{ .Revision }}
alpine3.16-arm64-{{ .Revision }}
alpine3.16-arm-{{ .Revision }}
alpine3.16-x86_64-{{ .Revision }}
alpine3.17-ppc64le-{{ .Revision }}
alpine3.17-s390x-{{ .Revision }}
alpine3.17-arm64-{{ .Revision }}
alpine3.17-arm-{{ .Revision }}
alpine3.17-x86_64-{{ .Revision }}
alpine3.18-ppc64le-{{ .Revision }}
alpine3.18-s390x-{{ .Revision }}
alpine3.18-arm64-{{ .Revision }}
alpine3.18-arm-{{ .Revision }}
alpine3.18-x86_64-{{ .Revision }}
alpine3.19-ppc64le-{{ .Revision }}
alpine3.19-s390x-{{ .Revision }}
alpine3.19-arm64-{{ .Revision }}
alpine3.19-arm-{{ .Revision }}
alpine3.19-x86_64-{{ .Revision }}
alpine3.21-ppc64le-{{ .Revision }}
alpine3.21-s390x-{{ .Revision }}
alpine3.21-arm64-{{ .Revision }}
alpine3.21-arm-{{ .Revision }}
alpine3.21-x86_64-{{ .Revision }}
alpine-latest-ppc64le-{{ .Revision }}
alpine-latest-s390x-{{ .Revision }}
alpine-latest-arm64-{{ .Revision }}
alpine-latest-arm-{{ .Revision }}
alpine-latest-x86_64-{{ .Revision }}
ppc64le-{{ .Revision }}
s390x-{{ .Revision }}
arm64-{{ .Revision }}
arm-{{ .Revision }}
x86_64-{{ .Revision }}
ubi-fips-x86_64-{{ .Revision }}
x86_64-{{ .Revision }}-servercore1809
x86_64-{{ .Revision }}-servercore21H2
x86_64-{{ .Revision }}-nanoserver1809
x86_64-{{ .Revision }}-nanoserver21H2

{{ if .IsLatest }}
ubuntu-x86_64-latest-pwsh
ubuntu-ppc64le-latest
ubuntu-s390x-latest
ubuntu-arm64-latest
ubuntu-arm-latest
ubuntu-x86_64-latest
alpine3.16-x86_64-latest-pwsh
alpine3.17-x86_64-latest-pwsh
alpine3.18-x86_64-latest-pwsh
alpine3.21-x86_64-latest-pwsh
alpine3.16-ppc64le-latest
alpine3.16-s390x-latest
alpine3.16-arm64-latest
alpine3.16-arm-latest
alpine3.16-x86_64-latest
alpine3.17-ppc64le-latest
alpine3.17-s390x-latest
alpine3.17-arm64-latest
alpine3.17-arm-latest
alpine3.17-x86_64-latest
alpine3.18-ppc64le-latest
alpine3.18-s390x-latest
alpine3.18-arm64-latest
alpine3.18-arm-latest
alpine3.18-x86_64-latest
alpine3.19-ppc64le-latest
alpine3.19-s390x-latest
alpine3.19-arm64-latest
alpine3.19-arm-latest
alpine3.19-x86_64-latest
alpine3.21-ppc64le-latest
alpine3.21-s390x-latest
alpine3.21-arm64-latest
alpine3.21-arm-latest
alpine3.21-x86_64-latest
alpine-latest-ppc64le-latest
alpine-latest-s390x-latest
alpine-latest-arm64-latest
alpine-latest-arm-latest
alpine-latest-x86_64-latest
ppc64le-latest
s390x-latest
arm64-latest
arm-latest
x86_64-latest
ubi-fips-x86_64-latest
x86_64-latest-servercore1809
x86_64-latest-servercore21H2
x86_64-latest-nanoserver1809
x86_64-latest-nanoserver21H2
{{ end }}
