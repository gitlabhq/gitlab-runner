module gitlab.com/gitlab-org/gitlab-runner

go 1.23

toolchain go1.23.2

require (
	cloud.google.com/go/compute/metadata v0.5.0
	cloud.google.com/go/iam v1.1.13
	cloud.google.com/go/secretmanager v1.13.6
	cloud.google.com/go/storage v1.43.0
	dario.cat/mergo v1.0.1
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.14.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.7.0
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets v0.11.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.3.2
	github.com/BurntSushi/toml v1.4.0
	github.com/KimMachineGun/automemlimit v0.3.0
	github.com/akeylesslabs/akeyless-go-cloud-id v0.3.5
	github.com/akeylesslabs/akeyless-go/v3 v3.6.3
	github.com/alexflint/go-arg v1.4.3
	github.com/aws/aws-sdk-go-v2 v1.30.4
	github.com/aws/aws-sdk-go-v2/config v1.27.31
	github.com/aws/aws-sdk-go-v2/credentials v1.17.30
	github.com/aws/aws-sdk-go-v2/service/s3 v1.61.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.5
	github.com/bmatcuk/doublestar/v4 v4.4.0
	github.com/creack/pty v1.1.21
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v25.0.6+incompatible
	github.com/docker/distribution v2.8.2+incompatible
	github.com/docker/docker v25.0.6+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.5.0
	github.com/docker/machine v0.7.1-0.20170120224952-7b7a141da844
	github.com/evanphx/json-patch v4.12.0+incompatible
	github.com/fatih/color v1.17.0
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/getsentry/sentry-go v0.13.0
	github.com/go-http-utils/headers v0.0.0-20181008091004-fed159eddc2a
	github.com/golang/mock v1.6.0
	github.com/googleapis/gax-go/v2 v2.13.0
	github.com/gorhill/cronexpr v0.0.0-20160318121724-f0984319b442
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/vault/api v1.12.2
	github.com/in-toto/in-toto-golang v0.9.0
	github.com/invopop/jsonschema v0.12.0
	github.com/jedib0t/go-pretty/v6 v6.4.7
	github.com/johannesboyne/gofakes3 v0.0.0-20240701191259-edd0227ffc37
	github.com/jpillora/backoff v1.0.0
	github.com/kardianos/service v1.2.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/klauspost/compress v1.17.9
	github.com/klauspost/pgzip v1.2.5
	github.com/magefile/mage v1.15.0
	github.com/minio/minio-go/v7 v7.0.70
	github.com/opencontainers/image-spec v1.1.0-rc5
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/client_model v0.3.0
	github.com/prometheus/common v0.37.0
	github.com/prometheus/procfs v0.8.0
	github.com/samber/lo v1.38.1
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/saracen/fastzip v0.1.11
	github.com/sirupsen/logrus v1.9.3
	github.com/sourcegraph/conc v0.3.0
	github.com/stretchr/testify v1.9.0
	github.com/urfave/cli v1.22.14
	gitlab.com/gitlab-org/fleeting/fleeting v0.0.0-20240911165028-a0ce7d6c3260
	gitlab.com/gitlab-org/fleeting/fleeting-artifact v0.0.0-20241018172108-3f6e6586dc5c
	gitlab.com/gitlab-org/fleeting/fleeting/metrics/prometheus v0.0.0-20240911165028-a0ce7d6c3260
	gitlab.com/gitlab-org/fleeting/nesting v0.1.1-0.20230410214021-ab545632193b
	gitlab.com/gitlab-org/fleeting/taskscaler v0.0.0-20241015171421-f3cdb5de12a4
	gitlab.com/gitlab-org/fleeting/taskscaler/metrics/prometheus v0.0.0-20230821135503-23448d122365
	gitlab.com/gitlab-org/gitlab-terminal v0.0.0-20230425133101-519a58790bfd
	gitlab.com/gitlab-org/golang-cli-helpers v0.0.0-20210929155855-70bef318ae0a
	gitlab.com/gitlab-org/labkit v1.17.0
	gitlab.com/gitlab-org/step-runner v0.1.1-0.20241202172711-513f1e22cf67
	go.uber.org/automaxprocs v1.5.2
	gocloud.dev v0.40.0
	golang.org/x/crypto v0.28.0
	golang.org/x/net v0.30.0
	golang.org/x/oauth2 v0.22.0
	golang.org/x/sync v0.8.0
	golang.org/x/sys v0.26.0
	golang.org/x/text v0.19.0
	google.golang.org/api v0.191.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.26.1
	k8s.io/apimachinery v0.26.1
	k8s.io/client-go v0.26.1
	mvdan.cc/sh/v3 v3.9.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	cloud.google.com/go v0.115.0 // indirect
	cloud.google.com/go/auth v0.8.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/internal v0.7.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/ChrisTrenkamp/goxpath v0.0.0-20210404020558-97928f7e12b6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.10.0 // indirect
	github.com/alexflint/go-scalar v1.1.0 // indirect
	github.com/aws/aws-sdk-go v1.55.5 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.3.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.17.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.5 // indirect
	github.com/aws/smithy-go v1.20.4 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bodgit/ntlmssp v0.0.0-20240506230425-31973bb52d9b // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.9.1 // indirect
	github.com/client9/reopen v1.0.0 // indirect
	github.com/containerd/cgroups/v3 v3.0.2 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.1.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20231031074852-3ec07828be7a // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-containerregistry v0.17.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/google/wire v0.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-plugin v1.6.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.8 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.6 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/masterzen/simplexml v0.0.0-20190410153822-31eea3082786 // indirect
	github.com/masterzen/winrm v0.0.0-20240702205601-3fad6e106085 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/spdystream v0.2.1-0.20240621174147-0c1fc43b0e4a // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/oklog/ulid/v2 v2.0.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.3 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/saracen/zipextra v0.0.0-20220303013732-0187cb0159ea // indirect
	github.com/sebest/xff v0.0.0-20210106013422-671bd2870b3a // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.6.0 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20230507112040-c3350d9342df // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tidwall/transform v0.0.0-20201103190739-32f242e2dbde // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/term v0.25.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	golang.org/x/tools v0.24.0 // indirect
	golang.org/x/xerrors v0.0.0-20240716161551-93cc26a95ae9 // indirect
	google.golang.org/genproto v0.0.0-20240812133136-8ffd90a71988 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241007155032-5fefd90f89a9 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
