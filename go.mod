module gitlab.com/gitlab-org/gitlab-runner

go 1.13

require (
	cloud.google.com/go/storage v1.12.0
	github.com/Azure/azure-storage-blob-go v0.11.1-0.20201209121048-6df5d9af221d
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/bmatcuk/doublestar v1.3.0
	github.com/boumenot/gocover-cobertura v1.1.0
	github.com/containerd/containerd v1.4.3 // indirect
	github.com/docker/cli v20.10.2+incompatible
	github.com/docker/distribution v2.7.0+incompatible
	github.com/docker/docker v20.10.2+incompatible
	github.com/docker/docker-credential-helpers v0.4.1 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-units v0.3.2-0.20160802145505-eb879ae3e2b8
	github.com/docker/machine v0.7.1-0.20170120224952-7b7a141da844
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20190404230743-d7302db945fa
	github.com/getsentry/raven-go v0.0.0-20160518204710-dffeb57df75d
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorhill/cronexpr v0.0.0-20160318121724-f0984319b442
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.3.1-0.20170228224354-599cba5e7b61
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/vault/api v1.0.4
	github.com/imdario/mergo v0.3.7
	github.com/jpillora/backoff v0.0.0-20170222002228-06c7a16c845d
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kardianos/osext v0.0.0-20160811001526-c2c54e542fb7
	github.com/kardianos/service v1.2.0
	github.com/klauspost/compress v1.11.6 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/klauspost/pgzip v1.2.5
	github.com/kr/pty v1.1.5
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/minio-go/v7 v7.0.13
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/gox v1.0.1
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/rs/xid v1.3.0 // indirect
	github.com/saracen/fastzip v0.1.5
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.6.2-0.20200720104044-95a9d909e987
	github.com/tevino/abool v0.0.0-20160628101133-3c25f2fe7cd0
	github.com/urfave/cli v1.20.0
	gitlab.com/ayufan/golang-cli-helpers v0.0.0-20171103152739-a7cf72d604cd
	gitlab.com/gitlab-org/gitlab-terminal v0.0.0-20210104151801-2a71b03b4462
	gocloud.dev v0.21.1-0.20201223184910-5094f54ed8bb
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/net v0.0.0-20210908191846-a5e095526f91
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0
	golang.org/x/text v0.3.7
	gopkg.in/ini.v1 v1.63.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
