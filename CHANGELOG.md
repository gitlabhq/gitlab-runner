## v18.9.0 (2026-02-19)

### New features

- Allow passing `env` and `labels` options to `json-file` Docker logging driver [!5638](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5638) (Patrick Decat @pdecat)
- Enable Job Inputs feature flag by default [!6275](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6275)
- Add CI Jobs to push packages to Pulp [!6073](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6073)
- Retry pulp content push  commands on specific errors [!6197](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6197)
- Instrument input interpolations [!6047](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6047)
- Add support for Google Cloud Service universe domain [!6338](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6338)
- Upgrade step-runner to version 0.24.0 [!6056](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6056)
- Push runner linux packages to Pulp [!6062](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6062)
- Add user agent to AWS Secrets Manager integration [!6060](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6060) (derikwang @derik01)
- Script function [!6029](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6029)
- Filter out obsolete distro releases [!6042](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6042)
- Pass job timeout in steps RunRequest so server can also enforce job timeouts [!6375](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6375)
- Implement user script to step [!6069](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6069)
- Helpers/retry: interrupt backoff sleep on context cancellation [!6061](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6061) (Emmanuel 326 @Emmanuel326)
- Update the libvirt use doc [!6034](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6034) (Funning @FunningC0217)
- Add zos build tags [!5835](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5835) (Joon Lee @jlee_ibm)
- Avoid interpolation without defined job inputs [!6374](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6374)
- Job Router client - WebSocket support [!6020](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6020)
- Upgrade step-runner to version 0.26.0 [!6351](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6351)

### Security fixes

- Update RUNNER_IMAGES_VERSION to 0.0.34 [!6066](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6066)

### Bug fixes

- Pass S3 session token for access key credentials [!6376](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6376)
- Fix FD exhaustion during retry requests [!6041](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6041)
- Fix proxy_exec secret masking permissions [!6044](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6044)
- Update pkcs7 library [!6016](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6016)
- Refactor Connector to allow setup before connection [!6359](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6359)
- Fix WebSocket tunnel data race [!6237](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6237)
- Avoid breaking change when script syntax is invalid when no inputs used [!6417](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6417)
- Add service container ID hostname when emulating links functionality [!6043](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6043)
- Runner_wrapper: fix backoff retry context cancellation handling [!6064](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6064) (Emmanuel 326 @Emmanuel326)
- Update the logic for comparing the urls and tokens [!6296](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6296) (Aayush @Aayush-Saini)
- Ensure check_interval takes effect and eliminate race condition between fleet of runners [!6081](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6081) (Pishel65 @pishel65)
- Guard against nil FileInfo in filepath.Walk [!6050](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6050) (Bob Singh @bobsingh.dev)

### Maintenance

- Adds MR Review instructions focused around Log Field Standardisation [!6353](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6353)
- Rename job router RPC package [!6049](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6049)
- Update linting configuration from GitLab project [!6352](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6352)
- De-duplicate kube warning events [!5926](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5926)
- Add command to sync go version in other files [!6378](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6378)
- Bump Go to 1.25.7 and RUNNER_IMAGES_VERSION to 0.0.35 [!6370](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6370)
- Update crosslink pointing to docs [!6346](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6346)
- Correct runner linux package archs [!6038](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6038)
- Add status check to launchctl I/O error troubleshooting [!6358](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6358)
- Add mage to project dependencies [!6348](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6348)
- Move JobResponse to spec.Job [!6058](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6058)
- Update go packages [!6032](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6032)
- Switch to a maintained YAML library go.yaml.in/yaml/v3 [!6065](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6065)
- Move versions to variables [!6368](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6368)
- Add UniverseDomain configuration for GCS cache [!6362](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6362)
- Network: stop retry backoff timer on context cancellation [!6063](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6063) (Emmanuel 326 @Emmanuel326)
- Clarify Bash requirement for GitLab Runner shell executor on macOS [!6350](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6350)
- Fix miscellaneous Markdown formatting issues [!6347](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6347)
- Update redirecting links [!6327](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6327)
- Fix supported distros documentation [!6048](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6048)
- Kubernetes executor GPU configuration requirements [!6077](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6077)
- Use chunk size of 10MB for pulp uploads [!6078](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6078)
- Fix headers passed when using CI_JOB_TOKEN [!6075](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6075)
- Rename GITLAB_TOKEN to GITLAB_TEST_TOKEN in test utilities [!6045](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6045) (Aayush @Aayush-Saini)
- AI Translated Documentation Push: GITTECHA-544 [!6360](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6360)
- Include runner_name in all relevant log lines [!5883](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5883)
- Add pod/container name to build logger fields [!5891](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5891)
- Add diagnostics logging for S3 cache AssumeRole operations [!6345](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6345)
- Remove EOL spaces in doc files - 2026-01-28 [!6326](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6326)
- Incorporate additional change from GitLab project [!6357](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6357)
- Document emulated docker links caveats [!6054](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6054)
- Restore environment variables to build container [!6333](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6333)

## v18.8.0 (2026-01-15)

### New features

- Job Router [!5945](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5945)
- Implement mage pulp:supportedOSVersions  target [!6024](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6024)
- Improve Portability of Git Version Check for z/OS [!6001](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6001) (Kai McGregor @kmcgreg-ibm)
- Introduce better job inputs interpolation error [!6014](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6014)
- Emulate deprecated Docker links functionality with ExtraHosts [!5980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5980)
- Mage target to create Pulp CLI configuration [!6039](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6039)

### Bug fixes

- Ensure buildlogger uses available masks (issue reported by Christian Sousa from Blue Origin Manufacturing, LLC) [!5909](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5909)
- Support Git submodules with different hosts via RepoURL insteadOf [!6025](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6025)
- Add IPv6 address when emulating links functionality [!6027](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6027)
- Cleanup dangling virtualbox resources [!5941](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5941)
- Add `-protected` suffix to docker cache volumes if any of the cache keys include the `-protected` suffix [!6021](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6021)
- Fix connector interface not being exposed for docker+machine and docker-autoscaler executors [!6015](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6015)

### Maintenance

- Fix service container log collection wait time [!6019](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6019)
- Fix race condition in TestDockerCommandWithRunnerServiceEnvironmentVariables [!6018](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6018)
- Fix a typo in the GitLab Runner system requirements page [!6031](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6031)
- Fix privileged setting for general Podman usage [!6023](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6023)
- Update Golang to 1.25.3 [!5978](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5978)
- Make Alpine 3.21 the default base for helper images [!5995](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5995)
- docs: Replace `curl | bash` commands with safer steps [!6036](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6036) (Yasssmiine @Yasssmiine-x)
- Remove alpine 3.19 [!5993](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5993)
- Restructure GitLab Runner installation documentation with card-based navigation [!6030](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6030)
- Clarify details about arm helper image, cleanup extra wording, fix link [!6012](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6012) (Ben Bodenmiller @bbodenmiller)
- Allow the i18n lint paths job to fail [!6017](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6017)

## v18.7.2 (2026-01-08)

### Bug fixes

- Support Git submodules with different hosts via RepoURL insteadOf [!6025](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6025)

## v18.7.1 (2025-12-23)

### Bug fixes

- Add IPv6 address when emulating links functionality [!6027](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6027)

## v18.7.0 (2025-12-18)

### New features

- Add reservation throttling config option [!6010](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6010)
- Introduce first iteration of job inputs interpolation behind FF [!5855](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5855)
- Emulate deprecated Docker links functionality with ExtraHosts [!5980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5980)

### Bug fixes

- Fix connector interface not being exposed for docker+machine and docker-autoscaler executors [!6015](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6015)
- Do not fail install if gitlab-runner service commands not available [!5948](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5948)
- Fix shell executor not working with variables that use file variables [!5958](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5958)
- Configure submodules to inherit parent repository credentials [!5962](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5962)
- Fix "unable to get password from user" errors in shell executor [!5961](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5961)
- Fix handling of relative builds dir [!5977](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5977)
- Ensure buildlogger uses available masks (issue reported by Christian Sousa from Blue Origin Manufacturing, LLC) [!5909](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5909)
- Fix clear-docker-cache script for Docker 29 [!5969](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5969)
- Fix bash shell cleanup to support variable expansion in paths [!5966](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5966)
- Ignore user-defined AWS_PROFILE variable in cache uploads [!5986](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5986)
- Fix misleading retry message when GET_SOURCES_ATTEMPTS=1 [!5998](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5998)
- Support resolving Windows 10.0.26200 helper image [!5984](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5984)

### Maintenance

- Add dashboard generation process and usage guidance [!5989](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5989)
- Remove CertificateDirectory global [!5956](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5956)
- Docs metadata update for group change from Deploy/Environments -> Verify/Runner Core [!5955](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5955)
- Bump golang.org/x/crypto [!5991](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5991)
- Enable log timestamps by default [!5861](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5861)
- AI Translated Documentation Push: GITTECHA-373 [!5934](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5934)
- Bump gitlab.com/gitlab-org/fleeting/taskscaler to pull a fix [!5999](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5999)
- Fix flaky tests [!5994](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5994)
- Regenerate mocks [!5974](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5974)
- Bump runner image version [!6007](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6007)
- Recursively set up Git submodules credentials [!5997](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5997)
- Roll documentation linting tool versions forward [!5954](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5954)
- Shorten headers to avoid markdownlint exclusion code (Runnner) [!5951](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5951)
- Use CI_RUNNER_VERSION for arm helper image [!6004](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6004) (Ben Bodenmiller @bbodenmiller)
- Bump step-runner to v0.20.0 [!5970](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5970)
- AI Translated Documentation Push: GITTECHA-371 [!5932](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5932)
- Fix typo of libvirt [!5953](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5953)
- Remove EOL spaces in doc files - 2025-11-17 [!5952](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5952)
- Fix service container log collection wait time [!6019](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6019)
- Document architecture support in `gitlab-runner-helper-images` package [!5976](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5976)
- Update GitLab Runner developer docs [!5853](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5853)
- Ensure `stable docker images` only runs after all tests pass [!5990](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5990)
- Update Kubernetes client-go library to 0.32.10 [!5929](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5929) (Stéphane Talbot @stalb)
- Refactor commands, construct a single GitLab client in a single place [!5950](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5950)
- Fix race condition in TestDockerServiceHealthcheckOverflow [!5985](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5985)
- Tidy go mod [!5973](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5973)
- AI Translated Documentation Push: GITTECHA-375 GITTECHA-420 [!5938](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5938)
- Docs(docker-machine): update docs [!6006](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6006)
- Fix a teeny-tiny typo in runner `common/config.go` [!5967](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5967) (Sadra Barikbin @s.barikbin)
- Misc refactors [!5949](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5949)
- Include go.mod changes when checking modules in pipeline [!5975](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5975)
- Fix toml spacing inconsistencies in k8s runner docs [!6003](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6003) (Ben Bodenmiller @bbodenmiller)
- Update instructions after UI redesign [!6000](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6000)
- Warn users about legacy /ci URL suffix in runner configuration [!5988](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5988)
- Update to Go 1.24.11 [!5992](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5992)
- Update supported OS distro/version docs [!5959](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5959)
- Fix flaky TestCredSetup with -race by removing CI_DEBUG_TRACE [!5987](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5987)
- AI Translated Documentation Push: GITTECHA-372 [!5933](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5933)
- Allow overriding git credentials in shell integration tests [!5982](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5982)

### Documentation changes

- Clarify documentation describing configuration for AWS ASGs with Docker autoscaler [!5996](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5996) (Dan Puttick @dan_oklo)

## v18.6.6 (2025-12-09)

### Bug fixes

- Ignore user-defined AWS_PROFILE variable in cache uploads [!5986](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5986)

### Maintenance

- Fix flaky tests [!5994](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5994)
- Update to Go 1.24.11 [!5992](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5992)
- Ensure `stable docker images` only runs after all tests pass [!5990](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5990)

## v18.6.4 (2025-12-05)

### Bug fixes

- Fix handling of relative builds dir [!5977](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5977)

## v18.6.3 (2025-11-28)

### Bug fixes

- Configure submodules to inherit parent repository credentials [!5962](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5962)
- Fix bash shell cleanup to support variable expansion in paths [!5966](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5966)

## v18.6.2 (2025-11-25)

### Bug fixes

- Fix "unable to get password from user" errors in shell executor [!5961](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5961)

## v18.6.0 (2025-11-17)

### New features

- Functions subcommands in runner binary [!5875](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5875)
- Add namespace support to GitLab Secrets Manager [!5918](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5918)
- Disallow shim execution mode if executor supports native steps execution [!5898](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5898)
- Add MachineOptionsWithName configuration option [!5920](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5920)
- Add slot-based cgroup support for Docker executor [!5870](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5870)
- Add LoongArch (loong64) build support [!5800](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5800) (Mingcong Bai @MingcongBai)
- Bootstrap gitlab-helper-binary for Docker [!5892](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5892)
- Add correlation_id to "Update job..." log line [!5887](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5887)

### Bug fixes

- Retry etcd request timeout error in Kubernetes executor [!5877](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5877)
- Always pass as a file for custom executor [!5904](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5904)
- Handle unexpected panics in trace buffer [!5890](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5890)
- Fix removing files recursively for bash on z/OS [!5623](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5623) (Kai McGregor @kmcgreg-ibm)
- Externalize git configuration [!5912](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5912)
- Fix job logs duplicating as service logs [!5863](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5863) (Markus Kaihola @makeri89)
- Expand variables in `image.docker.platform` before pulling images [!5897](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5897) (Bert Wesarg @bertwesarg)

### Maintenance

- Update Vale rules from GitLab project [!5884](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5884)
- AI Translated Documentation Push: GITTECHA-374 [!5935](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5935)
- Add troubleshooting docs for services on windows k8s executor [!5913](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5913) (Erik Petzold @erik.petzold1)
- Move build execute prepare/user scripts to their own functions [!5893](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5893)
- Docs: Add note for PowerShell versions in the custom executor [!5894](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5894)
- Steps execution via Connect() [!5927](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5927)
- Clarify docs for supported caching feature [!5910](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5910)
- Pull in fixes for CVEs [!5895](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5895)
- Translation Push - All - For English Anchor LInks [!5896](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5896)
- Integration Tests for GCP Secrets Manager [!5881](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5881)
- Use passed context in NewStepsDocker.Exec() [!5915](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5915)
- Remove obsolete code [!5902](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5902)
- Build linux/riscv64 platform for registry.gitlab.com/gitlab-org/gitlab-runner image [!5923](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5923) (Ludovic Henry @luhenry)
- docs(docker.md): add podman selinux mcs section [!5879](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5879) (vtardiveau @vtardiveau)
- Verify all: only use creds for local images [!5914](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5914)
- Update cache S3 SSE Key ID docs [!5919](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5919)
- Add warning regarding podman and GPUs [!5937](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5937)
- AI Translated Documentation Push: GITTECHA-370 [!5917](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5917)
- Fix typo [!5924](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5924)
- Make structured "job finished" log line with failure_reason and exit_code [!5885](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5885)
- [steps] Deflake steps command tests [!5905](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5905)
- Add s3:ListBucket to the required permissions for IAM role to access S3 bucket [!5903](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5903)
- docs: Update docker-machine version [!5899](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5899)
- Add link to UI redesign doc [!5925](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5925)
- Document error when performing sts:AssumeRoleWithWebIdentity for s3 cache [!5921](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5921)
- Minor copy edits in runner docs [!5944](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5944)
- Add link checking to i18n docs linting [!5943](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5943)
- Document Kubernetes CI [!5786](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5786)
- Copy edits to runner docs [!5911](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5911)
- Pull in some changes from the security fork [!5906](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5906)
- Use RFC3339Nano timestamp format for JSON logs [!5888](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5888)
- Remove curly brackets from example [!5942](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5942)
- chore: refactor TestAttach using canonical client [!5838](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5838) (Muhammad Daffa Dinaya @mdaffad)

## v18.5.0 (2025-10-13)

### New features

- Add Kubernetes context support for executor [!5859](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5859)
- Add label support to runner configuration [!5802](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5802)
- Implement minimal job confirmation API [!5843](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5843)
- Update Usage Log with more job context [!5869](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5869)
- Add project name to build logging fields [!5846](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5846)

### Bug fixes

- Fix logging of duration_s field [!5874](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5874)
- Remove duplicate prefix in docker service containers [!5840](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5840)

### Maintenance

- Refactor autoscaler provider for readability and update dependencies [!5807](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5807) (Sven Geisler @sge70)
- Remove EOL spaces in doc files - 2025-10-07 [!5873](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5873)
- Latest Translation Yaml enhancements [!5842](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5842)
- Reduce over-linking in GitLab Runner registration documentation [!5834](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5834)
- Improve branch selection logic for docs:check Hugo build job [!5866](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5866)
- Update Hugo version for Docs test [!5852](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5852)
- Update the ubuntu version used as a base image [!5845](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5845)
- Docs feedback: Add more context for Parallels executor [!5878](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5878)
- Rename Connect() to TerminalConnect() [!5880](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5880)
- Improve error logging in docker-machine executor [!5862](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5862)
- Update docker device documentation link [!5833](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5833) (Quentin MICHAUD @mh4ckt3mh4ckt1c4s)
- Add VMware vSphere to community maintained fleeting plugins [!5818](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5818) (Santhanu V @santhanuv)
- Update fleeting plugin and other dependencies [!5830](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5830)
- Clean up docs redirects - 2025-09-25 [!5847](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5847)
- Fix log field name for docker machine executor [!5860](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5860)
- Docs feedback: Clarify SSH `StrictHostKeyChecking` default behavior [!5871](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5871)
- Fix duplicate test cases [!5857](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5857)
- OKR: Reduce over-linking in GitLab Runner manual installation guide [!5854](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5854)
- Chore: rename VersionInfo to Info [!5849](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5849)
- Upgrades taskscaler for slot info on no capacity [!5872](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5872)
- Docs feedback: Make the executor selection workflow diagram readable [!5876](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5876)
- Integration Tests for AWS Secrets Manager [!5841](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5841)
- Group/stage change: Verify/Runner -> Verify/Runner Core and CI Functions Platform in the GitLab Runner project [!5858](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5858)

## v18.4.0 (2025-09-12)

### New features

- Add support for GIT_CLONE_EXTRA_FLAGS for native git clone [!5809](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5809) (Sven Geisler @sge70)
- Cache keys can be hashed [!5751](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5751)
- Update step-runner version to 0.16.0 [!5825](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5825)

### Bug fixes

- Fix arch label for IBM PPC arch [!5827](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5827)
- Make docker volumes really unique [!5783](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5783)
- Fix cache key sanitation issues, esp. re. "cache key files" [!5741](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5741)
- Update fleeting plugin dependency [!5784](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5784)
- [docker] Separate cache volumes for builds against protected and unprotected [!5773](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5773)
- Add new ruleset to cover all docs patterns [!5832](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5832)
- Upgrade base images to v0.0.26 [!5829](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5829)
- Ensure TOML feature flags are used and take precedence over job env [!5782](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5782)
- Remove health check from Vault client call [!5803](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5803)
- Add timeouts to all docker-machine command executions [!5789](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5789)
- Handle config concurrency deadlock with warnings and documentation [!5759](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5759)

### Maintenance

- Update docs links [!5814](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5814)
- Document community supported plugins [!5532](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5532)
- Remove EOL spaces in doc files - 2025-08-26 [!5804](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5804)
- Bump golang to 1.24.6 [!5796](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5796)
- Implement Kubernetes allowed users/groups in Runner config [!5724](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5724)
- Update fleeting plugin and other dependencies [!5823](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5823)
- Update API metric description to bring consistency [!5779](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5779)
- Add CI to test Hugo build with translated documentation [!5806](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5806)
- Add path verifications for localized files - Runner [!5790](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5790)
- Refactor registering of commands to be more explicit [!5816](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5816)
- Make the `default` helper alpine flavour point to `latest` [!5768](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5768)
- Use Hugo 0.148.2 for docs builds [!5815](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5815)
- Enable static checks [!5811](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5811)
- Fix nanosecond padding of timestamps [!5799](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5799) (Philipp Hahn @pmhahn)
- Bump RUNNER_IMAGES_VERSION to 0.0.25 [!5794](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5794)
- Revert changes made by Auto Releaser Bot [!5795](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5795)
- Migrate golangci lint to version 2 [!5772](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5772)
- Cross reference a KB article in the concurrency and limit docs [!5785](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5785)
- CI: Skip downloading artifacts of previous jobs [!5808](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5808) (Philipp Hahn @pmhahn)
- Use testing linter with t.Context related settings [!5812](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5812)
- Add support for signing and notarizing macOS binaries [!5792](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5792)
- Change option signature to not return error [!5775](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5775)
- Fix flaky unit test with TestDefaultDocker_Exec [!5798](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5798)
- Display seconds since epoch using a more widely supported method [!5736](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5736) (Kai McGregor @kmcgreg-ibm)
- Fix non-semantic linking word [!5801](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5801)
- Fix flaky Docker integration tests [!5797](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5797)

## v18.3.1 (2025-09-04)

### Bug fixes

- Remove health check from Vault client call [!5803](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5803)

## v18.3.0 (2025-08-21)

### New features

- Add native GitLab Secrets Manager support to GitLab Runner [!5733](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5733)
- Add method label to status counter metrics [!5739](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5739)
- Add status_class and method label to request duration metrics [!5752](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5752)
- Update step-runner version to 0.15.0 [!5757](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5757)
- Record request retries. [!5758](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5758)

### Bug fixes

- Update fastzip to v0.2.0 [!5778](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5778)
- Fix identity for aws_secrets_manager_resolver [!5747](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5747)
- Consume docker auth info in order [!5686](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5686)
- [docker] Separate cache volumes for builds against protected and unprotected [!5773](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5773)
- Add correlation id header to outgoing requests [!5743](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5743)
- Add support for 503 http code when the GitLab instance is in maintenance mode [!5685](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5685)
- Enable image executor opts in the kubernetes executor [!5745](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5745)
- Fix job duration reporting [!5711](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5711)
- Update fleeting plugin dependency [!5776](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5776)
- Parse the last line of stdout for UID/GID [!5765](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5765)
- Fix proxy-exec store temporary directory [!5780](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5780)
- Fix cache key sanitation issues, esp. re. "cache key files" [!5741](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5741)
- Tighten cache key sanitation [!5719](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5719)

### Maintenance

- Add operator pod_spec and deployment_spec docs [!5766](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5766)
- Remove unused lock from client struct [!5770](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5770)
- Mention that systempaths security_opt is not supported [!5769](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5769) (Andrés Delfino @andresdelfino)
- Change link to GA issue for the overwrite pod spec feature [!5732](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5732)
- Update a few region/zone IDs in examples [!5720](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5720)
- Add a max age of 24h for Kubernetes integration RBAC resources [!5760](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5760)
- Chore: Use stable alpine for RISC-V [!5714](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5714) (Aaron Dewes @AaronDewes)
- Allow customization of taskscaler and fleeting parameters in config.toml [!5777](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777) (Sven Geisler @sge70)
- Move backoff retry logic to retry requester [!5754](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5754)
- Refactor gitlab client unregister runner to table tests [!5670](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5670)
- Fix localization codeowners [!5712](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5712)
- Lbhardwaj/refactor/unregister command methods [!5742](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5742)
- Refactor move retry 429 status code logic to one place [!5727](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5727)
- Add test for abstract shell guardGetSourcesScriptHooks method [!5702](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5702)
- Sync vale rules from main project - Runner [!5753](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5753)
- Refactor verify runner tests to table tests and better assertions [!5763](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5763)
- Bridge releases with Hosted Runners [!5746](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5746)
- Cleanup dead code related to disabled Akeyless secrets integration feature [!5762](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5762)
- Drop Alpine Version 3.18 [!5744](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5744)
- Refactor kubernetes feature checker tests [!5774](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5774)
- Remove EOL spaces in docs [!5749](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5749)
- Remove line length rule for markdownlint for i18n files [!5723](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5723)
- Fix minor typos with executor interface docs [!5717](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5717)
- Correct erroneous compatibility chart features for docker-autoscaler [!5755](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5755)
- Docker machine AMI update [!5718](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5718)
- Add errorlint linter to golangci-lint settings [!5750](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5750)
- Introduce unnecessary-traversal Markdownlint rule to Runner docs [!5735](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5735)
- Upgrade prebuilt runner images back to Alpine 3.21 [!5730](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5730) (Aaron Dewes @AaronDewes)
- Upstream batch push 2025-07-21 [!5734](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5734)
- Refactor errors to wrap errors [!5731](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5731)
- Minor grammar updates in GitLab Runner README [!5756](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5756) (Anshi Mehta @anshikmehtaa)
- Unregister command unit tests [!5738](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5738)
- A bit of general copy edit cleanup [!5740](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5740)
- Update index file for getting started [!5722](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5722)
- Update hosted runners bridge wiki entry [!5767](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5767)
- Minor improvements to runner fleet scaling best practices doc [!5737](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5737)
- Add a note about the experiment status of GRIT [!5729](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5729)

## v18.2.0 (2025-07-12)

### New features

- Add reference to z/OS on the main runner install page [!5647](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5647)
- Thread job request correlation ID to git operations [!5653](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5653)
- Add functionality to retrieve secrets from AWS SecretsManager [!5587](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5587) (Markus Siebert @m-s-db)
- Update docs on how to use env variables for S3 cache access [!5648](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5648)
- Improve runner_name metric label coverage [!5609](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5609) (Josh Smith @jsmith25)
- Log usage of deault image for Docker and K8S executors [!5688](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5688)

### Bug fixes

- Tighten cache key sanitation [!5719](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5719)
- Add troubleshooting guide for GCS workload identity [!5651](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5651)
- Skip pre and post checkout hooks for empty [!5677](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5677)
- Docs: Fix protocol_port default for SSH [!5701](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5701)
- Revert MRs 5531 and 5676 [!5715](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5715)
- Reimplement ShortenToken [!5681](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5681)
- Put the fips binary in the fips runner image [!5669](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5669)
- Set `helper_image_flavor` to `ubi-fips` when fips mode is enabled [!5698](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5698)
- Ensure BuildErrors have FailureReason [!5676](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5676)
- Fix kubernetes executor helper image override log [!5655](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5655) (Ricard Bejarano @ricardbejarano)
- Add `-depth` option to `find -exec rm` invocations [!5692](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5692)

### Maintenance

- Add known SELinux issue regarding tmp and pip to the Podman docs [!5661](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5661)
- Switch jobs to Kubernetes [!5631](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5631)
- Updated the documents [!5596](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5596) (Jithin Vijayan @jithin.vijayan)
- Update docker machine versions [!5672](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5672)
- Add autoscaler IP address logging options [!5519](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519) (Brayden White @bwhite117)
- Docs: Improve prometheus scraping metrics docs including Operator [!5657](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5657)
- Update default ruby version in examples [!5693](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5693)
- Add aardvark-dns bug notice to podman guide [!5689](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5689) (Felix @f.preuschoff)
- Update region from us-central-1 to eu-central-1 [!5713](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5713)
- Run fewer vale lint rules on i18n (translation) docs MRs [!5699](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5699)
- Add additonal info about SHA-pinned images [!5700](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5700)
- Bump golang to 1.24.4 [!5668](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5668)
- chore: refactor TestAttachPodNotRunning [!5650](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5650) (Muhammad Daffa Dinaya @mdaffad)
- Update access a private registry from kubernetes executor [!5622](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5622)
- Add configure runner on OCI to the index page [!5649](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5649)
- Clean up runner docs [!5697](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5697)
- Fix OS version package support docs [!5703](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5703)
- Add .markdownlint-cli2.yaml for doc-locale [!5690](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5690)
- Fix this test [!5682](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5682)
- Update mockery to latest version 3.3.4 and generate mocks [!5646](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5646)
- Remove outdated information [!5691](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5691)
- Update file _index.md [!5665](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5665)
- Update SLSA build type documentation [!5639](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5639)
- Document tarzstd as an argument for CACHE_COMPRESSION_FORMAT [!5673](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5673)
- Clarify documentation on reading S3 credentials from the environment [!5671](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5671)
- Add Kubernetes executors docs for helper container memory sizing [!5659](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5659)
- Runner doc restructuring: Revamp the admin section index page [!5678](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5678)
- Add formatting target for easy fixes with golangci-lint [!5658](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5658)
- Add overprovisioning note, fix typos [!5656](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5656)
- Refactor gitlab client request job tests to table tests [!5666](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5666)
- Fix indent in config.toml example. [!5667](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5667) (Mathieu Gouin @mathieugouin)
- Update PowerShell UTF8 integration test [!5493](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5493) (Guillaume Chauvel @guillaume.chauvel)
- Update step-runner version to 0.13.0 [!5705](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5705)
- Fix docs pipelines for forks [!5664](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5664)
- Small typo fixes [!5652](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5652)

## v18.1.0 (2025-06-19)

### New features

- Added safety checks for nil sessions and empty endpoints [!5515](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5515) (Zubeen @syedzubeen)
- Log a different message for policy jobs with highest precedence [!5628](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5628)
- Add adaptive request concurrency [!5546](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5546)
- Allow to install/manipulate the gitlab-runner service as a user service (systemd) [!5534](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5534) (Tiago Teixeira @tiago.teixeira.erx)
- Bump base images to allow native clone to work [!5561](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5561)
- Support user as integer for Docker/Kubernetes executor_opts [!5552](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5552)
- Thread job request correlation ID to git operations [!5653](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5653)
- make preemptive mode configurable [!5565](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5565) (Pascal Sochacki @pascal.sochacki)
- Add queue_size and queue_depth metrics [!5592](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5592)
- Log policy job information [!5591](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5591)
- Add more request_concurrency related metrics [!5558](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5558)
- Enable powershell native clone [!5577](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5577) (Guillaume Chauvel @guillaume.chauvel)
- Add support for Overlay Volume Mounts when Podman is used with Docker Executor [!5522](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5522) (Napuu @napuu)

### Bug fixes

- Stop following symlinks when archiving documents [!5543](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5543)
- Docker+autoscaler: Properly clean up when a job times out or is cancelled [!5593](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5593)
- Fix AWS GovCloud with AWS S3 cache [!5613](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5613)
- Fix final job duration handling [!5583](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5583)
- Document DinD DNS behavior with network-per-build feature [!5611](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5611)
- Run git config cleanup before creating the template dir [!5598](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5598)
- Fix authentication towards HTTP docker registries [!5329](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5329) (François HORTA @fhorta1)
- Switch the default for FF_GIT_URLS_WITHOUT_TOKENS back to false [!5572](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5572)
- Handle the new glrtr- prefix [!5580](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5580)

### Maintenance

- Update golang.org/x/net to fix CVE-2025-22872 [!5594](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5594)
- Added missing commas [!5579](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5579)
- Docker executor image clarification and macOS virtualization info [!5571](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5571) (Charles Uneze @network-charles)
- Fix rules for the unit test job [!5618](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5618)
- Tidy runner_wrapper/api [!5604](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5604)
- Limit Unit test job to only MR pipelines for forks [!5608](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5608)
- Making timeout to acquire a new instance configurable within gitlab-runner [!5563](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5563) (Moritz Scheve @schevmo)
- Remove outdated information [!5620](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5620)
- Add correlation_id to request logs [!5615](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5615)
- Bump base-images to 0.0.18 [!5633](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5633)
- Fix pipelines for forks [!5607](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5607)
- Update redirected links [!5605](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5605)
- Improve pipelines for community, fork, docs MRs [!5576](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5576)
- Fix protoc binary download for macos [!5570](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5570)
- Document how to install GitLab Runner on z/OS manually [!5641](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5641)
- Upgrade Go to v1.24.3 [!5562](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5562)
- Clean up stray whitespace [!5585](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5585)
- Remove kaniko references in GitLab Runner docs [!5560](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5560)
- Update step-runner dependency version to 0.11.0 [!5645](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5645)
- Update dates in examples [!5621](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5621)
- Clean up tables in misc runner docs [!5589](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5589)
- Docs: more bold cleanup [!5586](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5586)
- Document how to set environment variables in GitLab Runner Helm chart [!5559](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5559)
- Restrict danger-review to canonical GitLab forks [!5640](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5640)
- Push GitLab Documentation Translations Upstream [2025-06-09] [!5630](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5630)
- docs: Add custom executor "shell" property [!5578](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5578) (Guillaume Chauvel @guillaume.chauvel)
- Remove randomness of TestProcessRunner_BuildLimit failures [!5588](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5588)
- Run Hugo build test on the correct Docs branch [!5545](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5545)
- Install local dev tools and dependency binaries in on go [!5632](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5632)
- chore: install tool binaries in tmp bin [!5629](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5629)
- docs: Remove a line that is not accurate to the current usages of GRIT [!5601](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5601)
- Config options refactor [!5373](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5373)
- Move internal docs into development directory [!5595](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5595)
- Update CHANGELOG to take in account 17.10.x to 18.0.x releases [!5643](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5643)
- Update docker machine in docs [!5603](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5603)
- Remove outdated mention [!5582](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5582)
- Added Experimental Status [!5602](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5602)
- Remove outdated content [!5597](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5597)
- Use mockery constructors in tests [!5581](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5581)
- Auto-format all remaining runner tables [!5584](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5584)
- Update the cntlm link to the new fork [!5556](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5556) (Dan Fredell @DFredell)
- Update docker-machine version in docs [!5617](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5617)

## v18.0.3 (2025-06-11)

### Bug fixes

- Fix AWS GovCloud with AWS S3 cache [!5613](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5613)
- Run Git config cleanup before creating the template dir [!5598](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5598)

### Maintenance

- Remove randomness of TestProcessRunner_BuildLimit failures [!5588](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5588)

## v17.11.3 (2025-06-11)

### Bug fixes

- Fix AWS GovCloud with AWS S3 cache [!5613](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5613)

## v17.10.2 (2025-06-11)

### Bug fixes

- Fix AWS GovCloud with AWS S3 cache [!5613](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5613)

## v17.11.2 (2025-05-22)

### Bug fixes

- Handle the new glrtr- prefix [!5580](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5580)
- Fix final job duration handling [!5583](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5583)

## v18.0.2 (2025-05-21)

### Bug fixes

- Handle the new glrtr- prefix [!5580](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5580)
- Fix final job duration handling [!5583](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5583)

## v18.0.1 (2025-05-16)

### Bug fixes

- Switch the default for FF_Git_URLS_WITHOUT_TOKENS back to false [!5572](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5572)

## v18.0.0 (2025-05-15)

### New features

- Add exponential backoff to execute stage retries [!4517](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4517)
- Add support for uid:gid format for Kubernetes executor options [!5540](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5540)
- Add adaptive request concurrency [!5546](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5546)
- Add more request_concurrency related metrics [!5558](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5558)
- Suppress unnecessary warnings when Kubernetes user values are empty [!5551](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5551)
- Shells: Implement the use of Git-clone(1) again [!5010](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5010)
- Adding How To Configure PVC Cache [!5536](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5536)
- Improve runner build failure reasons [!5531](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5531)

### Bug fixes

- Add support for submodules in the exec command [!75](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/75) (Lucas @fresskoma)
- Reimplement pull-policy validation [!5514](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5514)
- Update fleeting dependency [!5535](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5535)
- Add Cloud provider error message details for cache upload failures to cloud storage targets [!5527](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5527)

### Maintenance

- FF_Git_URLS_WITHOUT_TOKENS defaults to true [!5525](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5525)
- Wait for MR image before starting runner incept [!5528](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5528)
- Remove outdated mentions [!5510](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5510)
- Update markdownlint for JP Docs & Push Translations [!5547](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5547)
- Remove GitHub.com/Docker/machine library dependency [!5554](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5554)
- Promote FF_RETRIEVE_POD_WARNING_EVENTS to a config print_pod_warning_events [!5377](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5377)
- Deprecate ServiceAccountName [!5523](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5523)
- Create Japanese documentation directory [!5513](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5513)
- Bump base images to address CVE-2024-8176 [!5518](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5518)
- Retry packagecloud 504 errors [!5520](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5520)
- Remove outdated registration in test script [!5511](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5511)
- Use Hugo 0.145.0 for docs builds [!5521](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5521)
- Sync vale rules from main repo [!5549](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5549)
- Remove section referring to unapplied breaking change [!5529](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5529)
- Add recommendation to read Readme for plugin before installing [!5530](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5530)
- Add troubleshooting section for AZRebalance issue [!5494](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5494)
- Upgrade taskscaler dependency with updated heartbeat functionality [!5553](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5553)
- Update GPG key expiry date [!5539](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5539)
- Add instructions for installing prebuilt images while using binaries [!5508](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5508)
- Add note regarding support for only Azure VMSS Uniform Orchestration mode [!5526](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5526)
- Remove "Autoscaler algorithm and parameters" from the GitLab Runner instance group autoscaler page [!5517](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5517)
- Add argo_translation.yml for continuous translation process [!5541](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5541)
- Clean up tables in runner docs [!5548](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5548)
- Make dependant Docker images optional for runner incept [!5538](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5538)

## v17.11.1 (2025-05-05)

### Bug fixes

- Update fleeting dependency [!5535](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5535)

## v17.11.0 (2025-04-14)

### New features

- Add ubuntu arm64 pwsh runner helper image [!5512](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5512)
- kubernetes/docker executor: add job timeout as annotations/labels [!5463](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5463) (Gordon Bleux @UiP9AV6Y)
- docs: add information about GRIT support and min_support to docs [!5460](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5460)
- GLR | winrm+https and Protocol Port [!5301](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5301) (Brayden White @brayden-lm)
- docs: add section to docs about who is using GRIT [!5462](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5462)
- Fix cache's Last-Modified header by ensuring it is set to UTC [!5249](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5249) (clyfish @clyfish)
- Specify which "user" shall run the job from the gitlab-ci.yaml for k8s executor [!5469](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5469)
- Allow overriding FILTER_FLAG in clear-docker-cache script [!5417](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5417)
- docs: Add documentation about Advanced Configuration for GRIT [!5500](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5500)
- Expose started_at and finished_at values in Usage Log job details [!5484](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5484)
- Ensure automatic `git gc` operations run in the foreground [!5458](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5458) (Ben Brown @benjamb)
- Enable FF_USE_NATIVE_STEPS by default [!5490](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5490)
- docs: add contributing section for GRIT docs [!5461](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5461)

### Bug fixes

- RmFilesRecursive should not attempt to delete directories [!5454](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5454)
- Sign Windows runner binary executables [!5466](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5466)
- Clean git config [!5438](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438)
- Add note mentioning PathTooLongException regression on Windows [!5485](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5485)
- Update docs re. ECS Fargate image override [!5476](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5476)
- Fix powershell stdin data race [!5507](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5507) (Guillaume Chauvel @guillaume.chauvel)
- Change directories and files permissions for bash shell when FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR is enabled [!5415](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5415)
- Fix usage log timestamp generation [!5453](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5453)
- Fix cache extractor redownloading up-to-date caches for Go Cloud URLs [!5394](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5394)
- Fix CI_JOB_TOKEN storage and removal of credentials [!5430](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5430) (Guillaume Chauvel @guillaume.chauvel)
- Authenticate runner requests with JOB-TOKEN instead of PRIVATE-TOKEN [!5470](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5470)

### Maintenance

- SNPowerShell is only for Windows, remove OS check [!5498](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5498) (Guillaume Chauvel @guillaume.chauvel)
- ServiceAccountName deprecation [!5501](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5501)
- Consistent CI yaml formatting [!5465](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5465)
- Mask sensitive config fields for debug logs [!5116](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5116) (ZhengYuan Loo @loozhengyuan)
- Make sure that inline config can't override the 'default: false' setting [!5436](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5436)
- chore: consolidate regexes into a single regex [!5390](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5390)
- remove outdated mentions [!5499](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5499)
- Update target milestone from 18.0 to 20.0 for runner registration token [!5487](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5487)
- docs: Clarify how autoscaler idle_time is calculated [!5474](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5474)
- Update GitLab Runner Ubuntu support matrix to pin to end of standard support [!5424](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5424)
- Update link to tech writing course in `gitlab-runner` repo [!5433](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5433)
- Fix broken test due to sort order [!5479](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5479)
- Run unit tests in the Kubernetes cluster [!5420](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5420)
- feat: describe how to use FARGATE_TASK_DEFINITION [!5439](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5439) (Jonathan @KJLJon)
- Remove v0.2 of SLSA as no longer supported [!5475](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5475)
- Backfill missing changelog entries for v17 releases [!5450](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5450)
- Make Alpine 3.19 the default base for helper images [!5435](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5435)
- Update linting tools in project [!5503](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5503)
- Docs: Update link to documentation labels in runner repo [!5472](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5472)
- Trigger downstream pipeline to test OS packages [!5416](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5416)
- docs: Add warning against sharing autoscaling resources [!5445](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5445)
- Add job to rebase branches on main [!5497](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5497)
- Eliminate dependencies needed in `yaml:lint` CI job [!5467](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5467)
- Update docker-machine version to v0.16.2-gitlab.34 [!5451](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5451)
- Deploy to Kubernetes cluster with KUBERNETES_DEPLOY_BRANCH condition [!5489](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5489)
- Update step-runner module to v0.8.0 [!5488](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5488)
- Migrate to mockery's packages configuration [!5480](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5480)
- Add Support Warning to Fargate custom tutorial [!4911](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4911)
- Bump base images for CVE [!5483](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5483)
- Docs: Hugo migration - Updating gitlab-runner doc links [!5448](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5448)
- Separate GitLab Runner autoscaler content [!5468](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5468)
- Add a prerequisite to Windows Runner documentation [!5473](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5473)
- Update example [!5509](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5509)
- Add "file name too long" troubleshooting error due to job token breaking change [!5496](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5496)
- Update docker machine version [!5482](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5482)
- Revert "Don't make this pipeline depend on the downstream pipeline" [!5449](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5449)
- Remove mention of GitLab 18.0 removal [!5437](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5437)
- Simplify git credential get [!5447](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5447) (Guillaume Chauvel @guillaume.chauvel)
- Back up unsigned binaries [!5478](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5478)
- Improve concurrent-related messages and docs [!5143](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5143)
- Add permissions docs for Operator containers [!5444](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5444)
- Fix k8s integration tests resource groups [!5502](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5502)
- GitLab Runner instance group autoscaler doc improvements [!5492](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5492)
- Add note to not install runner in AMI and standardize capitalization, note no multi-zone instance group support [!5495](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5495)
- Fix incept tests [!5434](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5434)
- Add how to exclude image [!5335](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5335)
- Handle vulnerabilty against CVE-2025-30204 [!5481](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5481)
- Fix package tests pipeline trigger [!5452](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5452)

## v17.10.1 (2025-03-26)

### Bug fixes

- RmFilesRecursive should not attempt to delete directories [!5454](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5454)
- Fix usage log timestamp generation [!5453](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5453)

## v17.10.0 (2025-03-19)

### New features

- Add support for fleeting heartbeats/connectivity check before instance acquisition [!5340](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5340)
- Add GPUs support for services [!5380](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5380) (Sigurd Spieckermann @sisp)
- Add add-mask functionality to proxy-exec [!5401](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5401)
- [docker] Expand variables in volume destinations [!5396](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5396)
- Update runner process wrapper [!5349](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5349)
- Add devices support on services [!5343](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5343) (Sigurd Spieckermann @sisp)
- Add proxy shell execution [!5361](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5361)

### Security fixes

- Merge branch 'sh-cache-upload-env-file' into 'main' [!5408](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5408)

### Bug fixes

- Allow OS overwrite via ShellScriptInfo [!5384](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5384)
- Downgrade prebuilt runner helper images to Alpine 3.19 [!5426](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5426)
- Fix HTTP retries not working properly [!5409](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5409)
- Make submodule `--remote` more resilient [!5389](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5389)
- Fix runner_wrapper gRPC API client [!5400](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5400)
- Fix inconsistent arguments when creating a service in tests [!5355](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5355) (Sigurd Spieckermann @sisp)
- Exclute helpers/runner_wrapper/api/v* tags from version evaluation [!5427](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5427)
- Clean git config [!5442](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5442)
- Support non-ASCII characters in gzip artifact headers [!5186](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5186)
- Only add step-runner volume mount when native steps is enabled [!5398](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5398)
- Fix json schema validation warnings [!5374](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5374)
- Detect bucket location when not provided [!5381](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5381)
- Clean git config [!5438](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438)
- Fix table rendering [!5393](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5393)

### Maintenance

- Update vale rules for runner docs [!5388](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5388)
- Fix Vale issues in Runner docs: Part 17 [!5405](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5405)
- Avoid using deprecated class for review apps [!5382](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5382)
- Fix Vale issues in Runner docs: Part 21 [!5419](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5419)
- Update the example versions [!5413](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5413)
- Merge 17.9.1 CHANGELOG into main [!5410](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5410)
- Fix CVE-2024-45338 by updating golang.org/x/net [!5404](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5404)
- Fix autoscaler policy table format [!5387](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5387)
- Use pipeline helper-binary for custom, instance and ssh integration tests [!5386](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5386)
- Update docker-machine version in docs [!5366](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5366)
- Update route map for runner review apps [!5365](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5365)
- Update docs content to use Hugo shortcodes [!5362](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5362)
- Update zstandard version to 1.5.7.20250308 [!5411](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5411)
- Fix CVE-2025-27144 by upgrading github.com/go-jose/go-jose/v3 [!5403](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5403)
- Use correct values for log_format [!5376](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5376)
- Upgrade Ubuntu image to 24.04 [!5428](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5428)
- Bump runner base images version to 0.0.10 [!5423](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5423)
- Only use docs-gitlab-com project for review apps [!5364](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5364)
- Add new supported runner package distros [!5425](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5425)
- Bump base image version to 0.0.9 [!5407](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5407)
- Bump Go to version 1.23.6 [!5326](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5326)
- Fix Vale issues in Runner docs: Part 14 [!5383](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5383)
- Fix rules for trigger deploy kube job [!5369](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5369)
- Fix small typo [!5422](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5422)
- Make UBI image wait for downstream pipeline success [!5360](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5360)
- Add clarification on the support policy for the docker machine executor to autoscale.md [!5359](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5359)
- Use TW Team Docker image for site build test [!5391](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5391)
- Consistently use local helper image in CI [!5371](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5371)
- Add clarification on the support policy for the docker machine executor to dockermachine.md [!5358](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5358)
- Update feature flag docs template for Hugo site launch [!5258](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5258)
- Fix Vale issues in Runner docs: Part 20 [!5418](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5418)
- Fix Vale issues in Runner docs: Part 19 [!5412](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5412)
- Fix Vale issues in Runner docs: Part 18 [!5406](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5406)
- Added executor supported OS and selection criteria - part 1 [!5345](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5345)
- Remove duplicate hugo code to fix broken master [!5368](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5368)
- Add comment regarding scale in protection for an AWS auto scaling group [!5348](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5348)
- Update links to docs from runner docs [!5363](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5363)
- Update links for jobs and tags [!5375](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5375)
- Update documentation to point to Rake task to deduplicate tags [!5356](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5356)
- Pin zstandard version and specify checksum [!5395](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5395)
- Move trigger deploy to kubernetes to a deploy stage [!5372](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5372)
- Fixed Vale issues [!5378](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5378)


## v17.9.2 (2025-03-20)

### Bug fixes

- [17.9] Downgrade prebuilt runner helper images to Alpine 3.18 [!5431](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5431)
- Clean git config [!5441](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5441)
- Clean git config [!5438](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438)
- [17.9] Fix HTTP retries not working properly [!5432](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5432)

## v17.9.1 (2025-03-07)

### Security fixes

- Merge branch 'sh-cache-upload-env-file' into 'main' [!5408](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5408)

## v17.9.0 (2025-02-20)

### New features

- Add support for fleeting heartbeats/connectivity check before instance acquisition [!5340](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5340)
- Remove lock files left over in .git/refs [!5260](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5260) (Ben Brown @benjamb)
- Autogenerate documentation for supported linux distros/versions [!5276](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5276)
- use '-f' to allow for race condition (issue #38447) [!5324](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5324) (Christian Moore @moorehfl)
- Allow custom naming of service container for the k8s executor [!4469](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4469)
- Mask by default all known token prefixes [!4853](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4853)
- Introduce new custom executor build exit code [!5028](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5028) (Paul Bryant @paulbry)
- Add GRIT documentation [!5263](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5263)
- Expand default labels on build pods [!5212](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5212) (Zalan Meggyesi @zmeggyesi)
- Add finished job usage data logging [!5202](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5202)
- Add gitlab_runner_job_prepare_stage_duration_seconds histogram [!5334](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5334)
- Inject the step-runner binary into the build container [docker executor] [!5322](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5322)
- Run rpm_verify_fips against FIPS images [!5317](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5317)
- Support ImageLoad for prebuilt images [!5187](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5187)
- Update step-runner docker executor integration docs [!5347](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5347)
- Add labeling to Usage Logger [!5283](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5283)

### Security fixes

- Bump base images version to 0.0.6 [!5346](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5346)

### Bug fixes

- Upgrade RUNNER_IMAGES_VERSION to v0.0.4 [!5305](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5305)
- Fix Role ARN support with S3 Express buckets [!5291](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5291)
- Fix Windows image gitlab-runner-helper path [!5302](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5302)
- Image pusher fixes [!5294](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5294)
- Fix step-runner inject container run [!5354](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5354)
- Improve job final update mechanism [!5275](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5275)
- Revert "Merge branch 'sh-fix-role-arn-s3-express' into 'main'" [!5308](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5308)
- Deflake pod watcher tests [!5310](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5310)
- Fix runner image missing tag [!5289](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5289)
- Do not create containers with duplicate env vars [!5325](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5325)
- Upgrade RUNNER_IMAGES_VERSION to v0.0.3 [!5300](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5300)
- Fix race in pod watcher test [!5296](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5296)
- Fix runner release bugs [!5286](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5286)
- Document how to configure S3 Express buckets [!5321](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5321)
- Make custom_build_dir-enabled optional [!5333](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5333)
- Push the helper image packages to S3 [!5288](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5288)
- Create copy of aliased helper images, not symlinks [!5287](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5287)
- Disable interactive git credentials [!5080](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5080)
- Add clear-docker-cache script to runner image [!5357](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5357)
- Gracefully handle missing informer permissions [!5290](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5290)
- Catch external pod disruptions / terminations [!5068](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5068)
- Fix a Vault kv_ v2 error [!5341](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5341)
- Document apt limitation and required workaround [!5319](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5319)
- CI: add release on riscv64 [!5131](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5131) (Meng Zhuo @mengzhuo1203)
- Fix missing default alpine images [!5318](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5318)

### Maintenance

- Add clarification on the support policy for the docker machine executor to dockermachine.md [!5358](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5358)
- Update docs content to use Hugo shortcodes [!5362](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5362)
- Update self-managed naming in all Runner docs [!5309](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5309)
- Run ubi images with BUILD_COMMIT_SHA and PARENT_PIPELINE_ID [!5244](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5244)
- Fix formatting and add link to GRIT docs [!5273](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5273)
- Replace deprecated field name with the new name [!5298](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5298)
- Bump base image version [!5282](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5282)
- Docs: Fix broken external links in runner docs [!5344](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5344)
- Deploy each commit from main to kubernetes cluster [!5314](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5314)
- Fix flaky logrotate write test [!5292](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5292)
- Update step-runner library version to 0.3.0 [!5272](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5272)
- Make sure deploy to kubernets works only on main [!5352](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5352)
- Add global operator config options docs [!5351](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5351)
- Update offering badges to standard name [!5303](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5303)
- Update feature flag docs template for Hugo site launch [!5258](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5258)
- Docs update - Update Architecture naming for GRIT [!5274](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5274)
- Properly handle shortening for tokens with prefix glcbt- [!5270](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5270)
- Document userns_mode by providing links to Docker docs [!5194](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5194)
- Document select executors information as an unordered list [!5268](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5268)
- Update links to docs from runner docs [!5363](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5363)
- Docs: Render RPM distro table correctly [!5338](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5338)
- Fix helper-bin-host target [!5252](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5252)
- Reduce busy work in main job loop [!5350](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5350)
- Add riscv64 binary download links [!5304](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5304) (Meng Zhuo @mengzhuo1203)
- Remove hosted runner section from under Administer [!5299](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5299)
- Update docker-machine version [!5339](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5339)
- More debug logging for artifact uploads & troubleshoot docs [!5285](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5285)
- Update taskscaler to get ConnectInfo fix for state storage instances [!5281](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5281) (Matthias Baur @m.baur)
- Use embedded VCS information rather than add manually [!5330](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5330)
- Add clarification on the support policy for the docker machine executor to autoscale.md [!5359](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5359)
- Fix windows image zstd compressing [!5323](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5323)
- Clean up unused GetUploadEnv() in cache code [!5265](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5265)
- Document proxy and self-signed certificate error [!5280](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5280)
- Add service_account parameter in [runners.kubernetes] section [!5297](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5297)
- Docs: add the mount_propagation parameter to the k8s executors documentation [!5353](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5353) (Georgi N. Georgiev @ggeorgiev_gitlab)
- Roll docs linting tooling forward [!5284](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5284)
- Rename index and move titles to frontmatter [!5327](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5327)
- Direct-use of the `rpm`  command adversely impacts the `yum`/`dnf`  database... [!5311](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5311) (Thomas H Jones II @ferricoxide)
- Disable Windows Defender properly [!5279](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5279)
- Add support for building docker images for local dev [!5271](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5271) (Anthony Juckel @ajuckel)
- Add a CI job to test the docs website build [!5306](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5306)
- Add a template for kubernetes feature toggle [!5315](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5315)
- Remove obsolete note regarding Alpine DNS issues [!5320](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5320) (Craig Andrews @candrews)


## v17.8.4 (2025-03-20)

### Security fixes

- Use a dotenv file to store cache environment variables [!5414](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5414)

### Bug fixes

- Clean git config [!5440](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5440)
- Clean git config [!5438](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438)

## v17.8.3 (2025-01-23)

### Bug fixes

- Upgrade RUNNER_IMAGES_VERSION to v0.0.4 [!5305](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5305)
- Fix Role ARN support with S3 Express buckets [!5291](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5291)

### Maintenance

- Run ubi images with BUILD_COMMIT_SHA and PARENT_PIPELINE_ID [!5244](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5244)

## v17.8.2 (2025-01-22)

### Bug fixes

- Upgrade RUNNER_IMAGES_VERSION to v0.0.4 [!5305](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5305)
- Fix Role ARN support with S3 Express buckets [!5291](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5291)
- Fix Windows image gitlab-runner-helper path [!5302](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5302)
- Upgrade RUNNER_IMAGES_VERSION to v0.0.3 [!5300](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5300)
- Image pusher fixes [!5294](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5294)

### Maintenance

- Run ubi images with BUILD_COMMIT_SHA and PARENT_PIPELINE_ID [!5244](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5244)

## v17.8.1 (2025-01-17)

### Bug fixes

- Fix runner release bugs [!5286](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5286)
- Fix Windows image gitlab-runner-helper path [!5302](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5302)
- Image pusher fixes [!5294](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5294)
- Push the helper image packages to S3 [!5288](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5288)
- Upgrade RUNNER_IMAGES_VERSION to v0.0.3 [!5300](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5300)
- Fix runner image missing tag [!5289](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5289)
- Create copy of aliased helper images, not symlinks [!5287](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5287)
- Upgrade RUNNER_IMAGES_VERSION to v0.0.4 [!5305](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5305)
- Fix Role ARN support with S3 Express buckets [!5291](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5291)

### Maintenance

- Bump base image version [!5282](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5282)
- Run ubi images with BUILD_COMMIT_SHA and PARENT_PIPELINE_ID [!5244](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5244)

## v17.8.0 (2025-01-13)

### New features

- Add mount propagation mode for hostpath mounts on kubernetes [!5157](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5157) (Brinn Joyce @brinn.joyce)
- Add RoleARN to handle both upload and download S3 transfers [!5246](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5246)
- Expand variables for the docker platform with unit tests [!5146](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5146) (John Sallay @jasallay)
- Document RoleARN configuration parameter [!5264](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5264)
- Add support for Windows 24H2 [!5170](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5170) (Martin Blecker @AdrianDeWinter)

### Bug fixes

- Fix docker network config for Windows [!5182](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5182)
- Limit UploadARN session duration to 1 hour [!5230](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5230)
- Incompatible pull policies should not be a retryable error [!5256](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5256)
- Fix issue #29381: Missing labels from Docker config when starting service containers [!4913](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4913) (Andrew Rifken @arifken)
- Fix runner deb package upgrade [!5251](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5251)

### Maintenance

- Fix incorrect references to packagecloud.io [!5242](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5242)
- Fixed Vale issues in Runner docs: Part 9 [!5239](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5239)
- Upgrade Akeyless SDK to v4 [!5234](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5234) (Amir Maor @amir.m2)
- Update documentation for manual installation of the new packages [!5247](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5247) (Victor De Jong @victordejong)
- Note Reuse previous clone if it exists support for k8s [!5248](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5248) (Ben Bodenmiller @bbodenmiller)
- Add note about being unable to change some settings via config template due to known issue [!5240](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5240)
- A couple of minor tweaks to the gitlab-runner-helper-images package [!5262](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5262)
- Correct spelling in comment [!5181](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5181) (MarlinMr @MarlinMr)
- Clarify docker container support policy [!5232](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5232)
- Fixed Vale issues in the Configure GitLab Runner on OpenShift doc [!5208](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5208)
- Remove misleading information about the initiation of the session server [!5238](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5238) (Nicolas @nicoklaus)
- Use runner base images [!5148](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5148)
- Update 17-7 changelogs [!5259](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5259)
- Let Docker site redirect to latest version [!5222](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5222)
- Improve documentation for Azure workload identities [!5221](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5221)
- Change anyuid service account to gitlab-runner-app-sa [!5237](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5237)
- Put `RPM` in backticks [!5255](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5255)
- Fix a 404 error in the Runner repo [!5254](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5254)
- Clarify ServiceAccount of the runner manager [!5250](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5250)
- Add Troubleshooting for docker autoscaler executor [!5220](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5220)
- Runner cache s3 table cleanup [!5267](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5267)
- Fix fork pipelines by ensuring windows tag refs exist [!5241](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5241)
- Update steps version to 0.2.0 [!5219](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5219)
- Update step-runner library version to 0.3.0 [!5272](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5272)
- Update golang.org/x/crypto to v0.31.0 [!5253](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5253)

## v17.7.1 (2025-01-17)

### Bug fixes

- Fix runner deb package upgrade [!5251](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5251)

### Maintenance

- Update step-runner library version to 0.3.0 [!5272](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5272)
- A couple of minor tweaks to the gitlab-runner-helper-images package [!5262](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5262)

## v17.7.0 (2024-12-19)

### New features

- Move exported  helper images into separete  package [!5190](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5190)

### Bug fixes

- Update gitlab.com/gitlab-org/fleeting/fleeting version and other deps [!5207](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5207)
- Fix flaky step-integration test [!5199](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5199)
- [k8s] Do not wait poll timeout when container has terminated [!5112](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5112)
- Fix docker network config for Windows [!5182](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5182)
- Use GoCloud URLs for Azure downloads [!5188](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5188)
- Merge Outstanding Security MRs [!5171](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5171)

### Maintenance

- Add 'Example' column to Docker runner advanced configuration docs [!5177](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5177)
- Bump UBI base images to the newest 9.5.x versions [!5185](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5185)
- Revert "Merge branch 'avonbertoldi/git-lfs-is-bad' into 'main'" [!5169](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5169)
- Make build environment cache friendly [!5179](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5179)
- Doc/runner spelling exceptions [!5162](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5162)
- Moved some contextual info to runner registration section [!5178](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5178)
- Note in logs when runner manager is being unregistered [!5166](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5166)
- Custom.md: Fix typo [!5163](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5163) (Per Lundberg @perlun)
- Fix podman troubleshooting doc [!5211](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5211) (Thorsten Banhart @banhartt)
- Bump Go compiler version to 1.23.2 [!5153](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5153)
- Skip homedir fix test on Windows to not block the release [!5164](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5164)
- Update supported runner OS versions [!5217](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5217)
- Add docker connection error to troubleshooting [!5165](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5165)
- Skip testKubernetesBuildCancelFeatureFlag as it's flaky [!5228](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5228)
- Update file configuring_runner_operator.md [!5198](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5198)
- Add Windows Server 2025 to the Windows version support policy [!5183](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5183)
- Actually update step-runner version to 0.2.0 [!5227](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5227)
- Migrate Kubernetes integration tests to use the Runner Kubernetes Cluster [!5175](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5175)
- Remove redundant prepare runner-incept variables job [!5197](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5197)
- Fix apt package install version string and change to a newer version in the docs [!5180](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5180) (Thorsten Banhart @banhartt)
- Kubernetes API reference analyzer based on types instead of field names [!5158](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5158)
- Update steps version to 0.2.0 [!5219](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5219)
- Fix `limitations` in `gitlab-runner` repo [!5201](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5201)
- Fix Vale issues in Kubernetes executor doc [!5196](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5196)
- Add feature flag issue templates [!5156](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5156)
- Fix TestStackDumping test freezing [!5210](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5210)
- Separate kubernetes integration tests resource groups [!5223](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5223)
- Make fuzz variable mask job required to pass [!5209](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5209)
- Remove semgrep-sast CI rules [!5184](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5184)
- Speed up windows test runs [!5174](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5174)
- Added information about Docker Autoscaler and Instance executors in the executor selection table [!5161](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5161)
- Bump docker-machine to v0.16.2-gitlab.30 [!5218](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5218)
- Fix fuzz variable mask test [!5135](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5135)
- Remove the term worker from the Plan and operate a fleet of instance or group runners doc [!5189](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5189)
- Remove links to interactive web terminals [!5176](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5176)
- Make homedir easier to test [!5168](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5168)
- Add node tolerations to kubernetes integration tests [!5229](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5229)
- Document how to use Azure workload identities for the cache [!5204](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5204)
- Simplify kubernetes integration test names [!5024](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5024)
- Clarify docker container support policy [!5232](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5232)
- Update alpine versions [!5214](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5214)
- Add ability to create review apps by using the GitLab Docs Hugo project [!5205](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5205)
- Update dependency danger-review to v2 [!5206](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5206)
- Fix Vale issues in Runner docs: Part 5 [!5191](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5191)
- Update experiment-beta page path [!5193](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5193)

## v17.6.1 (2024-12-19)

### Bug fixes

- Fix docker network config for Windows [!5182](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5182)

## v17.6.0 (2024-11-20)

### New features

- Invoke step-runner from $PATH instead of / [!5140](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5140)
- Native Step Runner Integration for Docker Executor [!5069](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5069)
- Really publish sles/opensuse runner packages [!5101](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5101)
- Terminate job and display error when services are oom killed on the kubernetes executor [!4915](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4915) (Zach Hammer @zhammer)
- Add taskscaler state storage options [!5061](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5061)
- Create a GitLab Runner process wrapper [!5083](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5083)
- Warn user if no fleeting plugins to install [!5115](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5115) (ZhengYuan Loo @loozhengyuan)

### Bug fixes

- [k8s] Terminate PowerShell Script children processes when cancelling the job through UI [!5081](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5081)
- Fix path-style requests with Upload ARN functionality [!5150](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5150)
- Remove trailing "/" from cache fallback keys [!5076](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5076)
- Omit canonical ports for S3 endpoints [!5139](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5139)
- Fix UploadRoleARN URL when other regions are used [!5113](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5113)
- Fix home directory detection [!5087](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5087)
- Upgrade github.com/mvdan/sh to v3.9.0 [!5085](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5085)
- Disable FF_GIT_URLS_WITHOUT_TOKENS by default [!5088](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5088)
- Fix S3 cache access for buckets located outside US [!5111](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5111)
- Fix auth issues with FF_GIT_URLS_WITHOUT_TOKENS: 1 [!5103](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5103)
- Wait for k8s pod to become attachable as part of poll period in exec mode [!5079](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5079)
- Default to us-east-1 region for AWS SDK v2 [!5093](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5093)
- Hide Pod wait to be attachable behind a FF [!5098](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5098)
- Fix fleeting plugin installation architecture selection [!5090](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5090)

### Maintenance

- Remove fault tolerance section [!5154](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5154)
- Update CONTRIBUTING.md and LICENSE [!5133](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5133)
- Fix Vale issues in Runner docs: Part 4 [!5160](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5160)
- Added docker autoscaler and instance executors info [!5128](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5128)
- Replace the term shared runner with instance runner in Runner docs [!5104](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5104)
- Add AWS hosted MacOS instance troubleshooting note [!5082](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5082)
- Upgrade github.com/BurntSushi/toml, dario.cat/mergo [!5086](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5086)
- Add comment to help future users [!5070](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5070)
- Documented Podman with Runner K8s executor [!5056](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5056)
- Update CHANGELOG for v17.5.3 [!5136](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5136)
- Fix mage k8s:generatePermissionsDocs intermittent test faliures [!5107](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5107)
- Update CI release task to upload with AWS CLI [!5106](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5106)
- Remove broken link [!5118](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5118) (Zubeen @syedzubeen)
- Set gitlab-advanced-sast job to run on code changes [!5097](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5097)
- Fix Vale issues in Runner docs: Part 1 [!5149](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5149)
- Make docker and helper image jobs optional [!5141](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5141)
- Install git-lfs in ubi image from upstream RPM repo [!5122](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5122)
- Follow up MR to add changes to MR 5120 [!5123](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5123)
- Restore 2nd method of restarting after config [!5077](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5077)
- Improve helpers/cli/FixHOME [!5089](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5089)
- Add integration tests to cover service container behaviour [!5144](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5144)
- Skip TestBuildContainerOOMKilled integration test [!5151](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5151)
- Update GitLab Runner CHANGELOG after v17.5.2, v17.4.2, v17.3.3, v17.2.3 patches [!5120](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5120)
- Stop testing UseWindowsLegacyProcessStrategy for KillerTest [!5102](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5102)
- Skip homedir fix test on Windows to not block the release [!5164](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5164)
- Update GitLab Runner CHANGELOG after v17.5.1 patch [!5121](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5121)
- Fix a technical error in the Podman doc [!5138](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5138)
- Remove github.com/tevino/abool in favor of atomic.Bool [!5072](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5072)
- Removed fault tolerance section [!5159](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5159)
- Doc/executor intro feedback [!5155](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5155)
- Note in logs when runner manager is being unregistered [!5166](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5166)
- Delete Runner topics marked for removal [!5124](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5124)
- Minor documentation corrections [!5110](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5110)
- Explain relationship between limit and burst in runner autoscaler configs [!5100](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5100)
- Transfer MR short commit SHA to Runner Incept [!5130](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5130)
- Update Configuration of the metrics HTTP server for runners [!5142](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5142)
- Replace config_exec_args with config_args [!5109](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5109) (Davide Benini @davidebenini)
- Add tests for service name empty [!5065](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5065)
- Fixed Vale issues Runner docs: Part 2 [!5152](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5152)

## v17.5.5 (2024-12-19)

### Bug fixes

- Fix docker network config for Windows [!5182](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5182)

## v17.5.4 (2024-11-19)

### Maintenance

- Fix mage k8s:generatePermissionsDocs intermittent test faliures [!5107](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5107)

## v17.5.3 (2024-10-31)

### Bug fixes

- Fix UploadRoleARN URL when other regions are used [!5113](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5113)
- Fix S3 cache access for buckets located outside US [!5111](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5111)

## v17.5.2 (2024-10-22)

### New features

- Publish SLES and openSUSE runner packages [!5101](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5101)

### Bug fixes

- Fix fleeting plugin installation architecture selection [!5090](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5090)
- Default to us-east-1 region for AWS SDK v2 [!5093](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5093)
- Hide Pod wait to be attachable behind a feature flag [!5098](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5098)

### Maintenance

- Stop testing `UseWindowsLegacyProcessStrategy` for `KillerTest` [!5102](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5102)

## v17.4.2 (2024-10-22)

### Bug fixes

- Remove trailing "/" from cache fallback keys [!5076](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5076)

## v17.3.3 (2024-10-22)

### Bug fixes

- Remove trailing "/" from cache fallback keys [!5076](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5076)

## v17.2.3 (2024-10-22)

### Bug fixes

- Remove trailing "/" from cache fallback keys [!5076](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5076)

## v17.5.1 (2024-10-18)

### Bug fixes

- Disable `FF_GIT_URLS_WITHOUT_TOKENS` by default [!5088](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5088)
- Fix home directory detection [!5087](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5087)

## v17.5.0 (2024-10-17)

### New features

- Document fault tolerance feature [!5058](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5058)
- Add namespace support for DOCKER_AUTH_CONFIG [!4727](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4727) (Tobias Rautenkranz @tobiasrautenkranz)
- Support AWS S3 multipart uploads via scoped temporary credentials [!5027](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5027)
- Limit token exposure [!5031](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5031)
- Add support for Azure Managed Identities in cache [!5007](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5007)
- Publish runner and helper packages for SLES and  OpenSUSE [!4993](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4993) (Aazam Thakur @Alcadeus0)

### Bug fixes

- Cancel stage script upon job cancellation in attach mode [!4813](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4813)
- Make invalid service image name a build error [!5063](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5063)
- Allow pull_policy to be unset when defining allowed_pull_policies [!4943](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4943) (Mike Mayo @magicmayo)
- Resolve "get "panic: EOF" when register runners run in a container" [!5012](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5012)
- Adjust autoscaler policy on config reloading [!5064](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5064)
- Require only build container to start in Kubernetes [!5039](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5039)
- Track kubernetes pull policies based off of the container name [!5036](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5036)
- Downgrade go-fips base image to ubi8 [!5040](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5040)
- Fix graceful termination of jobs on Windows [!4808](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4808) (Nils Gladitz @nilsgladitz)
- Switch to AWS SDK for S3 cache access [!4987](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4987)
- Remove quotes around IP address in ssh invocation in example [!4899](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4899) (Yassine Ibr @yassineibr1)
- Wait for k8s pod to become attachable as part of poll period [!3556](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3556) (Jimmy Berry @jimmy-outschool)
- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)
- Fix `pod_annotations_overwrite_allowed` parsing error [!5032](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5032)
- Fix bug in scripts/logs dir for k8s executor [!4893](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4893) (Thomas John Randowski @WojoInc)
- Address CVE-2024-41110/GHSA-v23v-6jw2-98fq by upgrading github.com/docker/docker and github.com/docker/cli [!4925](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4925)

### Maintenance

- Update CHANGELOG after patches release [!5073](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5073)
- Helm chart install page: start structural revisions [!5038](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5038)
- Refactor container entrypoint forwarder [!5018](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5018)
- docs: set admin access for rancher desktop [!5062](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5062)
- Update gocloud.dev for AWS client-side rate limiting fix [!5066](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5066)
- Use latest markdownlint-cli2 and linter configuration [!5055](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5055)
- Add use case to docs for system_id and reusing configurations [!5051](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5051)
- Bump docker-machine to v0.16.2-gitlab.29 [!5047](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5047)
- Docs: Link to Docker certificate docs [!5023](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5023)
- Update GitLab release version for DualStack and Accelerate config [!5042](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5042)
- Fix capitalization [!5015](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5015) (maximilian @maximiliankolb)
- Use Windows test code coverage reports [!5041](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5041)
- Update Docker image for docs review apps [!5020](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5020)
- Remove trailing whitespace from GitLab Runner docs [!5074](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5074)
- Updating intro sentence again [!5025](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5025)
- Bump golang to 1.22.7 [!5035](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5035)
- Fix community Merge Request pipeline parse errors [!4973](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4973) (Thorsten Banhart @banhartt)
- Docker install: start with line-level cleanups [!5033](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5033)
- Improve flaky waitForRunningContainer test [!5016](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5016)
- Enable timestamps for CI/CD jobs [!5048](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5048)
- Refactor the linux repository page to follow CTRT [!5019](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5019)
- [k8s] more explicit docs on OS, Arch, KernelVersion selection [!5009](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5009)
- Squelch jsonschema warning about DualStack config [!5022](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5022)
- Clean up stray whitespace in gitlab-runner project [!5052](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5052)
- Update interactive runner registration documentation [!5008](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5008)
- Break apart Helm chart optional config into new page [!5054](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5054)
- [docs] Fix concurrent_id being used when it is concurrent_project_id [!5026](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5026) (Nils @NilsIRL)
- K8s install page: move troubleshooting info, tackle 2 subheads [!5034](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5034)
- Add deprecation note to the Docker Machine autoscale configuration docs page [!5060](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5060)
- Implement distroless UBI pattern [!4971](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4971)
- Add deprecation note to the Docker Machine executor docs page [!5059](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5059)
- Docker install page: clean up installation steps [!5037](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5037)
- Fix code coverage visualization not working in merge requests [!5029](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5029)
- Skip TestDockerCommand_MacAddressConfig on Windows OS [!4999](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4999)
- Resolve "Proxy configuration docs missing NO_PROXY instructions" [!5017](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5017)
- Remove license scanning template [!4735](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4735)
- Add markdownlint-cli2 as asdf dependency [!5053](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5053)
- Remove note that the Azure Fleeting plug-in is in beta from docs [!5046](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5046)
- Fix example trust relationship in UploadRoleARN config [!5043](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5043)
- Address line-level findings in Kubernetes install page [!5030](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5030)

## v17.4.1 (2024-10-10)

### Bug fixes

- Require only build container to start in Kubernetes [!5039](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5039)
- Downgrade go-fips base image to ubi8 [!5040](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5040)

## v17.3.2 (2024-10-10)

### Bug fixes

- Downgrade go-fips base image to ubi8 [!5040](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5040)
- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)

## v17.2.2 (2024-10-10)

### Bug fixes

- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)

## v17.1.2 (2024-10-10)

### Bug fixes

- Downgrade go-fips base image to ubi8 [!5040](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5040)
- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)

## v17.0.3 (2024-10-10)

### Bug fixes

- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)

## v16.11.4 (2024-10-10)

### Bug fixes

- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)

## v16.10.1 (2024-10-10)

### Maintenance

- Remove license scanning template [!4735](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4735)

## v17.4.0 (2024-09-19)

### New features

- Forward entrypoint logs [!4883](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4883)
- Akeyless support [!4975](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4975)

### Bug fixes

- Custom executor script shouldn't end with '.' when no file extension [!4898](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4898)
- Fix Docker+machine download URL [!5014](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5014)
- Resolve "`gitlab-runner start` throws "FATAL: Failed to start GitLab-runner: exit status 134" when started prior to being logged in" [!4995](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4995)
- Fix segfault in unregisterRunner when called with no arguments [!4932](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4932)
- Prevent script dump on job cancellation through UI [!4980](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4980)
- Make image generation fail upon failure to download dumb-init ubi-fips [!4955](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4955)

### Maintenance

- Remove the GitLab Hosted Runners as an example of security risks with DIND [!5011](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5011)
- CTRT: Refactor the intro for Install GitLab Runner [!4974](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4974)
- Add link to debug trace docs page in the Runner Advanced Configuration doc [!4938](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4938)
- Add reference to troubleshooting to install step 3c of the Install GitLab Runner on macOS doc [!4991](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4991)
- Copy edit GitLab Runner system services doc [!4981](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4981)
- Update the version of Ruby referenced in the Setup macOS runners docs. [!4977](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4977)
- Add links to new autoscaling executors to the [[runners]] section in the Runner Advanced Configuration doc [!4930](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4930)
- Update the default container registry of the helper images [!4935](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4935)
- Fix fleeting plugin version constraint format documentation [!4985](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4985) (joola @jooola)
- Add GitLab Advanced SAST to CI/CD config [!4965](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4965)
- Change `Docker` to container in Kubernetes section of the Runner Advanced Configuration doc [!4957](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4957)
- Update tooling for local development, fix FIPS requirements [!4937](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4937)
- Add `Instance` and `Docker Autoscaler` executors to the default build dir section in the Runner Advanced Configuration doc [!4964](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4964)
- Change `docker` to `container` in the image_pull_secrets parameter in the Runner Advanced Configuration doc [!4959](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4959)
- Docker integration test for MacAddress setting [!4967](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4967)
- Add `docker autoscaler` and `instance` executors to the runners custom build section in the runner Advanced Configuration doc [!4963](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4963)
- Change `executor` description in the [[runners]] section in the Runner Advanced Configuration doc [!4931](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4931)
- Runner instance generally available [!4998](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4998)
- Update install GitLab Runner documentation for Amazon Linux [!4934](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4934)
- Remove `Beta` from the [[runners.autoscaler]] section in the Runner Advanced Configuration doc [!4952](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4952)
- Update the intro to the Shells table in Runner Advanced Configuration [!4941](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4941)
- Add link to the `services` docs page in the Runner Advanced Configuration doc [!4948](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4948)
- Change URL for nesting to docs page entry in the Runner Advanced Config doc [!4953](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4953)
- Update 3.18 and 3.19 alpine info [!4944](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4944)
- Fix broken links [!4936](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4936)
- Use latest docs Docker image and linting configuration docs [!5001](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5001)
- Revise install step 2 in the Install GitLab Runner on macOS doc [!4989](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4989)
- Change the intro section in the Install GitLab Runner on macOS doc [!4988](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4988)
- Don't depend on k8s.io/Kubernetes [!4984](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4984)
- Remove `Alpine 3.16` from the runner images section in the Advanced Configuration doc [!4960](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4960)
- Skip TestDockerCommand_MacAddressConfig on Windows OS [!4999](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4999)
- CTRT: Refactor install GitLab Runner [!4983](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4983)
- Add token newline troubleshooting item [!4966](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4966)
- Add an example config for check interval [!4928](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4928)
- Delete `experiment` label from `idleScaleFactor` [!4950](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4950)
- Change designation of Fleeting plugin for Azure from BETA to generally available [!5013](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5013)
- Updated Ruby version from 3.3 to 3.3.x [!4979](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4979)
- Update the config.TOML example in Runner Advanced Config [!4927](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4927)
- Add intro to the [[runners.nachine.autoscaling]] section [!4951](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4951)
- Add link to the Git LFS page in the Runner Advanced Configuration doc [!4939](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4939)
- Fleeting.md: fix bin path [!4914](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4914) (Naftoli Gugenheim @nafg)
- Bump UBI base image from `9.4-1194` to `9.4-1227` [!4997](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4997)
- Add rules to semgrep-sast job [!4923](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4923)
- Update the Global Section in the Runner Advanced Config doc [!4926](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4926)
- Docs(Kubernetes): mention AWS ASG Zone rebalancing [!5002](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5002) (Ummet Civi @ummetcivi)
- Clarify allowed_pull_policies default [!4969](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4969)
- Re-order sentences in the Helper image registry section of the Runner Advanced Config doc [!4961](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4961)
- Change `images` to `container images` in the Kubernetes section of the Runner Advanced Configuration doc [!4958](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4958)
- Add reference to Docker executor in the [runners.Docker] section [!4942](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4942)
- Change to terminate process in the runner Advanced Configuration dov [!4947](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4947)
- Revise install Step 3a in the Install GitLab Runner on macOS doc [!4990](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4990)
- Backfill test for waitForRunningContainer [!4996](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4996)
- Address GHSA-xr7q-jx4m-x55m by updating Google.golang.org/grpc to 1.64.1 [!4946](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4946)
- Change text on the use of runner in offline environments in Runner Advanced Configuration doc [!4962](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4962)
- Add `instance` and `docker-autoscaler` executors to the executors table [!4940](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4940)
- Improve flaky waitForRunningContainer test [!5016](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5016)
- Update `dumb-init` version on GitLab Runner images [!4956](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4956)
- Bump Docker-machine to 0.16.2-GitLab.28 [!4924](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4924)
- Change intro in the [runner.Kubernetes] section in the Runner Advanced Config doc [!4954](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4954)
- Add troubleshooting step to resolve install error on Apple M1 to the Install GitLab Runner on macOS [!4992](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4992)
- Update from EOL Ruby 2.7 to Ruby 3.3 in examples used in the Runner Advanced Configuration Doc [!4978](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4978)
- Expand session server configuration example [!4929](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4929)

### Documentation changes

- Add planning issue template [!4986](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4986)

## v17.3.1 (2024-08-21)

### Bug fixes

- Make image generation fail upon failure to download dumb-init ubi-fips [!4955](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4955)

## v17.3.0 (2024-08-09)

### New features

- Add debug log message for resolving Docker credentials [!4902](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4902)
- Add Git_STRATEGY of "empty" to support clean build directories [!4889](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4889) (Nathan Cain @nathan.cain)

### Security fixes

- Update azidentity dependency [!4903](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4903)

### Bug fixes

- Gracefully stop long running processes when using the shell executor - Take 2 [!4896](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4896)
- Fix default log format to FormatRunner [!4910](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4910)
- Use pull policy configuration for services [!4854](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4854)
- Upgrade fleeting: fix tcpchan deadline for tunnelled connections [!4917](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4917)
- Cancel stage script upon job cancellation in attach mode [!4813](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4813)
- Reset log level and format to default values when not set [!4897](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4897)
- Prevent additional newline in job in attach mode [!4901](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4901)

### Maintenance

- Fix formatting in runner registration doc [!4921](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4921)
- Remove funlen [!4912](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4912)
- Add a Runner Docker image tag for the build's revision [!4862](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4862)
- Apply split Vale rules to project [!4918](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4918)
- Skip `TestRunCommand_configReloading` unit test on windows platform [!4916](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4916)
- Re-enable Windows Docker Git-lfs tests [!4900](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4900)
- Remove Git 1.8.3.1 test [!4856](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4856)
- Merge back 16.11, 17.0, 17.1 and 17.2 patches in main branch [!4905](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4905)
- Work around a syntax highlighting problem [!4920](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4920)
- Remove Beta plugin warning for AWS [!4919](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4919)
- Clarify where to install the fleeting plugin [!4894](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4894)
- Docs maintenance: Add internal pages to ignore list [!4895](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4895)
- [k8s] Fix `testKubernetesWithNonRootSecurityContext` integration tests [!4892](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4892)
- Remove unneeded notes in tabs [!4922](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4922)
- Bump UBI base image from `9.4-1134` to `9.4-1194` [!4909](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4909)
- Add runner path requirement [!4904](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4904)
- Update index.md [!4908](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4908)
- Add missing requirement for network_mtu [!4890](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4890)
- Add note about using Workload Identity Federation for GKE [!4884](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4884) (Nabil ZOUABI @nabil_zouabi)
- [k8s] Fix `CaptureServiceLogs` integration tests [!4891](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4891)

## v17.2.1 (2024-07-25)

### Bug fixes

- Cancel stage script upon job cancellation in attach mode !4813

## v17.1.1 (2024-07-25)

### Bug fixes

- Cancel stage script upon job cancellation in attach mode !4813

### Maintenance

- Remove Git 1.8.3.1 test !4856

## v17.0.2 (2024-07-25)

### Bug fixes

- Cancel stage script upon job cancellation in attach mode !4813

## v16.11.3 (2024-07-25)

### Bug fixes

- Cancel stage script upon job cancellation in attach mode !4813

## v17.2.0 (2024-07-18)

### New features

- Add mount propagation volume option for Kubernetes executor !4784 (Subhashis Suara @subhashissuara)
- Always attempt to retrieve pod warning events !4852
- Ability to turn on/off running umask 0000 command for Kubernetes executor !4842
- Make directories for scripts and logs configurable for k8s Executor !4805 (Thomas John Randowski @WojoInc)

### Bug fixes

- Ensure that dialing fleeting instance can be canceled !4874
- Upgrade fleeting/taskscaler: fixes reservation/unavailability instance churn !4865
- Shorten network names created by FF_NETWORK_PER_BUILD !4830 (Ricardo Katz @rikatz)
- Fix Kubernetes executor runner API permission !4800 (EuVen @euven)
- Upgrade fleeting and taskscaler to fix instance churn/runaway !4843
- Fix default runner alpine flavour version !4850
- Upgrade fleeting and taskscaler to fix instance churn/runaway !4844
- Update clear-Docker-cache script to keep cache volumes !4847
- Set file permissions before creating runner rpm/deb packages !4835
- Fix parsing of --Kubernetes-host_aliases command-line flag !4837

### Maintenance

- Change product lifecycle status of the Fleeting Plugin for AWS from Beta to generally available !4887
- Manually update linting configuration and update Vale version !4881
- Update docs for modifying Git LFS endpoints !4886
- Add note about Runner install requirement !4885
- Update dependency danger-review to v1.4.1 !4860
- Docs maintenance: Fix 404 links !4882
- Fix periods example in [[runners.autoscaler.policy]] documentation. !4863 (Jan Jörke @janjoerke)
- Add note that AWS and GCP Fleeting plugins is beta !4879
- Bump golang to 1.22.5 !4878
- Fix more k8s integration test !4869
- Manually update Vale and markdownlint rules and fix docs !4873
- Add note to runner autoscaling for runner manager on fault tolerant design !4870
- Add note to runner autoscaling for runner manager on preemptive mode !4871
- Add note on the runner `tls_verify` setting to the docs !4872
- Add note to runner autoscaling on use of credentials file !4868
- Add note to runner autoscaling for runner manager on containers !4867
- Fix testKubernetesPodEvents integration test !4866
- Fix link that is broken when published !4864
- Bump Docker+machine version to v0.16.2-GitLab.27 !4859
- Use the danger-review component !4828
- Adress CVE-2024-6104 in GitHub.com/hashicorp/go-retryablehttp !4858
- Bump UBI bases image from `9.4-949` to `9.4-1134` !4857
- Remove Git 1.8.3.1 test !4856
- Add note about `FF_ENABLE_JOB_CLEANUP` feature flag !4849
- Suppress golangci-lint config deprecations and warnings !4845
- Fix some external links in the project !4851
- Document k8s executors for read-only root filesystem !4848
- Replace sysvinit-adjacent commands with systemd equivalents !4841
- Bump to Go 1.22.4 !4838
- Specify in which scenarios missing index has been seen !4839
- Create issue template for bumping golang !4840
- Use upstream spdystream again !4836

## v17.0.1 (2024-07-05)

### Bug fixes

- Upgrade fleeting and taskscaler to fix instance churn/runaway !4844

### Maintenance

- Remove Git 1.8.3.1 test !4856

## v16.11.2 (2024-07-05)

### Bug fixes

- Upgrade fleeting and taskscaler to fix instance churn/runaway !4843

### Maintenance

- Remove Git 1.8.3.1 test !4856

## v17.1.0 (2024-06-20)

### New features

- Add AFTER_SCRIPT_IGNORE_ERRORS variable allow not ignoring after_script errors !4758 (Tim @timmmm)
- Add Kubernetes configurable retry backoff ceiling !4790 (Nabil ZOUABI @nabil_zouabi)
- Remove Beta from runner autoscaler supported public cloud docs section !4823
- Update Fleeting docs to reflect GCP plugin transition to GA !4820
- Change status of Docker Autoscaler and Instance to GA !4821
- Log ETag of extracted cache archive if available !4769
- Allow reading run-single configuration from a config file !4789 (Tobias Ribizel @upsj)
- Add steps shim !4803

### Bug fixes

- Fix panic when err is nil on retry for k8s executor !4834
- Fix linter violation !4827
- Support handling Docker images with @digest !4557
- Fix Docker client intermittently failing to connect to unix socket !4801
- Override helper images entrypoint on Docker import !4793
- Fix jsonschema validation warning for Docker services_limit !4782 (Malte Morgenstern @malmor)
- Propagate exit codes through nested pwsh calls !4715 (Andy Durant @AJDurant)
- Fix jsonschema validation warning for Kubernetes retry_limits !4786 (Malte Morgenstern @malmor)

### Maintenance

- Add exact commands to fix signed by unknown authority !4833
- Document troubleshooting steps for 500 error creating tags !4831
- Remove BETA from the autoscaler docs !4832
- Upgrade GitHub.com/MinIO/MinIO-go to v7.0.70 !4819 (Mathieu Quesnel @xmath279)
- Remove GitHub.com/samber/lo dependency from main application !4811
- Update Docker Autoscaler executor docs !4822
- Enable tarzstd archive format for caches !4807
- Bump Docker+machine version to v0.16.2-GitLab.26 !4816
- Upgrade ubi fips base image from ubi8 to ubi9 !4814
- Check links in more files !4815
- Upgrade helper image Git-lfs to 3.5.1 !4812
- Update runner registration documentation !4809
- Update docs linting Docker images !4806
- Add note about KMS Alias syntax - Documentation !4792
- Fix external links in docs !4802
- Remove trailing whitespace !4799
- Bump to Go 1.22.3 !4795
- Move docs-related CI jobs to the same file !4787
- Docs: match example to text !4794 (Anton Dollmaier @a.dollmaier)
- Bump to Go 1.22.2 !4759
- Use lowercase for beta and experiment !4788
- Made beta and experiment lowercase !4785

## v17.0.0 (2024-05-16)

### New features

- Add fleeting docs describing new plugin installation method !4749
- Support Google Cloud secrets from projects other than the one containing the WIF pool !4718 (Rich Wareham @rjw57)
- Interpret failed pods as system failures rather than script failures for Kubernetes executor !4698 (Daniel Barnes @dbarnes3)
- Implement new GCS Cache adapter that uses Google Cloud SDKs auth discovery defaults !4706
- Add cpuset_mems option to Docker executor !4725 (Karthik Natarajan @karthiknatarajan)
- Add docs for Runner Operator in disconnected network environments !4716
- Add support for taskscaler scale throttle !4722
- Add the ability to disable the automatic token rotation !4721

### Security fixes

- Stop installing tar in ubi fips base image !4703

### Bug fixes

- Revert "Merge remote-tracking branch 'origin/16-11-stable'" !4761
- Upgrade fleeting and taskscaler for fixes !4745
- Upgrade fleeting and taskscaler for fixes !4745
- Remove runner from config when unregister with token !4750 (Karthik Natarajan @karthiknatarajan)
- Correctly set CI_JOB_STATUS to timedout/canceled when appropriate !4717
- Fix fleeting install subcommand for Windows !4753
- Fix fleeting install subcommand for Windows !4753
- Correctly set CI_JOB_STATUS to timedout/canceled when appropriate !4717
- Upgrade fleeting-artifact to fix plugin installation !4748
- Fix buildlogger write race !4734
- Upgrade fleeting-artifact to fix plugin installation !4748
- Fix buildlogger write race !4734

### Maintenance

- Require ci prep before incept !4762
- Upgrade runner-linters image and golangci-lint for Go 1.22 !4776
- Improve upgrade docs !4780
- Clean up docs redirects, runner - 2024-05-02 !4756
- Added missing apostrophe !4781
- State clearly one job = one VM = one container !4774
- Add information regarding Beta feature !4757
- Updating docs tier badge rules !4779
- Fix broken link and typo !4775
- Add badge info to autoscaler page !4772
- Provide examples and clarify how MachineOptions work for the MachineDriver !4768
- Update GitLab Runner Version !4773
- Fix stuck Windows 1809 jobs !4771
- Remove unsupported GitLab versions from Verify Runner group docs !4764
- Remove support for old pre_clone_script and post_clone_script configuration settings !4767
- Clarify "circular" docs links !4738
- Remove slsa_v02 from artifact_metadata !4760
- Remove cmd shell !4754
- Remove shared runner naming from GitLab Runner docs !4744
- Switch to Lychee for link checking !4737
- Convert custom Kubernetes error to retryError !4662 (Nabil ZOUABI @nabil_zouabi)
- Remove license scanning template !4735
- Fix jsonschema validation warning for monitoring struct !4724 (Malte Morgenstern @malmor)
- Updated examples to avoid Helm error !4752
- April: fixes trailing whitespace in GitLab Runner project !4751
- Remove legacy shell quoting and FF_USE_NEW_SHELL_ESCAPE feature flag !4742
- Remove `gitlab-runner exec` command !4740
- Add SSH: handshake failed to troubleshooting !4743
- Update autoscaler config option documentation !4730
- Fix dead documentation anchor links in README !4733 (Markus Heidelberg @marcows)
- Remove terminationGracePeriodSeconds !4739
- Remove license scanning template !4735
- Add prerequisite info to runner registration token section !4714
- Document the Beta status of the Google Cloud plugin for fleeting !4726
- Add security risk on runner debug for Shell executors !4586

## v16.11.1 (2024-05-03)

### Bug fixes

- Upgrade fleeting and taskscaler for fixes !4745
- Correctly set CI_JOB_STATUS to timedout/canceled when appropriate !4717
- Fix fleeting install subcommand for Windows !4753
- Upgrade fleeting-artifact to fix plugin installation !4748
- Fix buildlogger write race !4734

### Maintenance

- Remove license scanning template !4735

## v16.11.0 (2024-04-18)

### New features

- Taskscaler and fleeting now use logrus logger !4719
- Add autoscaler instance_ready_command option !4709
- Implement timestamped logs !4591
- Add Jfrog Artifactory Secretes engine as External Secrets option in Hashicorp Vault !4486 (Ivo Ivanov @urbanwax)
- Add fleeting subcommand to manage fleeting plugins !4690
- Extend GitInfo with RepoObjectFormat to store object format !4645

### Bug fixes

- Fix log timestamps fixed format !4712
- Avoid errors when creating files in pwsh 2.0 !4661 (Robin Lambertz @roblabla)
- Give up on the trace finalUpdate if it keeps on failing !4692
- Fix test coverage report job !4701
- Fix Makefile.build.mk for armv7l (#36976) !4682 (Alexander Puck Neuwirth @APN-Pucky)
- Rename SSH.Config to common.SshConfig to avoid misleading config.TOML validation error !4694
- Update GO_FIPS_VERSION to match GO_VERSION !4687
- Revert "Merge branch 'avonbertoldi/27443/graceful-shutdown-shell' into 'main'" !4686

### Maintenance

- Bump Go version to 1.21.9 !4711
- Re-enable SAST scanning !4683
- Update a few dependencies !4700
- docs: Remove period in the middle of a sentence !4708 (Markus Heidelberg @marcows)
- Runner: Updates docs-lint image for new SVG use !4697
- Remove extra square brackets in podman section !4705
- Check Docker version before using deprecated arg !4699 (Anthony Juckel @ajuckel)
- Change beta to pre in version string !4681
- Upgrade GitHub.com/BurntSushi/TOML to v1.3.2 !4695
- Docs Update - Missing Hosted Runner Renaming !4693
- Use fork of moby/spdystream to fix race condition !4685
- Fix typo in note !4691

## v16.10.0 (2024-03-21)

### New features

- [Experimental] Define monitoring threshold for job queue duration !4480
- Enable feature cancel_gracefully !4655
- Add support for cgroup_parent setting on Docker executor !4652 (Stefano Tenuta @stenuta)
- Add runner token to header !4643
- Add support for isolating jobs to individual namespaces !4519 (Markus Kostrzewski @MKostrzewski)

### Security fixes

- FedRAMP/CVE: Don't install wget in ubi images !4660

### Bug fixes

- Revert "Merge branch 'avonbertoldi/27443/graceful-shutdown-shell' into 'main'" !4686
- Build is now canceled if autoscaled instance disappears !4669
- Add jobs to compile all tests !4651
- Set UseWindowsLegacyProcessStrategy to false by default !4659
- Really silence error when `exec`ing on container that does not exists !4665
- Gracefully stop long running processes when using the shell executor !4601
- Call Value() instead of Get() when validating variables !4647
- Call Value() instead of Get() when validating variables !4647
- Fix get IP on parallels executor on macOS intel !4642 (Carlos Lapao @cjlapao)
- Fix fallback_key for local cache !4349 (Andreas Bachlechner @andrbach)
- Revert default runner script timeout !4621

### Maintenance

- Update windows support section !4641
- Upgrade fleeting library !4679
- Document connection_max_age parameter !4678
- Remove broken link to Kubernetes docs' emptyDir !4656 (Victor M. @victoremepunto)
- Bump Docker+machine version to v0.16.2-GitLab.25 !4676
- Document how to troubleshoot Docker Machine issues !4677
- Update plugin status, link timeline/epic !4674
- Runner: updates last two redirecting links !4675
- Fix typo (Telsa -> Tesla) !4673 (Per Lundberg @perlun)
- Enabling Vale for badges !4671
- Service_linux.go: Remove non-existent syslog.target from service file !4667 (Martin @C0rn3j)
- Refactor network.newClient to use Go functional option pattern !4648
- Finishes link fixes in Runner docs !4670
- Fix Experimental -> Beta references !4668 (Per Lundberg @perlun)
- Updating SaaS to be .com !4666
- Update runner sizes !4664
- Fix reference to project issue !4663
- Skip TestBuildGitFetchStrategyFallback on Windows cmd !4653
- Update no-trailing-spaces configuration for consistency !4658
- Remove unneeded trailing spaces !4644
- Skip TestBuildGitFetchStrategyFallback on Windows cmd !4653
- Restructure Kubernetes executor page part 2 !4650
- Restructure Kubernetes executor page !4649
- Add-vale-plugin-to-runner-dev-env-setup !4639
- Update usage of GCP to Google Cloud !4623
- Git_LFS_VERSION is no longer required to be set !4636 (Matthew Bradburn @mbradburn-ext)

## v16.9.1 (2024-02-28)

### Bug fixes

- Call Value() instead of Get() when validating variables !4647

### Maintenance

- Skip TestBuildGitFetchStrategyFallback on Windows cmd !4653

## v16.9.0 (2024-03-01)

### New features

- Add Kubernetes configurable retry limits !4618
- Support cancelation of job script with resumed execution of later stages !4578
- Add support for s3 accelerate in runner cache !4313 (ArtyMaury @ArtyMaury)
- Kubernetes: add automount_service_account_token option !4543 (Thorsten Banhart @banhartt)
- Register runner using Docker exec !4334 (Zhiliang @ZhiliangWu)
- Clear worktree on Git fetch failures !4216 (Tim @timmmm)
- Kubernetes: add option to skip explicit imagePullSecrets configuration !3517 (Miao Wang @shankerwangmiao)
- Limit number, memory and cpu of services container for Docker runners !3804 (Kevin Goslar @kev.go)
- Provide early build setting validation !4611
- Allow FreeBSD to be used with Docker executor (unofficially supported) !4551 (Ben Cooksley @bcooksley)
- Add support for service health checks for the Kubernetes executor !4545
- Limit the max age of a TLS keepalive connection !4537
- Retry on tls: internal error message for k8s executor !4608
- Retry on connection refused k8s error message !4605
- Increment package build number !4595
- Make Kubernetes API retries configurable !4523 (Michał Skibicki @m.skibicki)
- Add support for Node Tolerations overwrite !4566 (Marc Ostrow @marc.ostrow)
- Rewrite ci package script to mage !4593

### Security fixes

- Address CVE-2023-48795 - upgrade golang.org/x/crypto !4573

### Bug fixes

- Call Value() instead of Get() when validating variables !4647
- Correctly use volume driver for all volumes !4579 (Mitar @mitar)
- Revert default runner script timeout !4621
- Avoid recursing into submodules on checkout and fetch !3463 (Ciprian Daniel Petrisor @dciprian.petrisor)
- Fix edge case where Git submodule sync is not being called !4619
- Fix file variable quoting issue with cmd shell !4528 (Robin Lambertz @roblabla)
- Allow zero value for cleanup_grace_period_seconds !4617
- Use Windows job to improve process termination !4525
- Helper image container should always use native platform !4581
- Helper image container should always use native platform !4581

### Maintenance

- Skip TestBuildGitFetchStrategyFallback on Windows cmd !4653
- Fix warning event integration test !4633
- Retry package cloud push on HTTP error 520 !4635
- Allow explicit cpu/memory service resources overwrites for the Kubernetes executor !4626 (Tarun Khandelwal @tkhandelwal1)
- Fix rpmsign invocation by quoting all arguments !4632
- Fix verify stable resources job !4630
- Fix rpmsign invocation again !4631
- Fixing badge format !4629
- Explain side-effect of using pre_build_script commands !4627
- Create separate Kubernetes troubleshooting page !4622
- Vale and Markdown rule refresh for project !4620
- Update Go version to v1.21.7 !4458
- Check directives script ignores .tmp dir !4615
- Fix rpmsign command invocation !4614
- Retry bad gateway errors for package cloud jobs !4606
- Restore argument "NoProfile" for PowerShell in encoded command !4427 (Alexandr Timoshenko @saw.friendship)
- Add Apple Silicon support to Parallels Executor !4580 (Carlos Lapao @cjlapao)
- Update alpine Docker tag !4603
- Fully implement markdownlint-cli2 in project !4610
- Update Docker+machine version to v0.16.2-GitLab.24 !4609
- Add ~"Category:Runner Core"  to bug issue template !4612
- Housekeeping docs update !4604
- Resolve merge conflicts for Updated documentation S3 endpoints to support IPv6 !4602
- Remove removed feature from docs !4594
- Replace old handbook URLs !4554
- Change file name usage in docs per word list !4596
- Remove timeout and retry of package-deb and package-rpm jobs !4597
- Update version notes to new style - Runner !4590
- Update Harbor self-signed certificate x509: unknown Certificate Authority gotcha !4321
- Add specific steps on secret creation !4589
- Clean up docs redirects, runner - 2024-01-22 !4588
- Update persona links to handbook subdomain !4587
- Update `k8s dumb-init` FF doc to convey its support in both Kubernetes modes !4582
- Update `k8s dumb-init` FF doc to convey its support in both Kubernetes modes !4582
- Fix the architecture of pwsh x86-64 helper images !4559

## v16.8.0 (2024-02-27)

### New features

- Set default runner script timeout to allow after_script !4491
- Move PodSpec feature to beta !4568
- Allow IAM Session Tokens for S3 cache client credentials !4526 (Mike Heyns @mike.heyns)
- Add allowed_users config for Docker executor !4550
- Add GCP Secret Manager secrets integration !4512

### Bug fixes

- Revert default runner script timeout !4621
- Helper image container should always use native platform !4581
- Delete cache dirs after failed extraction !4565
- Truncate runner token so it won't get logged !4521 (Matthew Bradburn @mbradburn-ext)
- Allow empty string on emptyDir volume size !4564
- Support default paths on Windows for custom clone path !2122 (Ben Boeckel @ben.boeckel)
- Hide Docker executor init behind a feature flag !4488
- Revert "Add custom entrypoint for the build container for Kubernetes executor" changes !4535

### Maintenance

- Build images with `bleeding` Postfix rather than `main` !4583
- Use version instead of sha commit to reference helper images !4558
- Update glossary !4574
- Remove alpine 315 !4575
- Add alpine 3.19 !4561
- Fix FPM building RPM packages !4560
- Update `k8s dumb-init` FF doc to convey its support in both Kubernetes modes !4582
- Rebuild CI image !4576
- Change update to upgrade for 'Update GitLab Runner' !4572
- Add omitempty to allowed_users runner config spec !4571
- Helm documentation for ImagePullSecrets less confusing !4536 (Baptiste Lalanne @BaptisteLalanne)
- Document hostname length issue when using Docker-machine !4518 (Andrés Delfino @andredelfino)
- Removing docs Vale rule !4567
- Fix the architecture of pwsh x86-64 helper images !4559
- Create subtests for each allowed image !4540 (Zubeen @syedzubeen)
- Changing title to active verb !4563
- Updating title to be verb !4562
- Adding metadata descriptions !4556
- Document runner managers and system_ID !4549
- Add section for unhealthy configuration !4552
- Add `grep` as a dependency when overriding an image's ENTRYPOINT !4553
- Clarify / revise GitLab-runner SIGQUIT config !4548
- Update to go 1.21.5 !4541
- Add missing Docker configuration for Docker-autoscaler !4534 (Nabil ZOUABI @nabil_zouabi)

## v16.7.0 (2023-12-21)

### Bug fixes

- Helper image container should always use native platform !4581

### Maintenance

- Update `k8s dumb-init` FF doc to convey its support in both Kubernetes modes !4582

## v16.6.2 (2023-12-21)

### Bug fixes

- Revert "Add custom entrypoint for the build container for Kubernetes executor" changes !4535
- Improve the collapsible element logic !4487
- Avoid SIGTERM propagation to processes on Windows OS !4524
- Fix PowerShell native command error output with Kubernetes executor !4474 (Matthew Bradburn @mbradburn-ext)
- Use -File to improve pwsh exit status !4468 (Matthew Bradburn @mbradburn-ext)
- Add a better handling of signal on both Helper and Build container for k8s executor in exec mode !4485
- Fix broken main !4499
- Hide Docker executor init behind a feature flag !4488
- Hide Docker executor init behind a feature flag !4488
- Make TestDockerBuildContainerGracefulShutdown less flaky !4479

### Maintenance

- Update alpine Docker tag !4167
- Fix orphaned links for Autoscaling GitLab Runner on AWS EC2 docs page !3575
- Fix flaky resolver_URL_test.go due to lack of cleanup !4542
- Fix broken link !4539
- Troubleshoot more "No URL provided" cases !4502
- Move section in Kubernetes executor page !4538
- Update alpha to experiment in k8s executor page !4532
- Add support for Windows 11 23H2 !4504 (Matthew Bradburn @mbradburn-ext)
- Add troubleshooting for Docker connect failed !4516 (Matthew Bradburn @mbradburn-ext)
- Document limitation in gcs-fuse-csi-driver for mounting volumes in init container !4527
- Exclude empty slices during the validation of the config.TOML !4520
- Improve Docker executor platform option integration test !4489
- Add 204 error troubleshooting steps to the k8s executor docs !4508
- Upgrade fleeting and taskscaler !4510
- Add clarification about feature flags usage !4503
- Clarify ability to set other non-root user ids for k8s runner !4513
- Update "filename" to "file name" !4515
- Rewrite Image building to Mage to export them for verification !4295
- Update links to TW team handbook page !4511
- Generate k8s API permissions docs !4442
- Separate trace/job log streams !3983
- Delete docs marked for removal !4507
- Change RBAC option from "enable" to "create" !4506 (Chen Wu @wuchen)
- Clarify user membership for Docker !4498
- Change "Experiment` to Beta in supported public cloud instances table !4492
- Revert "Merge branch 'less-verbose-logging' into 'main'" !4496
- Make autoscaler integration tests pass !4497
- Make autoscaler integration tests pass !4497
- Cross-link Docker in Docker TLS configuration docs !4495
- Bump some test timeouts !4490
- Doc | Add new error to the troubleshooting section of instance executor !4475
- Improve formatting !4484 (Ben Bodenmiller @bbodenmiller)
- Clarify process tree in kuberenetes build container !4482
- Recommend a mountpoint other than /Users/Shared !4478 (Matthew Bradburn @mbradburn-ext)
- Retry package-deb and package-rpm when job times out !4481
- Bump some test timeouts !4471

## v16.6.1 (2023-11-24)

### Bug fixes

- Hide Docker executor init behind a feature flag !4488

### Maintenance

- Make autoscaler integration tests pass !4497

## v16.6.0 (2023-11-16)

### New features

- feat: allow specifying image platform to pull images !3916 (Muhammed Ali @ar-mali)
- Docker executor: Add configuration to include Docker's `--group-add` !4459 (Ben Brown @benjamb)
- Add custom entrypoint for the build container for Kubernetes executor !4394 (Baptiste Lalanne @BaptisteLalanne)
- Prevent logging every connection to the instance when using an autoscaler !4332 (Mattias Michaux @mollux)
- Add SizeLimit option to emptyDir volumes for Kubernetes executor !4410
- Enable Git transfer.bundleURI by default !4418

### Security fixes

- Update various images to use latest Docker-machine version !4454
- Update some dependencies to resolve vulnerabilities !4453

### Bug fixes

- Implement graceful build container shutdown for Docker executor !4446
- Add a better handling of signal on both Helper and Build container for k8s executor in attach mode !4443
- Add a mutex to sync access to sentryLogHook !4450 (Matthew Bradburn @mbradburn-ext)
- Use lchmod for zip extract !4437 (Matthew Bradburn @mbradburn-ext)
- Don't use Docker links for user-defined networks !4092
- Fix compilation of Kubernetes integration tests !4455
- Sanitize image entrypoint to remove empty string !4452
- Manually refresh JobVariables prior to ConfigExec !4379 (Paul Bryant @paulbry)
- Fix file secrets in debug terminal !4423 (Matthew Bradburn @mbradburn-ext)
- Fix labeling of the GitLab_runner_failed_jobs_total metric !4433
- Fix Azure key vault JWT convert bug !4396 (Zehua Zhang @zhzhang93)

### Maintenance

- Doc | Fix typo: rename key_pathname to key_path !4476
- Add a link to runner tutorial !4467
- docs: Use "prerequisites," plural (Runner) !4473
- Clarify PowerShell defaults !4470 (Matthew Bradburn @mbradburn-ext)
- Change Docker and instance executor from experiment to beta !4463
- Skip instance executor tests for cmd !4462
- Removed deprecated link !4461
- Use latest Technical Writing images !4449
- Misc test fixes !4460
- Add link to forceful shutdown definition !4445
- Add basic Azure instance/Docker autoscaler examples !4451
- Update versions in documentation !4457
- Update runner_autoscale_aws documentation with required iam:PassRole !4286 (Sjoerd Smink @sjoerdsmink)
- Add Docker Autoscaler and Instance executor integration tests !4402
- Refactor the retry interface to be generic !4422
- Update CI_IMAGE to include Debian image flavor !4447
- Fix sync_Docker_images test not building !4448
- Change instance, Docker autoscaler and AWS plugin to BETA !4432
- Update gocloud.dev to v0.34.0 !4430
- Doc | Add sample command for creating Docker machines for troubleshooting !4444
- Update imagePullSecrets documentation !4440
- Add upgrade troubleshooting info to Runner docs !3968
- Update information regarding new runner creation workflow !4436
- Merge "Example" page into register runners page !4413
- Add tip about No URL provided !4435 (Matthew Bradburn @mbradburn-ext)
- Set test build timeout to the DefaultTimeout value !4439
- Add a support policy page for GitLab Runner support policies !4434
- Reduce timeout for package-deb/rpm jobs to 30 minutes !4431
- Fix usage of 'build' !4429
- Fix formatting in Docker Autoscaler executor page !4428
- Clarify how FF_USE_POD_ACTIVE_DEADLINE_SECONDS works !4424 (Ben Bodenmiller @bbodenmiller)
- Update runner version reference !4426

## v16.5.0 (2023-10-20)

### New features

- Print Kubernetes Pod events !4420
- Support of multi-line command output in job terminal output view for bash shell when FF_SCRIPT_SECTIONS is enabled !3486

### Security fixes

- Install Git and Git-lfs via package manager in ubi.fips.base image !4405
- Run `apk upgrade` in runner alpine images !4378

### Bug fixes

- Docker-machine: Ensure runner stays under limit when IdleCount is 0 !4314
- When single-quoting, don't also quote with backtick with PowerShell/pwsh !4387 (Matthew Bradburn @mbradburn)
- Add config to autoset Helper Image ARCH and OS !4386
- Add missing findutils package to ubi-base image !4414
- Set `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` default value to `true` !4361
- Retrieve script exit command after execution !4397
- Add missing runtime packages to ubi base image !4359
- Fix the repository cloning error on Windows with `cmd` shell executor !4341
- Fix PowerShell SourceEnv permission failure !4369
- Fix PowerShell SourceEnv permission failure !4369

### Maintenance

- Display the stage command exit code when debug log is enabled !4421
- Fix docs typo !4419 (Alex @AJIOB)
- Downgrade CI image to use Debian bullseye instead of bookworm !4417
- Enhance debug secrets warning in documentation !4415 (Matthew Bradburn @mbradburn-ext)
- Add missing rbac when debugging services !4412 (Ismael Posada Trobo @iposadat)
- Docs: point users to Docker-machine fork that successfully handles EC2 fleet spot instance requests !4403
- Remove note on no-support for Windows system certificate store !4409 (Taisuke 'Jeff' Inoue @jeffi7)
- Remove spaces from FF_NETWORK_PER_BUILD environment variable example !4416
- Use latest linter image in relevant pipelines !4411
- Part 3: CTRT edits for registering runners !4392
- Upgrade Go to version 1.20.10 !4348
- Remove WithBrokenGitSSLCAInfo tests as they no longer function as expected !4408
- Update file Kubernetes.md !4393 (Thomas Spear @tspearconquest)
- Detail how to output stdout for WSL on windows !4370
- Add docs about Kubernetes overrides by CI variables !4222
- Lock `gitlab-dangerfiles` to 4.1.0 !4401
- Add link to Azure plugin releases to the instance executor documentation !4363
- Add link to Azure plugin releases to the Docker autoscaler executor !4364
- CTRT register runners prt2 v2 !4395
- Adding dial tcp timeout !4389
- Update documentation to reflect use of runner-token !4390
- Update PACKAGE_CLOUD variable default value !4342
- Improve documentation regarding runner unregistration !4338
- CTRT Part 1: Registering runners page !4371
- Add documentation issue template to project !4382
- Run apk upgrade in image used to build images !4381
- Style and language improvements for Advanced configuration docs !4377
- Improve error messages that are emitted by tasks !4344 (Taliesin Millhouse @strongishllama)
- Add links to Trusting TLS certificates paragraphs !4376
- Enable configuration of MTU in Docker executor !3576 (Jasmin @nachtjasmin)
- fix: Sets some http headers to use constants !4355
- Update default Git_LFS_VERSION !4372 (Matthew Bradburn @mbradburn)
- Git_LFS_VERSION must be specified when running make at the command line. !4360 (Matthew Bradburn @mbradburn)
- Fixed nvidia-smi typo !4367 (Alexander Hallard @zanda8893)

## v16.4.0 (2023-09-25)

### New features

- Add script/after script timeout configuration via variables !4335
- Distinguish job failure in worker processing failures metric !4304
- Expose queueing duration histogram metric !3499

### Security fixes

- Clean up manual installation of Git and Git-lfs in ubi base image and bump Git-lfs version to 3.4.0 !4289
- Runner-helper fips image cleanups !4308
- Bump Git-lfs version to 3.4.0 !4296
- Clean up manual installation of Git and Git-lfs in ubi base image and bump Git-lfs version to 3.4.0 !4289
- Runner-helper fips image cleanups !4308
- Bump Git-lfs version to 3.4.0 !4296

### Bug fixes

- Fix PowerShell SourceEnv permission failure !4369
- Fixed: init-permissions takes too long for windows volumes !4324 (OK_MF @OK_MF)
- Switch deletion propagation to background for Pod's dependents !4339
- Do not propagate Build context to k8s executor cleanup method !4328
- Fix error when unmarshaling string with windows path for PowerShell and pwsh !4315
- Automatically set GOMEMLIMIT based on memory cgroup quotas !4312
- Do not propagate Build context to k8s executor cleanup method !4328
- Fix error when unmarshaling string with windows path for PowerShell and pwsh !4315
- Update fleeting and taskscaler to newest versions !4303
- Forward URL rewrite in lfs pull !4234 (François Leurent @131)

### Maintenance

- Set FF_RESOLVE_FULL_TLS_CHAIN to false by default !4292
- Generate packagecloud packages with Mage !4323
- Fix pass env cmd test !4365
- Refactor content for Docker autoscaler executor page !4354
- Update runner registration token deprecation link !4357
- Enable pushing to ECR and DockerHub !4353
- Improve documentation about pod_annotations !4336
- Use ADD to download Windows dockerfile dependencies !4346
- Use ADD to download Windows dockerfile dependencies !4346
- Fix link in documentation to avoid redirects !4347
- Remove trailing whitespace from documentation !4343
- Discourage `gitlab-runner restart` within containers !4331 (Benedikt Franke @spawnia)
- Add info about config.TOML file !4333
- Update binary version !4330
- Remove configmap section !4329
- Fix FF_USE_PowerShell_PATH_RESOLVER env var value !4327
- Remove disclaimer from putting runner tokens in secrets !4319
- Update nav steps !4310
- Add note about empty runner-registration-token !4276
- Simplify issue templates and add labels !4275
- Fix links that are redirecting in docs !4311
- Add Openshift4.3.8 and later anyuid SCC !4306
- Add FIPS-compliant helper images and binaries to S3 sync job !4302
- Refresh Vale rules and link checking Docker image !4299

## v16.3.1 (2023-09-14)

### Security fixes

- Clean up manual installation of Git and Git-lfs in ubi base image and bump Git-lfs version to 3.4.0 !4289
- Runner-helper fips image cleanups !4308
- Bump Git-lfs version to 3.4.0 !4296

### Bug fixes

- Do not propagate Build context to k8s executor cleanup method !4328
- Fix error when unmarshaling string with windows path for PowerShell and pwsh !4315

### Maintenance

- Use ADD to download Windows dockerfile dependencies !4346

## v16.3.0 (2023-08-20)

### New features

- Enable variable injection to Persistent Volume Claim name !4256 (OK_MF @OK_MF)
- Add `http2: client connection lost` for k8s API retry !4285
- Add debug message to diagnose fetching issuer certificate bug !4274
- Add RISC-V support !4226 (Aaron Dewes @AaronDewes)
- Add link to documentation when using forbidden arguments in register !4266
- Add `connect: connection timed out` for k8s API retry !4257
- Put warning event retrieval feature behind a Feature Flag !4246

### Bug fixes

- Fix cmd escaping/quoting of parentheses !4301
- Revert "Prune tags when fetching" !4300
- Use Git --unshallow when Git_DEPTH is zero !4288
- Fix Docker Cleanup() panic when nothing has been configured !4287
- Mark project working directory as safe for Git !3538
- Only decode certificates if HTTP GET is successful !4281
- Panic during build now prints stack trace !4283
- Retry sync and update submodules on failure !4278
- Fix Docker ulimit validation warning !4248 (Dennis Voss @DennisVoss)
- Fix script typo that caused packages not to be pushed to Packagecloud !4253

### Maintenance

- Adding All to tier badges !4297
- Add RBAC as required config !4293
- Fix whitespace in docs !4291
- Fix typos !4284 (Sven Strickroth @mrtux)
- Include first multiline commit in MR description for default template !4282
- Update Docker dependencies version to fix invalid Host header !4249 (Sword @RryLee)
- Update fleeting and taskscaler to newer versions !4280
- Propagate Kubernetes executor context !4125
- Prune tags when fetching !4218 (Guilhem Bonnefille @gbonnefille)
- Rename runner token to runner authentication token !4264
- Fix documentation to work with Runner Helm Chart v0.53.0 onwards !4269 (Konstantin Köhring @konstantin.koehring)
- Provide guidance on minimal permissions needed for EC2 autoscaling !4175
- Doc | Add troubleshooting steps for private registry ssl errors !4267
- Update link to EKS !4268
- Add space before backtick !4265
- Add Vale to .tool-versions file !4252
- Add K8s and Docker logging location to troubleshooting !4262
- Add warnings about shell executor !4261
- Include steps to enable metrics for Runners using Helm Chart !4260
- Update installation type references for docs !4258
- Fix potential race condition in Docker provider test !4244
- Add missing release binaries/images to GitLab release page !4254
- Fix table item !4250
- Restructure executor page !4245
- Ensure Windows helper images builds fail upon error !4243

## v16.2.0 (2023-07-21)

### New features

- Update Runner package repository with OS availability !4215
- Add warning events on failure with k8s executor !4211
- Check forbidden arguments in register command before calling the server !4158
- Ignore forbidden arguments to register if using --registration-token !4157
- Retry all k8s API calls in the runner Kubernetes executor !4143
- Print number of jobs being processed for each new job !4113
- Added zip+zstd and tar+zstd archivers !4107
- Add Azure key vault support !3809 (Zehua Zhang @zhzhang93)

### Security fixes

- Do not install Python in ubi-fips-base image !4213
- Build Git-lfs from source in runner ubi-fips image !4206
- Update GitHub.com/Docker/distribution dependency !4205
- Upgrade Go version to 1.20.5 !4179
- Update `ubi8-minimal` image to version `8.8-860` !4171

### Bug fixes

- Downgrade Git from v2.41.0 to v2.40.1 !4236
- Fix misleading error when cache isn't configured !4212
- Fix common build dir implementation in instance executor !4209
- Add documentation to describe runner issue 30769 and its workarounds !4181
- Fix panic for instance executor when instance config option is nil !4173
- Kubernetes executor: prevent background processes from hanging the entire job !4162 (Snaipe @Snaipe)
- Fix Docker-autoscaler proxy tunnel for Windows !4161

### Maintenance

- Fix old metadata in docs !4240
- Refactor instance executor docs content !4238
- Fix Git LFS not building !4237
- Fix typo in Docker executor !4235 (Raphaël Joie @raphaeljoie)
- Mark integration_k8s as optional temporarily !4233
- Update documentation links !4232
- Update runner reg instructions in macOS setup !4230
- Add links to executor pages !4229
- Remove homebrew from docs to set up runner on MacOS !4227
- Build Git-lfs in the base UBI fips image as multiarch !4219
- Add Troubleshooting Case !4208
- Fix TestStackDumping flaky test and incorrect log output !4207
- Update vale rules and exceptions !4204
- Update text in runner registration page !4203
- Add note for limited config template setting support !4202
- Add documentation for SETFCAP configuration !4183
- Fix flaky k8s TestProcessLogs !4177
- Update to include Runner system requirements !4176
- Upgrade GitHub.com/MinIO/MinIO-go to v7.0.59 !4174
- Fixed outdated URL and type of variable !4168
- Add Crowdstrike troubleshooting guidance !4160
- Emphasize use of runnerToken in Helm chart !4150
- Mark ConfigExecTimeout as optional !4145 (Nikolay Edigaryev @edigaryev)
- Propagate build context !4128
- Add troubleshooting section about "permission denied" errors due to helper image user mismatch in k8s executor !3990

### GitLab Runner distribution

- Fix ECR and DockerHub sync !4180
- Fix windows servercore pwsh version and checksums !4178

## v16.1.0 (2023-06-21)

### New features

- Enable variable expansion in fallback cache keys !4152 (René Hernández Remedios @renehernandez)
- Automatically set GOMAXPROCS based on CPU Quotas !4142
- Allow Instance executor to use a common build directory !4136
- Pass clean command args to sub modules !4135 (Markus Ferrell @markus.ferrell)
- Add dedicated failure reason for image pulling failures !4098
- Support allowed images for privileged jobs and services !4089 (Stéphane Talbot @stalb)
- Variable expansion implementation in cache policy field !4085 (René Hernández Remedios @renehernandez)
- Use executor's context to enforce timeouts on VirtualBox commands !4026 (Patrick Pirringer @patrick-pirringer)

### Bug fixes

- Fix Windows IsRoot() path utility !4153
- Warn if runner with same token being registered multiple times !4122
- Upgrade taskscaler to latest version !4114
- Ensure lock for builds when listing jobs via debug API !4111
- Ensure instance connection is closed when vm isolation is enabled !4108
- Fix community Merge Request pipeline parse errors !4077 (Anthony Juckel @ajuckel)
- Fix cache keys processing by improving the handling of the fallback keys !4069 (René Hernández Remedios @renehernandez)

### Maintenance

- Docs maintenance: Delete trailing whitespace !4166
- Bump version of markdownlint-cli2 in project !4164
- Correct the filename of configmap !4163
- In UBI-fips helper images remove installation of extra packages since they are... !4159
- Fix k8s integration tests !4156
- Update code example with proper nesting !4155
- Expand Runner Helm chart troubleshooting section !4149
- Update documentation to mention that --registration-token is deprecated !4148
- Improve readability of table !4144 (Bastien ANTOINE @bastantoine)
- Upgrade fastzip to v0.1.11 !4141
- Update Runner docs for consistent SaaS runner naming !4138
- Docs maintenance: Update redirects !4134
- Refresh Vale and markdownlint rules !4133
- Add GitLab-runner section to values example !4132
- Removing podman references !4131
- Change heading used to describe reusing an authentication token !4129
- Refactor instance executor page !4124
- Correct example AWS zone used in an example !4123 (Nabil ZOUABI @nabil_zouabi)
- Improve formatting !4121 (Ben Bodenmiller @bbodenmiller)
- Mention use of runner tokens in Kubernetes runnerRegistrationToken !4120
- Follow up edits instance executor !4119
- Remove trailing spaces !4115
- Update project to use Ruby 3.2.2-based Docker images !4112
- Build Git from source for UBI images !4110
- Make GitLab network client respect Retry-After header !4102
- Documentation versions update !4096
- Improve cacheFile() errors !4078 (Nikolay Edigaryev @edigaryev)
- Update alpine and pwsh versions !4072
- Add info about grouped runners to docs !4056

### GitLab Runner distribution

- Sync ci images to dockerhub and ecr !4139

### Documentation changes

- Update nav step !4154

## v16.0.2 (2023-06-02)

### Bug fixes

- Upgrade taskscaler to latest version !4114

## v15.11.1 (2023-05-25)

### Bug fixes

- Fix cache keys processing by improving the handling of the fallback keys !4069 (René Hernández Remedios @renehernandez)

## v16.0.1 (2023-05-24)

### Maintenance

- Build Git from source for UBI images !4110

## v16.0.0 (2023-05-22)

### New features

- Add docs how to create an ephemeral PVC !4100
- Update autoscaler image handling !4097
- Send system_ID when unregistering runner !4053
- Consider node selector overwrites for the helper image !4048 (Mike Hobbs @mike554)
- Improve autoscaling executor providers shutdown !4035

### Security fixes

- Upgrade GitHub.com/kardianos/service to v1.2.2 !4105

### Bug fixes

- Close connection to instance on nesting client connect fail !4104
- Support health checking multiple service ports for Docker !4079
- Fix helper images being published with the wrong architecture !4073 (Anthony Juckel @ajuckel)
- Fix cache keys processing by improving the handling of the fallback keys !4069 (René Hernández Remedios @renehernandez)
- Unresolved secrets now return error by default !4064
- Expand container related options before they are used !4002

### Maintenance

- Update Git LFS checksums in release_Docker_images !4106
- Upgrade Git-lfs to 3.3.0 !4101
- Remove note on tested K8s's environments from Runner k8s docs !4087
- Upgrade GitHub.com/Docker/Docker to 23.0.6 !4086
- Remove section: Use the configuration template to set additional options !4084
- Upgrade GitHub.com/emicklei/go-restful/v3 to 3.10.2 !4082
- Update Windows version support policy for Runner !4074
- Fixed link that was breaking UI !4071
- GCP fleeting docs for Instance and Docker Autoscaler executors !4068
- Update alpha to experiment in executor page !4067
- Remove deprecated gosec-sast job !4065
- Update Docker-machine version !4061
- Remove reference to Docker-SSH and Docker-SSH+machine !4060
- Update GPG verification details !4059
- Upgrade GitLab-terminal dependency !4057
- Update grammar, remove extra word !4054 (Rasheed Babatunde @rasheed)
- Remove trailing whitespaces !4052
- Make clearer that force send interval is related to logs !4043
- Update redhat/ubi8-minimal Docker tag to v8.7-1107 !4025
- Update version of Docker-machine bundled in runner images !4024
- Add tests for internal autoscaler acquisition !4005
- Use Splitic test runner !3967
- Update golang Docker tag to v1.19.9 !3962
- Update alpine Docker tag !3918
- Remove lll linter !2837

### Documentation changes

- Add idle_time entry to docs !4093

## v15.11.0 (2023-04-21)

### New features

- Add Config Validation section to runner Docs !4017
- Add fine grained configuration of autoscaler histograms !4014
- Update runner internal metrics !4001
- Update taskscaler/fleeting metric collectors configuration !3984
- Reorganize index sections for runner use cases !3980
- Add high-level docs for Instance and Docker Autoscaler executors !3953
- Add Docker-autoscaler. !3885
- Implement fallback cache keys !3875 (René Hernández Remedios @renehernandez)
- Support remote Windows Docker environments from Linux hosts !3345
- Add support for absolute submodule URLs !3198 (Nejc Habjan @nejc)
- Support for custom Kubernetes PodSpec !3114

### Bug fixes

- Add hostname and find commands to UBI FIPS image !4040
- Remove stray omitempty in long form for --Docker-devices !4029 (Robin Voetter @Snektron)
- Interactive terminal: Wait for terminal to be set !4027
- Initialize nesting client lazily !4020
- Handle build's parent context cancelation correctly !4018
- Reduce config validation message noise !4016
- Try all specified TCP ports when doing a service container health-check !4010
- Fix Docker-machine detail races !3999
- Do not ask for registration token if runner token is specified !3995
- Explicitly start Docker service in windows tests !3994
- Resolve "Runner FIPS RPM packages conflicts itself" !3974
- Gracefully terminate Windows processes with Ctrl-C event !3920 (Chris Wright @inkychris)

### Maintenance

- Update Docker Autoscaler introduction milestone !4050
- Add missing code block end in docs/install/windows.md !4049 (Celeste Fox @celestefox)
- Add container support for Windows 2022 21H2 !4047
- Add reference to CI_CONCURRENT_PROJECT_ID variable !4046
- Remove Windows 21H1 !4045
- Add merge release config to bump the VERSION file after the stable branches are merged into main !4041
- Upgrade golang.org/x/net to v0.7.0 !4039
- Add troubleshooting of the error "unsupported Windows Version" for k8s on Windows !4038
- Experiment: Add reviewer roulette !4037
- Use Docker_HOST if set in the build time !4036
- Docker daemon change data-root directory !4034
- Post-merge edits for Executor pages !4033
- Make runner manager lowercase !4032
- Add GitLab Runner autoscaling page !4031
- Use a fixed time in register command integration tests !4023
- Update version in docs !4022
- Add note about runner registration permission !4021
- Fix flaky racy tests !4019
- Update index.md to remove typo in the second paragraph !4013 (vsvincent @vsvincent)
- Fix flaky TestDockerPrivilegedServiceAccessingBuildsFolder !4012
- Fix flaky interactive terminal test - ensure terminal connected !4011
- Temporarily skip Git-lfs for TestDockerCommandMultistepBuild !4009
- Remove comments metadata !4008
- Fix Test_Executor_captureContainerLogs race !4007
- Add note about Arm64 helper image for runner on arm64 Kubernetes clusters (docs) !4006
- Fix Docker-machine Windows tests !4003
- Re-use helper container for Docker executor's predefined stages !4000
- Improve troubleshooting documentation for the Job failed: prepare environment error with the Shell executor (docs) !3998
- Start prebuild stage earlier !3997
- Add a Runner glossary to the documentation (docs) !3996
- Remove note about selecting runner by name !3993
- Fix TestBuildOnCustomDirectory for PowerShell/pwsh !3992
- Only quote cmd batch strings where necessary !3991
- Use Ruby 3.2.1-based docs Docker images !3988
- Restructure registering runners page !3985
- Refactor executor setup/executor name function !3982
- CTRT edits Kubernetes part 4 !3963
- Drop extraneous "to" in feature flag docs !3946
- Update pipeline to depend on runner-incept passing !3940
- Improve layout with tabs !3894
- Update instructions to suggest go install rather than go build for building plugins !3819
- Building runner helper images with Windows nanoserver !3460 (Hoff_IO @82phil)

## v15.10.0 (2023-03-17)

### New features

- Change runner type "specific" to "project" !3979
- Configure external address usage for autoscaler provider readiness check !3973
- Use UBI Minimal for GitLab Runner UBI-FIPS image !3966
- Make the `gitlab-runner register` command happen in a single operation !3957
- Do not send system_ID in UpdateJob call !3925
- Best-effort config validation !3924
- Implement ability to parse JSON payload from /runners/verify !3923
- Add -y to apt-get install Git-lfs to prevent stalling the installation. !3921 (Antoon Huiskens @antoonhu)
- Handle registration for runners created in GitLab UI !3910
- Add support for activeDeadlineSeconds on CI Job Pod with k8s executor !3897
- Documentation for private fargate setup !3803
- Allow custom executor to specify the shell used !3789 (Robin Lambertz @roblabla)
- Allow configuration of environment variables for runner services !3784
- Docker executor: add services_security_opt config option !3760 (Glenn Dirkx @juravenator)
- Add API requests latency metric !3316
- Support for custom Kubernetes PodSpec !3114

### Security fixes

- Address vulnerability reports against runner-helper alpine images !3958
- Fix CVE-2022-1996 by upgrading k8s.io/client-go !3951

### Bug fixes

- Fix inconsiderate test !3971
- Fix non-amd64 alpine runner-helper images !3965
- Return BuildError from instance executor's client Run !3964
- Fix 'clear-Docker-cache' script for Docker 23.0 !3960
- Remove .runner_system_ID from Docker images !3950
- Remove re-writing config.TOML file on configuration reload !3934
- Add Windows Build Number to version mapping for Windows 2022 !3917
- Handle empty artifact paths !3912
- Execute the script from the right container !3900
- Shells/bash.go: set permissions before dir/file deletion !3726 (Karl Wette @karl-wette)

### Maintenance

- Fix TestBuildOnCustomDirectory for PowerShell/pwsh !3992
- Fix merge request link with missing '-' scope !3987 (Niklas @Taucher2003)
- Indicate that Command Line and Config.TOML are separate for debug !3986
- Fix missing parenthesis in the runners.Docker section !3981 (Tugdual Saunier @tucksaun)
- Fix Windows PowerShell encoding test !3977
- Fix flaky interactive terminal test !3975
- Slightly change message shown when .runner_system_ID cannot be written !3969
- Update SSL troubleshooting link !3961
- Remove link to Docker Machine on GitHub - docs !3956
- Fix failing fuzzing test !3955
- Use Labkit for FIPS check !3954
- Kubernetes executor CTRT edits part 3 !3949
- Corrected minor typo !3948
- Bump Ubuntu version, ease quickstart with Runner !3947
- CTRT edits Kubernetes executor part 2 !3944
- Use latest docs Docker images !3941
- Fix deprecation notice legal disclaimer !3936
- Update Docker engine client to v23.0.1 !3935
- Remove reference to GitLab Runner 10 [docs] !3933
- Add container images support lifecycle [docs] !3931
- CTRT refactor for Kubernetes executor page part 1 !3928
- Fix typo in the post_clone_script deprecated warning message !3927 (Tamás Dévai @dorionka)
- Remove overview heading from shell docs !3926
- Avoid running 1809 integration tests in CC !3922
- Language edits for "Automate keeping up to date with packagecloud release"" !3914
- Add troubleshooting item for background processes and hung job !3913
- Update golangci-lint version to 1.51.2 !3911
- Update the URL for the Docker-machine version from .11 to .19 !3909
- Update taskscaler version in GitLab-runner !3903
- Fix Warning log during prepare stage for the Kubernetes executor !3902
- Add type::feature as a new feature section for changelog !3898
- Expand and consolidate Git LFS docs for non-Docker executors !3892 (Nejc Habjan @nejc)
- Upgrade Go version to 1.19.6 !3889
- Update documentation links for pod security context !3823
- Add step to enable linger to GitLab-runner !3688 (Peter Harsfalvi @peterh.six)

## v15.9.1 (2023-02-20)

### Bug fixes

- Remove re-writing config.TOML file on configuration reload !3934

## v15.9.0 (2023-02-19)

### New features

- Ignore glrt prefix for runner short token !3888
- Log artifact download request host !3872
- Use taskscaler and nesting slots !3818

### Bug fixes

- Handle empty artifact paths !3912
- Execute the script from the right container !3900
- Update removal milestone in warning log message for step_script !3893
- Generate random system_ID for run-single mode !3881 (Helio Machado @0x2b3bfa0)
- Clarify checking out message to reduce confusion !3880
- Allow runner to start when config directory is not writeable !3879
- Fix bug with project dir not resolving as in the project !3877
- Use JobVariable.Value() for internal values !3870
- Prevent masking panic by ignoring zero length secrets !3869
- Sending debug_trace param on PATCH job_trace requests !3857

### Maintenance

- Fix misspelling in documentation !3896 (Shafiullah Khan @gitshafi)
- Add additional test coverage around path matching for artifacts (doublestar) !3890
- Add documetnation for shutdown_timeout config.TOML setting !3887
- Update Docker Machine installed in runner container image !3886
- Upgrade GitHub.com/BurntSushi/TOML !3883
- Clarify the use of --version when installing the Helm chart !3882
- Fixed wording for help command in docs !3878 (E Jo Zim @designerzim)
- Use new Ruby 3.0.5-based Docker images !3876
- Drop Windows exemption for warning about system cert pool !3871
- Improve Docker Machine executor finish message !3868
- Add link to all metrics available !3867
- Update documentation about helper image being pushed on dockerhub !3866
- Update documentation to highlight access to CI Variables from container entrypoint with k8s executor !3865
- Add backticks to fix kramdown warning !3864
- Reduce log level to reduce noise in logging !3863
- Clean up docs redirects, runner - 2023-01-23 !3861
- Add metrics for counting configuration file access !3859
- Handle the content of the new pre_get_sources_script and post_get_sources_script job payloads in Runner !3858
- Use latest docs linting images for project !3856
- Update always policy to match the Docker wording !3851
- Log type of shell when using Shell executor !3850 (Anatoli Babenia @abitrolly)
- Add default annotations to Kubernetes build pods !3845 (Adrian Rasokat @adrian.rasokat.tui)
- Update removal milestone in deprecation warning !3844
- Document requirement for Docker executor image ENTRYPOINT to support sh/bash COMMAND !3839 (Pierre Beucher @pbeucher)
- Update golang Docker tag to v1.18.10 !3828
- Docker executor CTRT part 4 !3826
- Automate keeping up to date with packagecloud release !3821
- Automatically set Alpine and Ubuntu version defaults in make !3816
- Warn about exceeding the global concurrency limit when setting up a new runner !3797
- CTRT Docker executor part 2 !3788
- Make external address usage configurable !3783
- Update redhat/ubi8 Docker tag to v8.7-1054 !3764
- Add support for setting procMount of build container !3546 (Alex Wied @alex-cm)

### Documentation changes

- Change removal date to 17.0 for GitLab-runner exec (docs only) !3884

## v15.8.0 (2023-01-19)

### New features

- Add system_ID to Prometheus metrics !3825
- Send system_ID in jobs requests !3817
- Prepare register command to fail if runner server-side configuration options are passed together with a new glrt- token !3805
- Add nesting client to support VM-isolated build environments !3654
- #27863 Add mac address with isolation !3454 (Artem Makhno @artem.makhno.softsmile)
- Display system_ID on build log !3852

### Bug fixes

- Fix doublestar implementation to use paths relative to working directory !3849
- Fix windows integration tests failure check !3846
- Re-merge "Artifact/cache helpers now use POSIX shell syntax for expansion" !3833
- PowerShell: fix unwanted progress streams leaking to output !3831
- Fix skipped windows integration tests !3830
- Fix relative URL path handling with clone_URL !3815
- Prevent new autoscaler thrashing instances !3813
- Add a check for any artifact paths that do not fall within the project directory or its subpaths !3757
- Use exec mode to create the scripts in attach mode !3751
- PowerShell: Fix stdin handling with scripts !3843

### Maintenance

- Revert "Fix go.mod to downgrade doublestar to v1.3.0 to be same as main" !3842
- Add pwsh to supported shells for Docker-windows executor !3829
- `--url` is GitLab instance URL, and not the address of the runner !3807 (Anatoli Babenia @abitrolly)
- Revert "Merge branch 'avonbertoldi/29451/pkgcloud-auto-versions' into 'main'" !3794
- Bump the k8s integration test timeout to 15m !3787
- Make runner support multiple service aliases !3550 (Alessandro Chitolina @alekitto)

### GitLab Runner distribution

- Start pushing Helper images to DockerHub again !3847

### Documentation changes

- Include reference to build pod configuration documentation !3848
- Add PowerShell to proper names list & minor formatting fixes !3837 (Ben Bodenmiller @bbodenmiller)
- Fix Git for Windows casing !3836 (Ben Bodenmiller @bbodenmiller)
- Improve wording !3835 (Ben Bodenmiller @bbodenmiller)
- Clarify that GitLab-runner is required for both download and upload !3834 (Dillon Amburgey @dillon4)
- Clarify variable type !3824
- Docs surround Kubernetes_ values with quotes !3820
- Documented how to protect environment variable in Kubernetes executor !3812
- Add clarifications for k8s pull policies !3811
- Fix kramdown warning issue !3808
- Update GitOps workflow warning !3806
- CTRT edits for Docker executor part3 !3802
- Adding namespace to anyuid command !3798
- Update fargate troubleshooting !3772
- Update using security context example !3723

## v15.7.3 (2023-01-19)

### Bug fixes

- PowerShell: Fix stdin handling with scripts !3843

## v15.7.2 (2023-01-13)

### Bug fixes

- Fix relative URL path handling with clone_URL !3815
- PowerShell: fix unwanted progress streams leaking to output !3831
- Re-merge "Artifact/cache helpers now use POSIX shell syntax for expansion" !3833

## v15.7.1 (2022-12-19)

### Bug fixes

- Revert automate for which supported distro releases we create packages. !3794

## v15.7.0 (2022-12-17)

### New features

- Add PrivilegedServices option for allowing/disallowing Docker services to be privileged !2652
- Add support for Windows Server 21H2 !3746
- Generate global system ID !3758
- Add start_type to virtualbox configuration !2558
- Update secret resolver to return raw & masked variables !3750
- Allow Executors to clone via SSH !3518
- Add Docker support for `IpcMode` for IPC namespace sharing !3781
- Expose the build timeout as an environment variable !3778
- Improve Runner's API health checking and handling !3658

## v15.6.3 (2023-01-19)

### Bug fixes

- PowerShell: Fix stdin handling with scripts !3843

## v15.6.2 (2023-01-13)

### Bug fixes

- PowerShell: fix unwanted progress streams leaking to output !3831

## v15.6.1 (2022-11-24)

### Bug fixes

- Fix cache config needing to be provided !3747
- Add GitLab-runner user during ubi-fips image building !3725
- Fix Kubernetes pod labels overwrite !3582
- Correctly handle expansion of job file variables, and variables that reference file variables !3613
- Artifact/cache helpers now use POSIX shell syntax for expansion !3752

### Maintenance

- Upgrade GitHub.com/urfave/cli to 1.22.10 !3744
- Unit test to catch urfave bug !3749
- Makefile.build.mk: allow building for arm64 without overriding ARCH !3498
- Renovate Go version !3768
- Add warning about using SIGTERM/SIGINT over SIGQUIT !3769
- Update golang Docker tag to v1.18.9 !3776
- Automate for which supported distro releases we create packages. !3756
- Fix silent Docker images build failure and retry buildx !3786
- Rename Docker's PrivilegedServices to ServicesPrivileged !3791

### Documentation changes

- Making things a little more obvious for those of us who may skip ahead !3697
- Clean up docs redirects, runner - 2022-11-23
- Document behavior for local addresses in [session_server] configuration !3676
- Docs: Nested guidelines for clarity !3729
- Fix some wording in docs and add links in convenient areas !3684
- Updated serviceaccount setting to match the code !3387
- Update agent for Kubernetes installation docs !3748
- Change deprecation documentation for register command !3742
- Make pod_labels more specific !3645
- Added doc to inform about saving cost when using private subnets and AWS S3 cache !3453
- Add more descriptive headings on executor pages !3763
- Add security warning to Runner install docs !3762
- Add troubleshooting details !3755
- Add note for self-managed customers !3761
- Update docs/executors/virtualbox native OpenSSH PowerShell !3775
- Fix Kubernetes Executor docs !3770
- Add note for AWS IAM instance profile !3774
- Add a requirement to create a namespace before overwriting !3696
- CTRT edits for The Docker executor part 1 !3753
- Expanded on downloading helper images and updated a link to use a more modern file. !3562
- Add `deprecated` to `gitlab-runner exec` !3773

## v15.6.0 (2022-11-21)

### New features

- Add support for Node Selector Overwrite !3221
- Handle job execution interruption for the new autoscaler executor provider !3672
- Add maximum size to uploaded cache !3552
- Allow multiple paths in Git_SUBMODULE_PATHS !3675
- Capture helper service logs into job/tasks main trace log !3680
- Add a feature flag to disable resolving of TLS chain !3699
- Adds proper handling of ExecutorProviders initialization and shutdown !3657

### Bug fixes

- Detect Windows build 10.0.19042 as 20H2 !3694
- Force PowerShell/pwsh input/output encoding to UTF-8 !3707
- Skip non-regular files for artifact metadata generator inclusion !3709
- Filter Kubernetes trace to remove newline added for long logs in attach mode !3691
- Enable PowerShell via stdin by default !3728
- Kubernetes executor: redial backend on internal server errors !3732

### Maintenance

- Update redhat/ubi8 Docker tag to v8.7-929 !3738
- Add OS versions supported by packagecloud 3.0.6 release !3734
- Add tests for Kubernetes scheduler name config !3643
- Update Go distribution to version 1.18.8 !3720
- Update logging levels from Debug to Info !3710
- Move autoscaler Acquire() to the ExecutorProvider !3660
- Document internal Executor Interface !3291
- Update Git to 2.38.1 and Git-lfs to 3.2.0 to address CVE-2022-29187 !3674
- Switch to markdownlint-cli2 !3683
- Ensure `go-fips` container is rebuilt when the version of Go is updated !3685
- Add logging in UpdateJob to include checksum and bytesize !3693
- Update taskscaler to newer version !3706
- Skip Docker Test_CaptureServiceLogs integration tests on windows !3703
- Update GoCloud to v0.27.0 and update Azure cache to use new SDK !3701

### Documentation changes

- Explain ANSI-relevance of log_format options !3739
- Fix broken links in runner docs !3737
- Add podman-plugins package dependency for service container network aliases !3733
- Add Taskscaler and Fleeting plugin instructions to Runner development !3730
- Document macOS workaround for TLS issues !3724
- Remove misleading statement regarding Bash in Windows planned feature support !3722
- Deprecate register command !3702
- Mark runnerRegistrationToken as deprecated !3704
- Add Helm repo update command to Kubernetes install docs !3736
- Add additional documentation around the use of submodules !3670
- Add Kubernetes certificate guide !3608
- Troubleshooting for pods always assigned worker node's IAM role !3678
- Change $shell to $SHELL in "Set up macOS runners" docs !3681
- Fix docs review app script and domain !3682
- Update redirected links in the runner docs !3690
- Improve development setup docs !3661
- Update Runner Helm chart docs to include list of deprecated fields !3686
- Add details to Documentation MR template !3698
- Adding Ubuntu 22 to the supported OS list !3712
- Adds deprecation notes for Docker-SSH and Docker-SSH+machine executors !3714
- Updated template to match other repo !3715

## v15.5.1 (2022-11-11)

### New features

- Add a feature flag to disable resolving of TLS chain !3699

## v15.5.0 (2022-10-21)

### New features

- Add shell+autoscaler executor !3617
- Add Docker volume driver ops !3620
- Kubernetes executor: support podspec.schedulerName !2740
- Add IPv6 support to Docker networks !3583
- Add Prometheus metrics to executor autoscaler !3635
- Add Git_SUBMODULE_DEPTH variable !3651
- Add support for PAT masking in trace !3639

### Bug fixes

- Set all existing variables into the build container !3607
- Add pgrep to ubi-fips image !3625
- Standardize Attestation Artifact Names and Permissions !3650
- Do not expand some CMD variables <https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38>

### Maintenance

- Upgrade Go to version 1.18.6 !3589
- Add TMPDIR to test's env allowlist !3603
- Go 1.18 mod tidy !3619
- Drop runtime.GC() after every check !3595
- Upgrade Go FIPS image version to 1.18 !3624
- Add internal autoscaler executor provider unit tests !3633
- Only generate mocks that are actually used in tests !3630
- Fix incorrect spelling of acquisition !3621
- Add User config setting for Docker executor !2913
- Upgrade Go FIPS image version to 1.18.7 !3640
- Upgrade Go distribution to version 1.18.7 !3656

### Documentation changes

- Added GitLab Runner to title !3618
- Clarify k8s executor overrides per CI/CD job !3626
- Add note about Docker-in-Docker !3628
- Fix indentation for [runners.cache] in Kubernetes docs !3634
- Clean up docs redirects !3632
- Document hidden retry for failed Docker pull !3638
- Refactor autoscaler terminology !3641
- Update redirecting external links for Runner !3631
- Explain metric …request_concurrency_exceeded_total !3558
- Update contribution details when it requires changes to both GitLab and Runner !3649
- Disk root size parameter !3652
- Remove Grafana dashboard link !3653
- Move Content from best_practices page !3665
- Remove content that didn't add value !3667
- Updated path for group runners !3664
- Fix ordered list display abnormal error !3663
- Set variable to new domain for docs review apps (Runner) !3671

## v15.4.2 (2022-11-11)

### New features

- Add a feature flag to disable resolving of TLS chain !3699

## v15.4.1 (2022-10-21)

### Security fixes

- Do not expand variables in Command <https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38>

## v15.4.0 (2022-09-21)

### New features

- Add renovate support !3592

### Bug fixes

- Reset token in config template when set !3593
- Remove reliance on text/transform for trace masking !3482

### Maintenance

- Update instructions with new menu title !3599
- Update project for latest Vale and markdownlint tooling and rules !3598
- Docs: Small edit to language !3596
- Updated title to match left nav !3588
- Delete tmp/GitLab-test directory. !3585
- Updated title to match our standards !3584
- Allow setting of Docker volume label mode independent of read/write mode !3580
- Improve clarity of runner metrics examples !3578
- Remove 'respectively' and 'please note' !3574
- Add io error to troubleshooting section !3573
- Docs: Adding details about GitOps configuration for agent !3572
- Fix runners location in docs !3555
- Add path implementation to support Windows Docker from unix !3344
- Update redhat/ubi8 Docker tag to v8.6-943 !3605
- Update alpine Docker tags !3604

### Security fixes

- Upgrade Prometheus/client-golang from v1.1.0 to v1.11.1

## v15.3.3 (2022-11-11)

### New features

- Add a feature flag to disable resolving of TLS chain !3699

## v15.3.2 (2022-09-21)

### Security fixes

- Do not expand variables in Command <https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38>

## v15.3.1 (2022-09-21)

### Security fixes

- Upgrade Prometheus/client-golang from v1.1.0 to v1.11.1

## v15.3.0 (2022-08-19)

### New features

- Improve documentation about installing and using Podman as a Docker executor replacement !3570
- Add support SELinux type label setting in Kubernetes executor !3451 (Omar Aloraini @ooraini)
- Add a check whether boringssl is being used by using the Enabled method !3390
- Add support for server side encryption for S3 Cache !3295 (Johan Lanzrein @lanzrein)
- Remove CentOS 6 packaging !2871 (Bene @bene64)

### Bug fixes

- Generate artifacts metadata only for zip !3565
- Build s390x images alongside the other images !3561
- Ensure that runner always uses the customized User-Agent !3543
- Revert GitHub.com/urfave/cli back to v1.20.0 !3539
- Improve error message when there's a conflict between `pull_policy` and `allowed_pull_policies` settings !3526
- Sanitize user-provided custom build directory before passing it forward !3360

### Maintenance

- Docs: Remove old install page !3563
- Update default label for documentation MR template !3559
- Promote GitLab.MultiLineLinks to error !3554 (Niklas @Taucher2003)
- Fix links split across multiple lines in Runner repo !3553
- Add note on GitLab instance pre-requisite for using Runners - docs !3549
- Update markdownlint and Vale configuration !3548
- Fix "broken" links (redirect) !3542 (Lee Tickett @leetickett)
- Add `hostname` to the UBI-fips helper image !3540
- Docs: Fix a typo in `pull_policy` which is should be underscore !3537
- Update linter version to 1.46.2 !3536
- Update Helm chart troubleshooting for missing secrets !3534
- Protect commands/config with a mutex !3507
- Fix dead link & other runner docs cleanup !3491 (Ben Bodenmiller @bbodenmiller)

### Documentation changes

- Remove premium tier from agent install docs !3535
- Add new functionality related to runner token expiration !3209 (Kyle Edwards @KyleFromKitware)

## v15.2.0 (2022-07-20)

### Bug fixes

- Update GitHub.com/containerd/containerd dependency !3525
- Rename DEBUG env var to RUNNER_DEBUG !3497

### Maintenance

- Push image on registry during release stage only when enabled !3528
- Fix version history formatting !3523
- Upgrade Go to 1.17.9 in project !3515
- Disable push to ECR in all cases !3514
- Make resource checking disabled by default !3513
- Fix DEB_PLATFORMS definition in the Makefile !3510
- Monitor Docker-machine provision failed state !3355 (StoneMan @Wenyuyang)
- Run incept tests only for canonical namespaces !3341

### Documentation changes

- Update command usage and GitLab Runner version !3531
- Restore previous step for freebsd install procedure !3527
- Fix link to cluster agent !3521
- Add explanation on how to select runner manager node with nodeSelector !3520
- Update sysrc command for Freebsd installation procedure
 !3519 (Roller Angel @rollerangel)
- Add security context for init permissions container !3516
- Add note about configurability of Fargate host properties !3509
- Remove columns to correct rendering config.TOML, CLI options and ENV variable for the register table !3508
- Add the pull-policy from jobs support to Kubernetes !3504
- Remove trailing spaces from docs !3502
- Add note for pre existing runner use condition !3501
- Improve the output of registration command !3500
- Fix description of 'Coordinator' in FAQ !3496
- Add some clarifications to how job_env in Custom Executor works !2810

## v15.1.0 (2022-06-20)

### New features

- Generate artifacts metadata !3489
- Add image pull-policy support to services !3488

### Bug fixes

- Init submodules prior to sync to ensure submodules remote URL configuration is properly synchronized !3265 (David Alger @davidalger)
- Honor entrypoint for build and helper images with exec passthrough !3212 (bdwyertech @bdwyertech)

### Maintenance

- Ignore TestPowershell_GetConfiguration for all windows versions !3494
- Add TestPowershell_GetConfiguration/pwsh_on_shell_with_custom_user_(windows)... !3492
- Update Docker images for linting docs !3490
- Add note about GitLab-runner-fips !3487
- Update MinIO-go dependency to fix FIPS endpoints !3484
- The context of the language would suggest the plural form of this noun. !3483
- Fixed a broken link for FIPS RHEL runner !3481 (Brock R @fearthebadger)
- Clarify on Docker engine version requirement !3479
- Expand variables for Pod volume subPath and mountpath config !3478
- Update documentation on interactive web terminal support for Helm chart !3477
- Add upgrade code sample for arm64 !3475
- Fix error in oc create configmap command - docs !3471
- Remove windows server 2004/20H2/21H1 related tests from community MR pipeline !3467
- Do not retry artifact download on 401 response !3461
- Modify doc mentions of RedHat to Red Hat !3459 (lousyd @lousyd)
- Update project to use latest linting images from GitLab-docs !3452
- Use `T.TempDir` to create temporary test directory !3410 (Eng Zer Jun @Juneezee)
- Use 'go install' instead of 'go get' to install tools !3402 (M. Ángel @jimen0)
- DeviceCgroupRules for Docker Executors !3309 (Alexander Sinn @embeddedcoder)
- Workaround to slow artifacts upload to GCS !3194
- Add extra information when "no matching files" !3079 (Adrian Mârza @adrian.marza.mambu)
- Override ci image and registry for all windows helper pushing jobs !3485
- health-check port discovery should be consistent with WAIT_FOR_SERVICE_TCP_PORT !3033 (Anton Neznaienko @neanton)

### GitLab Runner distribution

- Trigger UBI images for all releases and main branch !3466
- Fix not pushing main Runner images to Docker hub !3465

### Documentation changes

- Add Podman configuration steps !3480
- Implement allowed_pull_policies in config.TOML !3422
- Implement supporting pull_policy from jobs !3412
- Allow to overwrite Pod labels in the Kubernetes executor !3352 (Mathieu Parent @sathieu)
- Add a flag to `gitlab-runner exec` to specify the CI/CD config file !3246 (Alexis Jeandeau @jeandeaual)
- Use GCP metadata server and sign blob API for GCS cache URL !3231 (Jasper Maes @jlemaes)
- Complete the example configuration for gcp cache !2956 (Edward Smit @edwardsmit)
- Support Priority Class Name for Kubernetes executor !2685 (ayoub mrini @ayoubmrini424)

## v15.0.0 (2022-05-19)

### Security fixes

- Improve sensitive URL parameter masking !3404

### Bug fixes

- Allow S3 cache's AuthenticationType to be provided case-insensitively !3446

### Maintenance

- Update Git-lfs to 2.13.3 !3458
- Add TestMachineIdleLimits in the windows 21h1 test failure !3457
- Repair redirected links !3456
- Add history to docs for Kubernetes pull policy !3455 (Raimund Hook @stingrayza)
- Run bleeding edge windows builds for security pipelines as well !3449
- Fix minor grammatical error. !3448 (Crafton Williams @crafton)
- Fix windows 21H1 pushing helper images and integration tests !3447
- Delete trailing whitespace !3443
- Fix alpine-latest pipelines for pwsh and prevent this happening on main in the future !3442
- Moved content to executor pages !3440
- Add instructions for how to specify what user a job is run as via Docker executor !3438
- Update alpine versions to latest !3436
- Parallelize Kubernetes TestRunIntegrationTestsWithFeatureFlag tests !3435
- Update FIPS base UBI image to 8.6-754 !3434
- Add alpine-latest helper image flavor and switch default alias to 3.15 !3433
- List source of Default templates !3431 (Ben Bodenmiller @bbodenmiller)
- Switch from cobertura to coverage_report keyword !3429
- Stop publishing helper images to Docker Hub !3425
- Add a note to troubleshooting section regarding security release !3424
- Set max_line_length attribute in .editorconfig !3423
- Fix 21h1 hcsshim::CreateComputeSystem error !3421
- Fix indentation for Docker run runner example !3419
- Register runner with renamed paused argument !3414
- Enable CGO_ENABLED by default in golang-fips compiler !3413
- Change amazonec2-security-group to XXXX in example !3411
- Check serviceaccount and imagepullsecret availability before creating pod !3399
- Make clear-Docker-cache script to work for Docker versions below 17.06.1 !3394 (Roland Hügli @rhuegli)
- Servername in openssl command !3374
- Update index.md !3356 (Don Low @Don.Low)
- Docs: Small edit to change 'how' to 'what' !3325
- Update docs/monitoring/index.md !3216
- Expose fastzip configuration options !3130
- Docs: Update autoscale_aws_fargate to include ca certificate location !2625
- Print out service timeout seconds in Docker executor !279 (Elan Ruusamäe @glensc)

### GitLab Runner distribution

- Add packages added by package cloud 3.0.5 release !3437
- Use SHA256 instead of MD5 for digest !3415

### Documentation changes

- Add step for AppSec in the security release template !3432
- Make explicit disabling of strict-host-key-checking mandatory by default !3418
- Add support for Windows server 2022 !3218
- Add sh to --shell --help following documentation !2988 (David Hannasch @dHannasch)

## v14.10.1 (2022-05-02)

### Security fixes

- Disallow reserved CACHE_FALLBACK_KEY values !49

## v14.10.0 (2022-04-19)

### Bug fixes

- add tip for windows Docker permissions !3397
- Add newline between the command and the output when collapsible section is enabled !3389 (Thomas Chandelle @tchandelle)
- Increase token short length if it includes prefix !3373

### Maintenance

- Update lint-Markdown image for docs !3408
- Remove explicit mention of t4g.nano !3405
- Log object storage forbidden errors during artifact downloads !3400
- Change release milestone for k8s operator - docs !3395
- Link macOS install docs to config docs !3392
- Add runnerImage property to OpenShift Operator docs !3385 (Em Karisch @QuingKhaos)
- Artifacts download argument validation !3384
- Added how to fix TLS handshake timeout error in a proxy environment !3383
- Fix a typo in the cache uploading messaging !3382 (Lee Tickett @leetickett)
- Add new troubleshooting step to the Kubernetes docs !3380
- Change the docs review apps IP !3379
- Debian 9 won't build / qemu now requires -F !3369 (Donny Davis @donnydavis)
- Add support for Docker client version negotiation !3322
- docs: update region specific s3 endpoint urls !2975 (Casey Vockrodt @casey.vockrodt)
- Add archiver staging directory option to runner helper !3403

### GitLab Runner distribution

- Add amazon/2 RPM distribution to the release list !3378

### Documentation changes

- Add Kubernetes operator installation and uninstallation docs and updated OpenShift docs !3388
- Add runner registration related properties to OpenShift Operator !3386 (Em Karisch @QuingKhaos)
- Support Docker container custom labels !3304 (aylinsenadogan @aylinsenadogan)
- Update release process link in readme !3319 (Théo DELCEY @theodelcey)

## v14.9.0 (2022-03-21)

### New features

- Add posix shell quoting implementation !3367

### Bug fixes

- Use token from job payload when composing repository URL based on clone_URL !3366
- Upgrade MinIO to v7.0.24 pre-release, for IAM timeout fix !3354
- Upgrade fastzip to v0.1.9, fixes invalid timestamps !3353
- Update network responses to support 64-bit Job IDs !3346
- Upgrade fastzip to v0.1.8 !3333
- Allow changing shell executor with pwsh user !3298
- Remove bashisms from Bash shell implementation !3014 (Neil Roza @realtime-neil)

### Maintenance

- Update stringData for Custom TLS cert !3372
- Add default issue & MR templates !3368
- Docs: Added fleet management link !3364
- Add link to AWS driver docs in GitLab Docker machine - docs !3363
- Change fleet scaling to best practices for runner shared services - docs !3362
- Docs: Kubernetes volumes are mounted on services !3361 (Quentin Barbe @forty1)
- Add warning about enabling debug logging !3359
- Add links to clarify AWS and Docker credentials requirements and clarification on image: tag !3358
- Add link to Docker machine fork - docs !3357
- Edited for style !3351
- Run trigger-UBI-images-build job also for patch release tags !3350
- Update runner registration failure log message !3349
- Add runner registration message section - docs !3348
- Move Path interface to Docker volume consumer !3343
- Neaten helpers/path unix path impl !3342
- Fix misleading error during cache restoration !3340
- Clean up docs redirects - 2022-02-22 !3339
- Make SSH command/executor shell agnostic !3337
- Remove redundant shell config environment property !3336
- Updated agent for Kubernetes !3334
- Update CI toolchain versions !3330
- Upgrade Docker to 20.10.12 !3328
- Support Vault EE namespaces !3320 (Aleksander Zak @aleksanderzak)
- Add Debian bullseye to supported versions !3318
- Add post_clone_script hook !3211 (Dan Rice @dnrce)
- Docs: Update Kubernetes key file format !3097 (Brandon Hee @brandonhee)
- fix grammatical error !2896 (James Dube @jamesdube)

### GitLab Runner distribution

- Fixes version definition in VERSION file !3371
- Align Debian releases for stable and Bleeding Edge versions !3335

### Documentation changes

- Add support for Kubernetes runtime class !2326
- Add docs about security risks for using cache and the Git_strategy=fetch !3365

## v14.8.0 (2022-02-20)

### New features

- Allow specifying maintenance-note on runner registration !3268
- Support Apple Silicon (darwin/arm64) !2274
- Add variable support for services (Stefano Tenuta @ST-Apps1) !3158

### Bug fixes

- Fix artifacts upload redirection support !3308
- Handle redirects on artifact uploads !3303
- Introduce non-reusable Docker cache volumes !3269
- Merge the config template before asking the user for configuration !2561 (Matthias Baur @m.baur)
- Make use of build requests/limits for build permission init container !3321

### Maintenance

- Add details to docs on CI_SERVER_TLS_CA_FILE !3332 (Ben Bodenmiller @bbodenmiller)
- Ensure shell writers terminate with newline flush !3329
- Upgrade Go to 1.17.7 !3327
- Install supported Go version for Windows prior to testing !3324
- Upgrade MinIO to v7.0.21 !3323
- Fix milestone ship date error for the idlescalefactor feature - docs !3317
- Remove vendor/ directory !3314
- Divide packages buildling jobs in the pipeline even more !3313
- Use latest docs linting image for Markdown !3312
- Docs: Update shell descriptions to use full names !3310 (Neil Roza @realtime-neil)
- Bump version of Go for project to 1.17.6 !3305
- Fix Azure caching example config !3300 (Stefan Asseg @stefanasseg)
- Encourage use of K8s secrets !3299 (Christian Mäder @nxt.cma)
- Update interactive example that was incorrectly set to non-interactive !3297 (Arran Walker @ajwalker)
- Update support for session_server using Helm chart !3296
- Cleanup cache proxy pattern !3294
- Adds details about how to limit the number of VMs when autoscaling !3289
- Update linting configuration from GitLab project !3288
- Replace Ruby:2.6 in examples and test cases with Ruby:2.7 !3287
- Update runner security docs !3279
- Update Page with more common -machine-machine-options for use with Docker and amazon ec2 instances. !3259
- Add information on how to connect to S3 from Runners on Amazon EKS with IAM Role for ServiceAccount !3251
- Add version number to windows helper image tags !3217 (Florian Greinacher @fgreinacher)
- Update docs/executors/shell.md !3208
- To disable wait_for_services_timeout use -1 not 0 !3207
- Add support for extra submodule update flags !3192 (Nejc Habjan @nejc)
- Clarify that listed limitations are specific to Windows !3155
- Ensure proper assumptions !3038 (Deniz Adrian @zined)
- Update the security caveats about the usage of privileged mode !2482
- Add Debian/bullseye to packagecloud DEB_PLATFORMS !2940 (James Addison @jayaddison-collabora)

### Documentation changes

- Add details on concurrent parameter for Docker executor - docs !3286
- Add alpine 3.15 as new runner/helper-image flavor !3281 (Fabio Huser @fh1ch)

## v14.7.0 (2022-01-19)

### New features

- Add RHEL/UBI amd64 FIPS support !3255

### Bug fixes

- Exclude stderr content from parsing UID/GID information within Docker executor !2768

### Maintenance

- Fix fips rpm package name to sign !3285
- Mark "prepare go fips" job as optional !3284
- Updating documentation linting images for project !3283
- Fix external links from project and remove old redirects !3282
- Restore Git 1.8.3.1 tests !3278
- Fix tests using GitLab-grack submodule !3272
- Clarify how to configure network mode with Docker executor !3264
- Update golangci-lint !3261
- Pass UPSTREAM_CI_COMMIT_REF to incept tests !3257
- Update Sentry library from raven-go to Sentry-go !3199 (Markus Legner @mlegner)
- Bump used Go version to 1.17 !3112
- Show error details for failed artifact uploads !3240

### GitLab Runner distribution

- Fix the 'stable GitLab release' job !3252

### Documentation changes

- Point to GitLab maintained fork of Docker Machine !3276 (Thameez Bodhanya @thameezbo)
- Release of a FIPS Compliant runner !3274
- Adds note about 5 GB S3 cache limit !3266
- Added troubleshooting steps !3273
- Fix broken external links !3270 (Niklas @Taucher2003)
- Update to mention CentOS stream 8 instead of CentOS linux 8 !3267 (Ondřej Budai @ondrejbudai)
- Document need for entrypoint to open shell !3256
- Updated language for Kubernetes executor !3253
- Update link to K8s pull policy !3254
- Improve the cache documentation for k8s executor !3237
- Update docs for GitLab Runner Helm Chart using ACS (retired) to AKS !3219
- Remove trailing spaces for Jan 2022 TW monthly chores !3275

## v14.6.0 (2021-12-17)

### Bug fixes

- Implement improved JSON termination mode for Kubernetes !3225

### Maintenance

- Add Vale rule updates from the GitLab project to this project !3249
- Minor capitalization and style fix !3248
- Trigger UBI images build also from security fork !3245
- Add note about running Docker runner with Docker-machine functionality !3236 (Ihor Martyniuk @enoot)
- Remove coverage reports from S3 release !3235
- Add curl in alpine image !3233
- Fix flaky garbage collection test !3230
- Move the "static QA" job to the postrelease stage !3227
- Automatically retry integration_k8s jobs !3226
- Docs: Clarifying that it's "a" macOS machine, rather than "yours" !3223
- Remove unneeded quotes from markdownlint config !3215
- Run incept tests in the postrelease stage so that all binaries and images are available !3214
- Update markdownlint and Vale rules from GitLab project !3213
- Add additional docs and integration tests for cache.s3.AuthenticationType !3210
- Docs: Changed "clean up" from noun to verb !3206
- Docs: Clarify what Runner Cloud is !3205
- Drop gorilla/mux in favour of http.ServeMux !3203
- Add idle GitLab_runner_jobs metric per runner !3202
- Fix links to shared runners documentation !3201
- Add openssl command to download the cert !3200
- Improve Runner container image size for Ubuntu and alpine !3185 (Furkan Türkal @Dentrax)
- Autoscale VMs based on a percentage of in-use VMs !3179
- Use native go errors and drop pkg/errors !3104 (feistel   @feistel)
- Fix the 'stable GitLab release' job !3252

### GitLab Runner distribution

- Push stable images built on security fork to canonical repository !3242
- Update the GitLab Release job !3228

### Documentation changes

- Update lint-html image for docs !3239
- Docs: Added OpenSSL SSL_connect: SSL_ERROR_SYSCALL troubleshooting topic !3229
- Docs: Add pod cleanup info in the Kubernetes doc !3224
- Update docs for installing runner from binary !3222 (Wojciech Pater @wpater)
- Changed symbol in docs table !3220
- Add Native Windows OpenSSH Server and PowerShell support for Virtualbox and Parallels executors !3176 (Guillaume Chauvel @guillaume.chauvel)

## v14.5.2 (2021-12-10)

### Security fixes

- Fix `syscall.forkExec` calling `close(fd=0)` on pipe error [!44](https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/44)

## v14.5.1 (2021-12-01)

### Security fixes

- Limit Docker executor's container reads to prevent memory exhaustion [!37](https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/37)

## v14.5.0 (2021-11-21)

### New features

- Scrub the X-Amz-Security-Token parameter from query strings !3171 (Estelle Poulin @estelle.a.poulin)
- Kubernetes executor container security context !3116

### Bug fixes

- Fix lockfile cleanup for submodules !2858 (Nejc Habjan @nejch1)

### Maintenance

- Docs: Added SSH executor disable_strict_host_key_checking details !3195
- Fix releasing alpine 3.12 helper images !3193
- Renamed enterprise_guide to fleet_scaling !3191
- Add all available unix OS build tags to unix targeted go files !3189 (Arran Walker @ajwalker)
- Fix GitLab grack to use our own repositories !3187
- Use newer docs linting image !3186
- Update changelog generator configuration !3183
- Fix Docker pulling image integration test !3182
- Break out shell blocks to allow copy from button !3181
- Add troubleshooting info to Runner installation with Agent !3180
- Log errors when failing to close Docker client !3178
- GitLab-runner Dockerfile: clear /tmp of Ubuntu Docker image !3177 (Yalcin Ozhabes @trim_the_main)
- Fix PVC volume config generation in Kubernetes executor !3174 (Sandra Tatarevićová @sandra17)
- Add troubleshooting note for dind connection error on k8s executor !3173
- Docs: Clarified concurrency setting !3172
- Fixed broken external links !3168
- Fix: typo in docs/register/index.md !3166 (David Duncan @duncan.davidii)
- Docs: Clarify runner token !3165 (Stefan Schmalzhaf @the_s)
- docs: add useful notes on setting session_server !3164 (Yang Liu @robturtle)
- Updated broken external links !3163
- Refactor images building and publishing jobs !3162
- Add changeable config directory for root !3161 (Boris Korzun @boris.korzun)
- Docs: Correct link to Windows shared runner info !3160
- Use sync.Mutex rather than RWMutex for simple protections !3159
- Remove need for Git in runner images !3152 (Ben Bodenmiller @bbodenmiller)
- Suppress Git hints about branch naming standards !3148
- Update golang-cli-helpers library, support env namespaces !3147
- Handle situations when neither `ntpdate` nor `sntp` is available !3143 (Alexander Kutelev @kutelev)
- Docs: Small edits to enhance readability !3137 (Ankita Singh @ankita.singh.200020)
- Better support for PowerShell on Kubernetes !3119
- Do not pass in bash detection script into build container !3095
- Kubernetes executor should only used SharedBuildsDir behaviour when it is required !3078 (David Alger @davidalger)
- [DOCS] - Improve image pull secrets documentation clarity !3047 (Aaron Johnson @acjohnson1985)
- Document how to run jobs as non-root user for Kubernetes and Kubernetes with Helm !2900
- Allow finer-grained control over pod grace periods. !2130 (Dominic Bevacqua @dbevacqua)

### GitLab Runner distribution

- Provide Docker images for alpine 3.12 (default), 3.13 and 3.14. !3122

## v14.4.0 (2021-10-25)

### Security fixes

- Sanitize Git folder after each build !3134

### Bug fixes

- Add Kubernetes pod label sanitization !3054 (Theodor van Nahl @t.vannahl)

### Maintenance

- Revert "Merge branch 'alpine-3-13' into 'main'" !3157
- Consider all Docker pull image system error as runner script failure !3142
- Docker Executor: use Stop for graceful shutdown !3128 (Aaron Friel @frieltwochairs)
- Update to MinIO-go v7.0.13 !3120 (Philip Schwartz @pschwar1)
- Explicit configuration for cache s3 authentication type !3117
- refactor: remove osext dependency !3101 (feistel @feistel)
- Respect Docker Runtime setting for services !3063 (Jakob-Niklas See @networkException)

### GitLab Runner distribution

- Split packagecloud release by distribution flavor !3146

### Documentation changes

- Mark URLs compatible with markdownlint-cli 0.29.0 !3154
- Remove Fedora 34 from list of packages - docs !3151
- Fixed docs crosslink from Advanced Config !3149 (Raimund Hook @stingrayza)
- Update Autoscale config due to Docker machine deprecation docs !3144
- Compatibility chart update !3141 (Alexander Kutelev @kutelev)
- Update Docker_machine.md - docs !3140
- Change description for GitLab_runner_limit !3139
- docs: Delete link to GitLab.com-config (target does not exist) !3138 (Stefan Schmalzhaf @the_s)
- Fix YAML indention of GCS secretName !3136 (Kate @kate_stack11)
- Replace incorrect terminology (storage -> bandwidth) !3135 (Jay Williams @codingJWilliams)
- Docs: Updated Microsoft Service policy links !3133
- Runner: fix some broken external links !3127
- Additional step when install GitLab Runner using KAS !3126
- Added info about code handling for Windows runner !1975

## v14.3.0 (2021-09-21)

### New features

- Cleanup build directory with feature flag !3065
- Consider only Docker image pull system error as runner-system-failure !3060

### Security fixes

- Restrict accepted metric listener HTTP methods !3109

### Bug fixes

- Fix Docker-machine executor check to reduce warning log spam for no runners able to process a job !3106 (Thomas Scully @tscully49)

### Maintenance

- Turn on FF_SCRIPT_SECTIONS for GitLab Runner pipelines !3124
- Expose runner limit error message on registration !3108
- Split linux packages into multiple jobs !3105
- Upgrade MinIO to v7 !3099
- Update runner docs tests !3096
- Remove Docker-machine feature flag !3093
- Improve log line decoding for Kubernetes !3091
- Add strict host key checking to SSH config !3074
- Upgrade alpine version to 3.13.6 !3057
- Improved bash shell escaping (behavior, performance) !2882

### Documentation changes

- Added mac setup guide !3129
- Fix trailing spaces in Runner docs !3125
- Per-build networking is recommended !3118
- Fixed typo in Dockerfile example for installing Runner with Docker !3113 (Markus Möslinger @metabytewien)
- Update documentation template !3107
- Use latest docs linting images !3100
- Update feature-flags.md, fixed typo, runners.feature_flag -> runners.feature_flags !3098 (Joost van der Sluis @jvdsluis)
- Reword so that Docker services list "images" instead of "applications" !3094
- Adds Linux register command for behind a proxy !3087 (Rui Duarte @P0w3rb0y)
- Add info for Docker_HOST value in Using Docker:dind !3085
- Added Docker image build steps for Alpine !3076
- Add doc in FAQ about running Elasticsearch !3110
- Fix typo in security process !3092

## v14.2.0 (2021-08-22)

### Bug fixes

- Do not execute checks for windows integration test in docs only default branch pipeline !3070
- Limit job log to ensure it contains UTF-8 valid data !3037
- Fix Ubuntu helper image builds to use correct platform (not always amd64) !3032 (Sneha Kanekar @skanekar1)
- Fix trace short writes when large masks are configured !2979
- Fix cleaning of removed sub-submodules when using fetch strategy !2883 (DmtiryK @dkozlov)

### Maintenance

- Update trace force send interval to be dynamically adjusted based on update interval !3064
- Update rules for windows tests to fix docs pipeline !3062
- wrap each line in a script block as a section !3051
- Add new histogram metrics to Docker+machine executor !3050
- Do not ignore failure in Windows jobs due to timeout !3042
- Fix release job to use JOB-TOKEN !3041
- Support of Kubernetes lifecycle hooks !3036
- Add all of gl-docsteam to docs CODEOWNERS !3026
- Add Evan and Marcel to docs CODEOWNERS !3025
- Use CI_JOB_TOKEN to create releases !3023
- Explicitly set Kubernetes pull image failure as script failure !3015
- Implement changes rules for executing full and docs-only pipelines !2978
- Move build log's ANSI Reset to before newline to fix test output  !2977
- Update configuration of changelog generator !2968
- Update remaining only except to rules in pipeline !2938
- Add support for determining helper image from node selector information !2840
- Upgrade specified Git version to 2.30.2 !2825
- Add allowed images restriction to Kubernetes executor !2669 (Yi Wei Pang @pangyiwei)
- Allow CI image option to override base image name (VirtualBox & Parallels) !1257 (Alexander Kutelev @kutelev)

### Documentation changes

- Modified the runner troubleshooting page for confirming the GitLab version and runner version !3081
- Update docs with the correct link about runner scope !3077
- Clarify the need for max overwrite definitions when overwriting via CI/CD script !3075
- Add troubleshooting entries for k8s-caused faults !3073
- Docs: Recommend to use latest self-managed runners with .com !3072
- Docs: Addded FREE tier badge !3069
- Docs: Addded FREE tier badge !3068
- Docs: Addded FREE tier badge !3067
- Docs: Added code block end tag that was missing !3066
- Docs: Fixed typo, changed "process" to "signal" !3061 (Igor @igordata)
- Docs: Add how to log in as current user in the Terminal so GitLab-runner installs properly !3055
- Improve wording of docs/security/index.md !3031 (Ed Sabol @esabol)
- Docs update advanced configuration !3028
- Update Vale rules with latest settings from GitLab project !3024
- Fix outdated link to custom build directories in runner advanced configuration docs !3022 (zertrin @zertrin)
- Docs: Add version for Kubernetes custom builds directory mount option !3016 (Ben Bodenmiller @bbodenmiller)
- Capitalize CPU on line 187 !2893
- Create Enterprise guide for deploying and scaling a GitLab Runner Fleet !2755

### Other changes

- Improve testKubernetesGarbageCollection integration test !3080
- Update the Kubernetes executor's attach strategy to work with Windows pods !3059
- Fix missing end quote in packagecloud script !3049
- Fix incorrect Kubernetes Windows paths for artifacts and caches !3046
- Set DOCS_REVIEW_APPS_DOMAIN in the CI config directly !3044
- Updated CODEOWNERS for docs team members who are maintainers !3035
- Update build versions for Fedora !3034
- Enable container scanning for GitLab Runner !3027
- Garbage collection supports for Kubernetes executor !2983
- Fix flakiness of the TestAttachReconnectReadLogs test !2954

## v14.1.0 (2021-07-20)

### Bug fixes

- Fix trace short writes for large log lines !2993
- Confirm if Docker is installed in `clear-docker-cache` !2961

### Maintenance

- Add CODEOWNERS for docs !3017 (Ben Bodenmiller @bbodenmiller)
- Add TestBuildOnCustomDirectory/pwsh as test failure on windows 20h2 and 2004 and TestMachineIdleLimits on 1809 !3011
- Allow KUBECONFIG and GitLab_CI env in integration tests !3010
- Fix vendor out of sync !3008
- Use image's PowerShell Core for Windows tests !3005
- Remove explicit use of GOROOT/GOPATH now that we're using Go modules !3002
- Remove unneeded test configuration !3001
- Fail k8s integration tests when the check command fails !2999
- Fix on-demand releasing of helper images !2998
- Stop considering Docker pull image as runner system failure !2995
- Skip Docker-machine provision on failure by default !2986
- Fix make prepare_index read GPG_KEY from file !2985
- Fail CI build if test failures not updated !2976
- Only print necessary env vars in tests !2971
- Update environment name for Linux Docker images !2970
- Don't run fuzz variable mask test for docs !2965
- Add environment for GitLab stable release !2962
- Add environment name for package jobs !2959
- Use file based variables for GPG_KEY !2958
- Update default branch from master to main !2930
- Only allow failures with exit code 99 in Linux tests !2704
- Test passing a config template to the RegisterCommand !2451
- Make the variable type for the GitLab CI secret configurable !2414

### GitLab Runner distribution

- Add support for Windows Server core, version 20H2 [Semi-Annual Channel release] !2908

### Documentation changes

- Restructure markdownlint configuration !3012
- Update sudo command for linux repository install !3009
- Fix broken links in Runner docs !3007
- Add note on IdleCount to autoscaling docs !3004
- Update feature flag FF_SKIP_Docker_MACHINE_PROVISION_ON_CREATION_FAILURE grammar !3000
- Docs: Complete sentence, link to general SSL troubleshooting info !2994
- Update runner readmes to index !2990
- Added note for Overwriting Kubernetes Namespace section !2984
- Mention liveness project when adding Windows runners !2981
- Add details on how to assign Runner Manager to security fork project !2974
- Docs: Updated Shell topic titles to be more clear !2972
- Update Kubernetes execution strategy documentation !2966
- Fix outdated VS Code package recommendation !2964
- Add docs about DEB/RPM packages signature verification !2963
- Docs: Specify exact Parallels product names !2960
- Provide JSON job response file for custom executor. !2912 (Paul Bryant @paulbry)
- Add instructions for proxying the GitLab registry !2865
- Fix typo/incorrect grammar !2842 (Per Lundberg @perlun)

## v14.0.0 (2021-06-19)

### New features

- Send GPU config string !2848
- Add support for selective Git submodule paths inclusion/exclusion !2249

### Bug fixes

- Fix race blocking goroutine in shell executor !2910
- Order masked values by length to prevent longer values being partially revealed !2892
- Kubernetes attach strategy hangs when log file is deleted !2824

### Maintenance

- Enable Kubernetes attach strategy by default !2955
- Add ASDF .tool-versions file !2948
- Make check test directives depend on prepare_done !2947
- Fix broken test output produced by MakeFatalToPanic !2929
- Use main branch for docs reviews !2925
- Disable windows anti-malware monitoring !2920
- Remove FF_RESET_HELPER_IMAGE_ENTRYPOINT feature flag !2906
- Remove legacy process termination for shell executor !2905
- Pull helper image from GitLab registry by default !2904
- Pwsh shell support for Kubernetes when legacy execution strategy ff is set to false !2902
- Remove offpeak settings Docker autoscaling !2897
- Add shell benchmarks !2894
- Make pwsh the default shell for new registrations !2889
- Remove FF_USE_GO_CLOUD_WITH_CACHE_ARCHIVER feature flag !2887
- Remove deprecated Makefile targets !2885
- Update Kubernetes client-go library to 0.21.1 !2878
- Segregate `unit test` job into a separate `integration test` job !2783
- Add supported failure reasons for build errors !2744
- Upgrade kardianos service !2729
- Enable fastzip & progress meter !2565
- Allow building behind a proxy !2168 (dHannasch1 @dHannasch1)

### GitLab Runner distribution

- Remove support for Windows 1909 !2924
- Remove support for Windows 1903 !2915
- Remove Ubuntu/eoan package !2888
- Publish Windows helper image :latest tags !2879
- Add Ubuntu-based runner-helper image !2835

### Documentation changes

- Add troubleshooting note on GitLab-runner symlink removal !2953
- Disable skel directory usage by default for DEB/RPM installation !2942
- Update PROCESS.md referencing runner release helper templates !2939
- Add tlsctl to runner docs !2937
- Remove old redirects !2933
- Update troubleshooting documentation for old Docker versions on Windows Server !2927
- Add remove_date to YAML frontmatter !2922
- Revert (arch) change !2918
- Shell executor: Link to latest available Git version !2917
- Be more specific about Windows version support !2916
- Made images smaller !2909
- Add troubleshooting steps to GitLab Runner operator !2901
- Fix external links !2895
- Fixed typo in Docker runner documentation !2891
- Use DNS option in TOML to avoid proxy and route in docs !2815
- Change order of steps to configure autoscaling !2665
- Update docs/executors/Kubernetes.md !1903

### Other changes

- Remove conversion between failed and cancelled buildStage for Prometheus metrics !2932
- Delete unused 1909 allowed test failures !2928
- Updated test case names for TestBuildJobStatusEnvVars !2907 (listout @listout)
- Specify the working version for lsif-go image !2898
- Remove /usr/lib/GitLab-runner symlink from packages !2890
- Make Git-lfs tar checksum usage coherent !2268

## v13.12.0 (2021-05-20)

### New features

- Support Git strategy with Kubernetes executor !2862

### Bug fixes

- Add utf-8 invalid replacement encoder to trace transformers !2881
- Pass PowerShell scripts as a file to shell executor !2874
- Add new eval execution strategy for capturing exit code !2818

### Maintenance

- Revert "Publish Windows helper image :latest tags" !2880
- Use latest docs linting images for testing !2877
- Ensure Docker client is in experimental mode !2870
- Improve trace masking performance !2863
- Use PowerShell for resolving paths !2836
- Move commands package integration tests to own files !2795
- Allow whole Vault Secret configuration to be variable-expandable !2772
- Update coverage and windows tests to rules !2756

### Documentation changes

- Changed ${arch} to $(arch) !2875
- Fix TOML syntax in Kubernetes documentation !2872
- Convert pull policies bolds into headers !2867
- Update GitLab Runner connectivity !2866
- Update Kubernetes pull policy documentation !2860
- Document operator properties, custom installation and permissions !2847
- Clarify, N-to-M relationship of Runners to GitLab instances !2788

## v13.11.0 (2021-04-20)

### New features

- Allow user to specify multiple pull policies for Kubernetes executor !2807

### Bug fixes

- Use inspect.GID() to collect GID value in Docker executor !2769
- Fix Kubernetes attach strategy for non-root environments !2749

### Maintenance

- ci: wrap GOCACHE with double quotes !2859
- Add bridge job to runner-incept !2845
- Fix archives/zip tests on Windows !2832
- Report CI test build failures !2829
- Add job URL to container labels !2823
- Compile GitLab-runner binary for shell integration tests !2820
- Don't return error when checking feature flag !2812
- Simplify the triggerring of 'GitLab-docs' Review App pipelines !2809
- Fix Git 1.8.3.1 job errors !2791
- Fix job duration value when in log format is JSON !2787
- Add support for CSI volumes !2784 (Brandon Butler @brandonbutler)
- Move Kubernetes integration tests to separate file !2779
- Support for env variables expansion for image name for Kubernetes executor !2778
- Segregate integration tests in Docker executor !2776
- Remove test with int overflow !2597 (Fábio Matavelli @fabiomatavelli)
- Adding Git depth for submodules !2107 (Nico Bollen @bollenn)

### Documentation changes

- Removed reference to master !2855
- Fix pipeline configuration for docs branches !2853
- Make clear when FF runner configuration syntax was introduced !2852
- Roughly alphabetised Docker Container parms - docs !2851 (Raimund Hook @stingrayza)
- Updated docs reference to MinIO !2850 (Raimund Hook @stingrayza)
- Documentation Update/docs/security/index.md !2849 (Anshuman Singh @singhanshuman)
- Add clarification on Runner and GitLab Version match !2841
- Edited for style !2838
- More edits for style !2834
- Add services note to Windows container troubleshooting !2833
- Edited for grammar and style !2830
- Moved troubleshooting to OS topics !2819
- Fix heading type in GPU documentation !2817
- pWIP: Add configuration section to docs for runner Operator on OpenShift !2816
- Add feature flags in config.TOML !2811
- Update links to redirected files !2808
- Add a note to docs on usage of Docker script from `13.9` !2806
- Remove Docker-machine provision on creation failure !2805
- Improve documentation for GPUs for all executors !2804
- Update redirected links in runner docs !2802
- Add troubleshooting section in Kubernetes executor documentation !2799 (Vincent Firmin @winkies)
- Edited for style and consistency !2777
- Document how to customize environment variables for Runner !2775

### Other changes

- Update warning message URL for DockerHub !2844

## v13.10.0 (2021-03-21)

### Bug fixes

- Don't print DockerHub helper image warning when custom image is defined !2761
- Allow graceful termination on Windows !2739

### Maintenance

- Include symbols in GitLab-runner binary !2800
- Move process package integration tests to own files !2794
- Update `code navigation` job definition !2792
- Rename shell_writer_test.go to reflect use !2782
- Move virtualbox executor integration tests to own file !2781
- Move parallels executor integration tests to own file !2780
- Update trace limit wording !2765
- Update of Docker error message !2759
- Add integration tests for trace limit handling !2758
- Add integration tests for build trace masking !2754
- Version pin pwsh version inside of our CI tests !2748
- Update hashicorp/go-version dependency to v1.2.1 !2746
- Removal of unused replace from go.mod !2745
- Start updating runner pipeline to rules !2728

### Documentation changes

- Add mentions to pwsh to documentation !2797
- Update Vale rules !2789
- Add mention to pwsh support in Docker executor docs !2786
- Fix example gcp zone for Docker+machine config !2771
- Runner: Update spelling exceptions list !2770
- Docs for installing runner on a separate machine !2767
- Update docs/monitoring/README.md !2766
- Fix misspelling of "Force" in PowerShell examples !2764 (Gabriel Smith @yodal\_)
- Add runner execution flow diagram !2760
- Fix duplicate labels in CONTRIBUTING.md !2747 (h.yoshida @hirokiyoshida837)
- Add backticks around --Google-accelerator docs !2742
- Update documented check internal for config changes !2741
- Add documentation for using GPUs with Docker Machine !2736
- Update MachineOptions to only mandatory configuration. !2673

## v13.9.0 (2021-02-22)

### New features

- Enable PowerShell Core support in Kubernetes Executor !2705
- Enable PowerShell Core support in Docker Executor on Linux !2563
- Add support for setting the artifact/cache compression level !2684
- Display feature flags that are set to a non-default status !2606
- Add GPU support for Docker executor !1955 (Andreas Gravgaard Andersen @agravgaard)

### Security fixes

- Remove skipVerify from client struct !2654

### Bug fixes

- Fix panic when PKCS7-encoded payload has no certificate !2737
- Correctly set fastzip's staging directory !2693
- Improve trace secret masking with x/text/transform !2677
- Add explicit bash shell error checks !2671
- Terminate requests on process shutdown !1684

### Maintenance

- Change env to bash to resolve Illegal option !2732
- Upgrade Docker version to 20.10.2 !2722
- Update Docker script default to Docker prune volumes !2720
- Default to no Docker image compression in local environment !2717
- pwsh scripts can be passed over STDIN on shell executor !2715
- Update GitHub.com/Docker/cli dependency !2714
- Add artifact and cache download progress meter !2708
- Remove requirement for Docker daemon experimental mode from image build scripts !2707
- Fix the image that is used to create the pwsh tag !2706
- Exclude out/binaries/GitLab-runner-helper from binaries artifacts !2703
- Improve logging to packagecloud push !2702
- Upgrade PowerShell Core to 7.1.1 !2696
- Make TestHelperImageRegistry not need real prebuilt images !2682
- Add test for permissions container in k8s !2676
- Add object storage cache credentials adapter !2674
- Add artifact/cache upload progress meter !2670
- Refactor Docker pull logic into dedicated package !2659
- Update to Docker client v20.10.2 !2658
- Update GitLab-terminal package !2656
- Create separate helper image with PowerShell Core !2641
- Prioritize helper image specified in config to change K8s log dir permissions !2578 (naruhito @naruhito1)
- Remove helpers/trace redundant io.Pipe use !2464
- Kubernetes tests simplify build creation !2445
- Report deleted pods as a system failure with attach strategy !2444
- Fix incorrect path/filepath use !2313
- Improve Docker cleanup script to also include old builds and images !2310
- Output coverage to Cobertura report !2252
- Version pin pwsh version inside of our CI tests !2748
- Add integration tests for trace limit handling !2758
- Add integration tests for build trace masking !2754

### Documentation changes

- Document how to view Windows service logs with cli !2733
- Update linux-manually.md !2731 (Simon Carr @simonjcarr)
- Added details about guided install !2730
- Use correct Vale extension in VS Code ext file !2727
- Refresh Vale linting rules !2726
- Specify tag syntax for tagged releases !2725
- Add note about permissions !2723
- do not link to unmaintained Docker image cleanup app !2712 (Antoine Beaupré @anarcat)
- Fix formatting of FF_USE_LEGACY_Kubernetes_EXECUTION_STRATEGY !2701 (Ben Bodenmiller @bbodenmiller)
- Clarify download instructions !2700
- Replace x86 and amd64 with 32-bit and 64-bit terminology for download !2699
- Add buildImage in the default OpenShift operator example !2698
- 1/3 Add crosslink to Kubernetes Agent docs !2697
- docs: Clarify self-signed certs on windows !2695 (Stefan Schmalzhaf @the_s)
- Docs: Fix minor whitespace inconsistency !2694 (Stefan Schmalzhaf @the_s)
- 27451 Fix Documentation - podAnnotation should be a TOML table !2692 (Benjamin Souty @B-Souty)
- Split docs linting jobs !2689
- Docs: Links documentation to working example for CMD Shell usage on Windows GitLab Runners where only PowerShell can be the default !2687
- Documentation - Supported OS updates !2683
- Whole hour periods for autoscale !2681
- Mention version sync on first sign of trouble !2680
- Fix typo in Kubernetes.md !2675
- Removed extra spaces !2672
- Update install runner on Kubernetes install page - docs !2668
- Simplification of dind service section !2663 (Keith Kirkwood @keithkirkwood)
- Instructions for installing dependencies on CentOS. !2619 (David Hannasch @dHannasch)
- Include in docs details about the updated script !2586
- Changed recommendation to instance type in docs to a smaller one !2579 (Jan Pobořil @janpoboril)
- Document known race condition about Helm upgrade !2541
- Improve TLS custom cert documentation !2487

### Other changes

- Add CODEOWNERS for 3 files at repo root !2667
- Revert "Improve trace secret masking with x/text/transform" !2752

## v13.8.0 (2021-01-20)

### New features

- Allow user to specify multiple pull policies for Docker executor !2623

### Bug fixes

- Fix fastzip to support artifacts for nonroot users !2661
- Fix s3 cache upload for aws EKS IRSA !2644 (Clemens Buchacher @cbuchacher)
- Fix cache push for failed jobs for Docker and Kubernetes executor !2638 (Axel Amigo @hax0l)
- Fix Azure cache not working in K8S executor !2626
- Fix path checking in Build.getCustomBuildDir !2251

### Maintenance

- Add Docker integration tests for cache push for failed job !2657
- Report that the Runner returns exit codes !2645
- Update GoCloud to v0.21.1+ !2637
- Add tests to PowerShell shell !2634
- Lock mutex in Buffer.SetLimit !2627
- Fix/k8s skip hostaliases for empty services !2582 (Horatiu Eugen Vlad @hvlad)
- Fix windowsPath to handle local named pipes correctly !2470
- Override Git HTTP user agent !2392
- Allow using prebuilt Docker helper images when running from out/binaries !2104

### Documentation changes

- Finish runner standardization update !2666
- Update linux-repository.md changes date of end of life date of CentOS 8 !2662 (Mohammad.E @emamirazavi)
- Removed ntrights reference !2660
- Restructure "Supported options for self-signed certificates" doc section !2651
- Edited runner to be lowercase !2650
- Edited runner to be lowercase !2649
- Edited runner to be lowercase !2648
- Edited runner to be lowercase !2647
- Edited runner capitalization !2646
- Fix documentation issue in Kubernetes node_selector !2643
- Update docs for the new GitLab Runner operator !2640
- Synchronize Vale rules and fix !2633
- Improve documentation for configuring the cache with a K8S runner !2632
- Add Azure to possible runners cache type !2631
- Add reference to GitLab Runner Operator and 13.7 MVC issue !2630
- Add `make runner-and-helper-docker-host` to `make help` !2629
- Add troubleshooting guide for GitLab Runner !2628
- Runner: add Vale test for possessive form of GitLab !2624
- Docs: Removed possessive GitLab's !2620
- Runner: fix unquoted curl command URL strings !2618
- Runner: move CurlStringsQuoted.yml rule to error !2617
- Add Windows Server Core command for logs !2602
- Fixed typo: libivrt -> libvirt !2519 (Aaron @aaronk6)
- Autodetect VirtualBox path on Windows !2020 (Pedro Pombeiro @pedropombeiro)
- Update Kubernetes.md to reflect <https://gitlab.com/charts/gitlab-runner/merge_requests/34> !1470

### Other changes

- Add missing entry to 13.6 changelog !2642

## v13.7.0 (2020-12-21)

### Security fixes

- Updating min TLS version to 1.2 !2576
- Replace umask usage with files permission change when a non-root image used !2539

### Bug fixes

- Upgrade fastzip to v0.1.4 !2605
- Remove .Git/config.lock in build directory !2580
- Fix attempting Kubernetes Docker registry secret cleanup on failed creation !2429

### Maintenance

- Gracefully fail unexpected Stream() calls !2609
- Update lowest Git version support inside of CI !2600
- windows: Don't log crypto/x509: system root pool warning !2595
- Add .editorconfig !2588
- Use helper image to change K8s log dir permissions !2573
- Fix check_modules command !2572
- Replace assert.True and errors.Is with assert.ErrorAs/ErrorIs !2571
- Exclude secure jobs from docs pipelines !2564
- Submit exit code back to Rails when a job fails !2562
- Fix dead URL in Docker.go !2557 (Victor Mireyev @AmbientLighter)
- Pin StefanScherer/windows_2019_Docker box to 2020.04.15 !2555
- Pull helper image from GitLab.com registry !2554
- Update testify package to version supporting errors.Is/As directly !2537
- Introduce Docker internal user package !2534
- Introduce Docker internal exec package !2533
- Send build trace bytesize in the final build update !2521
- Support Pod DNS policy for Kubernetes executor !2477
- Support Pod DNS Config and Policy for Kubernetes executor !2473
- Add support for Windows Server Core 2004 !2459 (Raphael Gozzo @raphaelgz)
- Ensure that runner is unregistered on registration failure !2447
- Make runner-and-helper-Docker-host use host arch and os. !2432 (Horatiu Eugen Vlad @hvlad)
- Improve cache upload speed !2358 (Erik Lindahl @erik.lindahl)
- Disable syslogging by default for systemd systems !2333 (Matthias Baur @m.baur)

### GitLab Runner distribution

- Publish Docker images to ECR public !2608
- Add job to create ecr token for pipeline !2607
- Install aws cli in CI image !2599

### Documentation changes

- Removed spaces from diagram !2616
- Remove alert box vale rules !2613
- Add interaction diagram to Kubernetes executor docs !2612
- Changed format of alert boxes !2610
- Fix unescaped characters in a table !2604
- Correct grammar/spelling errors in advanced configuration !2603
- Removed one-sentence topics !2601
- Fixed error in `config example` !2598
- Fix indentation of runners.cache in Kubernetes.md !2592 (Yorgos Oikonomou @yorgos..oik)
- Fixed Vale future tense issues !2585
- Fixed Vale future tense errors !2584
- Moved Kubernetes keywords into sub-tables !2583
- Commented out modal install window details (2 of 2) !2577
- Fix trailing space issues in docs !2569
- Fix broken links in the GitLab-runner docs !2568
- Fix typo in monitoring documentation !2556 (Horst Gutmann @h.gutmann)
- Add documentation on how to add a new Windows version !2498
- Updated compatibility table !2489
- Update index page for style !2484
- Allow specifying `basefolder` when creating virtualbox VM !2461 (Jack Dunn @JackDunnNZ)
- Runner guided install (2 of 2) !2460
- Allow to set extra hosts on Kubernetes executor !2446 (Horatiu Eugen Vlad @hvlad)
- Updates documentation to highlight that SELinux can cause errors in the "Prepare Environment" state !2309 (Sean McNamara @seanmcn)
- Update AWS autoscale docs for clarity !1820
- Update generated PowerShell script example mkdir !1565
- Add advice on network segmentation !1404

### Other changes

- Update GitLab Changelog configuration !2615
- Remove product from product-categories URL from template !2611

## v13.6.0 (2020-11-21)

### New features

- Add labels to cache-init Docker container !2412
- Expose custom executor services with $CI_JOB_SERVICES !1827 (Jovan Marić @jovanmaric)
- Enable PowerShell Core support in Docker-Windows executor !2492

### Maintenance

- Expose ci job services as custom !2550
- Publish helper images to registry.GitLab.com !2540
- Allow user to define command and entrypoint to services from config !2525
- Consolidate helper exe location for Dockerfile build !2501
- Fix Azure cache uploads using Go Cloud !2500
- Fix definition of security related jobs !2499
- Move doc/dependency_decisions.yml file to a better place !2485
- Fix TestBuildCancel from timing out !2468
- Teach artifact/cache commands about the archive interface !2467
- Improve build logging testing !2465
- Skip CleanupFileVariables stage if no file variables !2456
- Change in interactive --URL question to match docs !2431
- Added SubPath support to Kubernetes volume definitions !2424 (Matt Mikitka @mmikitka)
- Add fastzip archiver/extractor !2210
- Implement archiver/extractor interface !2195
- Manage driver defined job variables in custom executor !2032 (Paul Bryant @paulbry)
- Update doc about release windows image script !1561

### Documentation changes

- More Vale rules updates !2552
- Clarify windows install instructions !2549
- synchronize Vale rules and fix !2547
- Add reference to config.TOML for setting Docker image pull policy - docs !2545
- Remove extra parentheses !2542 (Ben Bodenmiller @bbodenmiller)
- Docs: Rename and redirect docs/install/registry_and_cache_servers.md !2535
- Add stage / group metadata to docs pages !2528
- Add mention that registry mirror is started as HTTP not HTTPS !2527
- Elaborate on Docker mirror, and link to Docker doc !2526
- Docs: Redirected custom executor index page !2522
- Docs: Changed bullets to a table !2517
- Added docs for using a configuration template in the Helm chart !2503
- Update vale rules !2502
- Use latest docs linting image !2497
- Docs: Updated top-level page !2496
- Update link to runner helper image in documentation !2494 (botayhard @botayhard)
- Change mention of custom cache containers to volumes !2491
- Add missing supported architectures for Runner helper !2490
- Update [runners.machine] section in Autoscaling GitLab Runner on AWS EC2 documentation !2480
- Provide a full list of metrics available for GitLab runners in the documentation !2479
- Clarify how service_account in TOML is used !2476 (Ben Bodenmiller @bbodenmiller)
- Introduce usage of Runner Manager terminology !2474
- Docs: Revamp Runner home page !2472
- Update Kubernetes' documentation to include ephemeral storage requests/limits !2457
- Add Kubernetes runners allowPrivilegeEscalation security context configuration !2430 (Horatiu Eugen Vlad @hvlad)
- Update Runner registry and cache documentation page !2386
- Cap maximum Docker Machine provisioning rate !1038 (Joel Low @lowjoel)

## v13.5.0 (2020-10-20)

### New features

- Allow runner to archive cache on failure !2416
- Add job status environment variables !2342
- Add labels to Docker cache volumes !2334
- Set k8s runner ephemeral storage requests and limits !2279

### Bug fixes

- Docker executor: return error on pull/import failures !2113
- Fix path separator for CI_PROJECT_DIR in Windows in bash shells !1977

### Maintenance

- Ensure that for abort only abort is called !2463
- Detach runtime state/metric from CI_JOB_STATUS !2462
- Update stretchr/testify library to fix flaky test !2450
- Report Kubernetes pods' conditions when they're pending !2434
- Move variable creation out of specific resolver implementation !2413
- Test more executors in TestAskRunnerOverrideDefaults !2406
- Test for detecting overriding of CI server values !2403
- Support 'canceling' remote job status. !2377
- Add basic fuzz tests as part of dogfooding coverage guided fuzzing !2347
- Standardize indentation in YAML code !2328
- Use newest helper image version in tests !2223
- Update calls for SkipIntegrationTests to not return !2065
- Setup secure jobs !1897
- Disable secret_detection job !2471

### Documentation changes

- Doc `cleanup_file_variables` for custom executor !2455
- Link Azure storage container docs !2454
- Use Google driver for examples !2442
- Fix typo in k8s read_only config flag documentation !2441
- Docs: Removed extra notes !2440
- Removed many of the notes !2439
- Harmonize docs linting rules !2435
- Docs: Fixed here links and added metadata !2425
- Minor edits of recent edits !2423
- Remove contractions linting rule !2421
- Docs: Edits for Vale rules and other style !2420
- Documentation: Add log level mention to troubleshooting !2419
- Switch autoscaling Docker Machine examples to GCP and Ubuntu !2417
- Add troubleshooting about windows mapped drives !2415
- Docs: Updating metadata !2405
- Docs: Update Docker tables to clarify what's supported !2404
- Update default install docs to disable skel !2402
- Docker version requirements in Windows Server !2401
- Document vargrant-parallels plugin and add clone instructions !2399
- Changing Kubernetes executor service-account command !2312

## v13.4.0 (2020-09-18)

### New features

- Add Hashicorp Vault secret resolver !2374
- Add Hashicorp Vault integration package !2373
- Add Hashicorp Vault golang library !2371
- Add secrets handling abstraction !2370

### Bug fixes

- Improved interrupt/cancelation build tests !2382
- Fix Windows runner helper Docker container !2379
- Fix metric reading race conditions !2360
- Record only first resolved credentials for each Docker registry !2357
- Ensure PowerShell file variables contain no BOM !2320

### Maintenance

- Use consts for job state in TestUpdateJob !2397
- Support trace rewind !2390
- Support update interval on update job !2389
- Introduce `UpdateJobResult` and `PatchState` !2388
- Fix check_mocks make target !2387
- Update docs pipeline to use new image !2384
- Add support for custom PUT HTTP headers in cache archiver !2378
- Send trace checksum on job updates !2375
- Update node affinity tests assertions !2369
- Add test for cache archiver shell execution !2367
- Update log message for starting VM in Parallels executor !2361 (Per Lundberg @perlun)
- Fix changelog generator config to catch all maintenance related labels !2359
- Update log message for starting VM in virtualbox executor !2356 (Per Lundberg @perlun)
- Remove trailing spaces check !2352
- Replace whitelist terminology with allowlist !2338
- Use configured userns mode for services !2330 (Lukáš Brzobohatý @lukas.brzobohaty)
- Add Kubernetes node affinities settings !2324 (Alexander Petermann @lexxxel)
- Re-enable windows Docker tests !2308
- Use new function to create Docker client !2299
- Add Secrets entry to job payload structures !2288
- Remove redundant Docker executor integration tests !2211
- Add missing assert for mock !2116
- Allow overwriting Service and Helper container resources !2108 (Renan Gonçalves @renan.saddam)
- Use parallel compression and decompression for Gzip archives and caches !2055 (Ben Boeckel @ben.boeckel)
- Add variable to enable fallback cache key !1534 (Erik Lindahl @erik.lindahl)
- Print Docker image digest !1380 (David Nyström @nysan)

### Documentation changes

- Update docs-lint job to use latest image. !2398
- Add note not to use AWS security group ID with Docker machine !2396
- Docs: improve documentation grammar !2395 (Jonston Chan @JonstonChan)
- Fix grammar in documentation index page !2394 (AmeliaYura @AmeliaYura)
- Add documentation on how to use Ubuntu image in Kubernetes !2393
- adding a tip on configuring timestamp in Docker runner !2391
- Docs: Fix misspelled word !2383
- Update Vale and markdownlint rules !2380
- Docs: Fix minor typo in Registering runners page !2376
- Add Azure Blob Storage support for cache !2366
- Add note to docs about using shell executor when building macOS/iOS apps !2365
- Cleaned up some of the wording for macOS install !2364
- Document node affinity !2363
- Change order of headers in exec docs !2362
- Docs: Edited Fargate doc !2355
- Fix broken link !2354
- Update Kubernetes.md documentation replace example gitlabUrl !2353 (Tyler Wellman @tylerwel)
- Fix section numbering in docs/development !2349
- CONTRIBUTING.md: fix FreeBSD label !2348 (Kenyon Ralph @kenyon)
- Use `shell` instead of `bash` for Markdown !2345
- Update Registering Runners page !2337
- Add documentation for configuring private registries with imagePullSecrets !2131 (Tom Bruyninx @TomBrx)

### Other changes

- Clarify --help text for --ID flag !2385

## v13.3.0 (2020-08-20)

### Bug fixes

- Install Runner in /usr/bin and helper in /usr/lib in Linux !2329
- Fix PowerShell #requires use !2318
- Fix untagged registration and add regression tests !2303
- Add openssh-client to Docker images !2281
- Use container ID, not name, for service's healthcheck hostname !2118

### Maintenance

- Add security harness !2315
- Move GitLab release to its own job !2314
- Fix typo for security branch !2304
- Add MR piplines for security fork on master !2301
- Add release jobs to security fork !2300
- Add security issue and merge request templates !2298
- Refresh linting rules !2297
- Make `.stage_done` available also on docs MRs !2295
- Remove needs from feature flags docs job !2293
- Fix DAG dependencies of release jobs !2289
- Run Docker import for helper-dockerarchive-host !2275
- Update changelog generator to accept new labels !2271
- Fix typo in DUMB_INIT_S390X_CHECKSUM variable name !2270
- Cache GOCACHE in CI !2187
- Enable DAG for some jobs !2076
- Upgrade Git version !2306
- Update Ubuntu Docker container to Ubuntu 20.04 !2286 (Markus Teufelberger @markusteufelberger)
- Log additional Docker-machine prep/cleanup info !2277

### Documentation changes

- Synchronize lint rules and fix where required !2341
- Fix name script !2339 (Andros Fenollosa @tanrax)
- Document how to renew GPG key !2336
- Update Documentation template to reflect standard !2332
- Fix broken external links !2331
- Document security release process !2322
- Fix incorrect Fargate cluster name !2321 (Matt Breden @mattbred56)
- Added specific token steps !2317
- Update docs.GitLab-ci.yml to use trigger-build script !2311
- Add content describing Runner behavior for changes to config.TOML - docs !2307
- Made links descriptive !2302
- Creation of OpenShift Runner doc. !2296
- Removed accidentally commited installation instructions in 13.2 !2290
- Update info about support Linux/OS/archs !2287
- Add explicit location for Windows logs !2285
- Fix link to TOML docs Array of Tables. !2280 (Bheesham Persaud @bheesham)
- Added architecture info !2278
- Fixes mixed-case anchor !2272
- Make it clear which Fargate container should have the specific name !2269
- Update a link to download the latest Fargate driver version !2259 (Ricardo Mendes @ricardomendes)
- Replace backticks with bold for UI elements !2099
- Add an ENTRYPOINT script to the helper image Dockerfiles to add CA certificates !2058

## v13.2.0 (2020-07-20)

### New features

- Publish a GitLab Runner Docker image for Linux on IBM Z !2263
- Pass `multi_build_steps` as a Runner Feature when requesting a job !2213
- Leverage Docker buildx for the helper image and build for s390x !2206
- Enable PowerShell Core support in Shell Executor !2199
- Build and release binary for s390x !2196
- Label Docker networks in the same way as containers !1930
- Tag helper image with runner version !1919 (Fábio Matavelli @fabiomatavelli)

### Bug fixes

- Fix Kubernetes runner timeout when the image name is invalid !2197 (Matthias van de Meent @matthias.vandemeent)
- Update Git TLS settings to be configured for repo URL, not GitLab URL !2111
- Fix support for UNC paths in PowerShell executor !1976 (Pedro Pombeiro @pedropombeiro)
- Set EFS flag to indicate that filenames and comments are UTF-8 encoded !1325 (Kazunori Yamamoto @kaz.yamamoto)
- Add openssh-client to Docker images !2281

### Maintenance

- Unsilence the `make lint` target !2245
- Fix warnings reported by goargs linter !2233
- Fix shellcheck linter reported issues !2232
- Add goargs to CI build !2224
- Replace gocyclo linter with gocognit !2217
- Enable Windows tests for community MRs !2215
- Report `panic` failures in CI tests !2212
- Fix integration tests on Windows that rely on Git version !2207
- Enable optional checks in gocritic linter !2162
- Enable shadowing checking in govet !2150
- Enable funlen linter !2149
- Enable goprintffuncname linter !2148
- Enable nakedret linter !2143
- Enable nestif linter !2142
- Enable line limit linter !2141
- Dockerfiles restructuring !2114
- Rename trace.Fail to trace.Complete !2102
- Remove duplication from build_test.go !1843
- Ensure CI image is built if CI_IMAGE value changes !2267
- Retry helper image build !2265
- Remove `GOLANGCI_LINT_CACHE` usage !2257
- Remove unnecessary indentation in method !2256
- Update alpine image version in `static QA` job to 3.12.0 !2255
- Write diagnostics for missing `make development_setup` call !2250
- Run PSScriptAnalyzer on PowerShell scripts !2242
- Fix helper-Docker target !2226
- Fix code navigation job to wait until the image job is done !2221
- Fix a spelling error in the bug template and tidy up some other wording !2219
- Standardize Makefile target names !2216
- Fix data race in TestDockerCommandBuildCancel !2208
- Add native code intelligence block to CI !2201
- Speed up `clone test repo` job !2192
- Fix flaky TestListenReadLines in log processor !2191
- Run Kubernetes integration tests !2155
- Enable unparam linter and fix reported errors !2135
- Enable errcheck linter !2134
- Fix Dockerfile issues reported by halolint !2106
- Fix out-of-date test expectations !2012
- Update entrypoint shebang for Docker images !1780 (J0WI @J0WI)
- Reduced layer count on Windows helper images !1777 (Alexander Kutelev @kutelev)
- Update to alpine v3.12 !1763

### Documentation changes

- Docs: Updated note to add install from UI instructions !2264
- update "screenshot" of running GitLab-runner without arguments. from version 1.0.0 to 13.0 !2262 (@mxschumacher @mxschumacher)
- Session server listen on IPv4 and IPv6 !2260
- Update documentation for helper image tags !2258
- Synchronize lint rules !2254
- Update custom executor docs with `step_*` !2253
- Docs: Fixed Git commands !2244 (Stefan Zehe @szehe)
- Docs: Updated broken links !2240
- Adjust metadata and move page !2235
- Docs: fix broken external links !2234
- Fix Debian container path and SSH port in the Autoscaling GitLab CI on AWS Fargate guide !2230
- New config for Vale and markdownlint !2214
- Note that Interactive Web terminal don't work with Helm yet !2189 (Ben Bodenmiller @bbodenmiller)
- Update doc for Autoscaling GitLab CI on AWS Fargate, adds troubleshooting section. !2188 ( Rob @rwd4)
- Update Fargate Task connection info in autoscaling aws fargate doc !2181
- Review Handbook page: /runner/configuration/tls-self-signed.html !2170
- Add docs how to use k8s secrets for registration !2154
- Update index.md to include documentation in for the `--access-level` param values !2137

## v13.1.0 (2020-06-19)

### New features

- Fix file archiver message to include directories !2159
- Use direct-download on a first attempt for artifacts !2115
- Add full support for artifacts/exclude feature !2110
- Add data format definition for build / artifacts / exclude !2105
- Add support for `direct_download` artifacts !2093
- Publish Windows 1909 helper image !2086
- Support runner predefined variables inside overwrite variables Kubernetes !2069
- Add Centos8 and Ubuntu 19.10 & 20.04 packages !2002
- Change default Git fetch flags allowing user to overwrite them !2000 (Łukasz Groszkowski @falxcerebri)
- Run any step from job response in a separate BuildSection !1963

### Bug fixes

- Fix missing logs from Docker executor !2101
- Fix automatically adding cache directory when cache disabled on register !2091 (Max Wittig @max-wittig)
- Fix millicpu comparison for maxOverwrite !2019
- Make commander start process group for each process !1743
- Extract commander from custom executor !1654
- Extract process killing from custom executor !1653

### Maintenance

- Increase allowed data races !2204
- Fix test assertions for k8s integration tests !2171
- Increase allowed data races !2164
- Fix TestDockerCommandUsingCustomClonePath for Windows !2153
- Rename network manager file for Docker executor !2147
- Enable staticcheck linter !2136
- Update GitLab CI image to include Git LFS !2124
- Implement Is for \*BuildError !2121
- Update log message for failure of removing network for build !2119 (Max Wittig @max-wittig)
- Change license management to use rules !2096
- Use Docker client's ContainerWait !2073
- Use taskkill windows !1797
- Cleanup dependencies for alpine based Docker image !1778 (J0WI @J0WI)

### Documentation changes

- Add all Vale rules from main GitLab project !2203
- Docs: Fix distribution order !2200 (Martin @C0rn3j)
- Update the register page to use the correct Docker registration commands - docs !2186
- Sync spelling exceptions list from GitLab project !2184
- Docs: fix broken links in Runner docs !2183
- Remove reference to lack of arm64 Docker images !2178
- Fix documentation TOML examples with [[runners.machine.autoscaling]] !2177
- Update GitLab Runner in a container documentation to prevent errors in using the Runner image - docs !2175
- Docs: Edited runners.cache.s3 details !2167
- Add example logs for `runner` and `json` log-format options - docs !2163
- Adds workaround for env vars in config.TOML !2156
- Update redirected links !2152
- Add Docker to capitalization rules !2146
- Include MachineName and MachineDriver in autoscaling example !2140
- Specify pull policy for Kubernetes executor !2129
- Improve Batch deprecated details !2128 (Ben Bodenmiller @bbodenmiller)
- docs: Link to example of how to color PowerShell output !2127 (Ben Bodenmiller @bbodenmiller)
- Docs: removed Ubuntu from LXD instructions !2126
- Refresh Vale rules !2125
- Adds note about the image for AWS Fargate !2100
- Add GDK to capitalization rules !2097
- Docs: edited autoscaling period content !2094
- Drop mention of 'OffPeakPeriods' from 'docs/faq/README.md' !2092
- Skip build stages that have no operations !2081
- Add vale plugin to recommended VS Code extensions !2078
- AWS Fargate guide walkthrough !2075
- Mark Prepare environment stage as system failure !1915
- Expose Code coverage report artifact !1863
- Send `SIGTERM` then `SIGKILL` to process in Shell executor !1770
- Publish Windows 1903 helper image !1634

### Other changes

- Fix data race in TestNewReadLogsCommandFileLogStreamProviderCorrect !2193
- Fix building of Windows helper image !2180
- Rename ill-named script variable in release_Docker_images !2173
- Change alpine mirrors to default mirrors for arm/arm64 !2165
- Skip flaky log processor test TestResumesFromCorrectSinceTimeAfterSuccessThenFailure !2151
- Enable gocritic linter !2145
- Return error from k8s `limits` function when parsing resource limits !2144
- Upgrade golangci-lint to v1.27.0 !2139
- Pass an explicit context path to Docker build in `build_ci_image` !2133
- Fix error when using attach strategy and ErrSkipBuildStage is returned when generating script !2123
- Revert removal of Windows Batch support !2112
- Do not log warning if trace update interval header value is empty !2103
- Add retries for runner system failures in CI !2098
- Remove `--kubernetes-services` command line flag !2074
- More verbose logging for artifact uploading !2052 (Sashi @ksashikumar)
- Fix file name typo !2049
- Unify Docker registry authentication in Docker and Kubernetes executors !2048
- Improve Kubernetes executor attach strategy command execution and handling by using a new read-logs command in the helper image !2038
- Remove superfluous packages from Ubuntu based Docker image !1781 (J0WI @J0WI)

## v13.0.1 (2020-06-01)

### Bug fixes

- Fix missing logs from Docker executor !2101

## v13.0.0 (2020-05-20)

### Breaking Changes

- Remove support for --Docker-services flag on register command !2036
- Remove fedora/29 package !1905 (Fábio Matavelli @fabiomatavelli)
- Remove /debug/jobs/list?v=1 endpoint !1894 (Fábio Matavelli @fabiomatavelli)
- Remove backported os.Expand() implementation !1892 (Fábio Matavelli @fabiomatavelli)
- Remove FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER feature flag !1889 (Fábio Matavelli @fabiomatavelli)
- Remove macOS 32 bit support !2051
- Remove support for Windows 1803 !2033
- Remove legacy build directory caching in Docker Executor !2067
- Remove support for array of strings when defining services for Docker Executor !2035

### New features

- Support more glob patterns for artifact/cache !1917
- Add arm64 Docker images for GitLab/GitLab-runner !1861
- Make Docker machine configuration more elastic !1980
- Add support for `direct_download` artifacts !2093

### Bug fixes

- Fix duplicate volume check with trailing slash !2050
- Fix permissions of Docker volumes created by Runner !2047
- Fix removal of build volume when disable_cache set to true !2042
- Fix err checks from volume manager !2034
- Revert "Merge branch '4450-fix-container-wait' into 'master'" !2026

### Maintenance

- Retry Docker build jobs !2087
- Update installation of mockery !2085
- Fix Docker Auth config to be platform agnostic !2077
- Refactor tests in builds_helper_test !2057
- Enable unused linter !2043
- Remove support for array of strings when defining services for Docker Executor !2035
- Update assertion for Docker test !2031
- Add tests for Docker config read when no username is specified !2024 (Andrii Zakharov @andriiz1)
- Skip flaky TestDockerCommandRunAttempts until fix is merged !2017
- Remove prealloc linter !2014
- Pin CI jobs to GitLab-org runners !1979
- Replace Code Climate with golangci-lint !1956
- Change license management to use rules !2096

### Documentation changes

- Update capitalization configuration !2084
- Update proxy.md documentation for grammar and clarity !2071 (Kade Cole @kadecole)
- Add link to AWS Fargate documentation page !2070
- Adds the link to new AWS Fargate page !2068
- Add more Vale rules to project !2061
- Remove tip alert box !2054
- Added Kaniko reference materials to Runner Helm charts page !2039
- Sync Vale substitutions rules from GitLab project !2029
- Update PowerShell documentation to include video and working example project. !2028
- Handle situation where vale docs-lint error is overwritten by markdownlint success !2025
- Update faq to include firewall troubleshooting !2023
- Add recommended extensions for VS Code !2022
- Move documentation linting to Makefile !2021
- Add section about using TLS with custom CA in regular build scripts !2018
- Sync markdownlint settings from GitLab project !2015
- Fixed Helm search command !2007 (penguindustin @penguindustin)
- Improve signals documentation and add a best practice for graceful shutdown !1988
- Make Docker machine configuration more elastic !1980
- Autoscale GitLab Runner on AWS Fargate configuration doc !1914
- Add details about how pull always is still fast and efficient !1885 (Ben Bodenmiller @bbodenmiller)
- Correct documentation inaccuracies for `OffPeakPeriods` !1805 (Wes Cossick @wescossick)
- Removed `CONTAINER_ID` in prepare.sh, so `CONTAINER_ID` in base.sh is used. !1723 (JUN JIE NAN @nanjj)

## v12.10.0 (2020-04-21)

### New features

- Allow Windows 1909 for Docker executor !1999
- Allow windows 1903 for Docker executor !1984
- Add support for `raw` variables !1882

### Bug fixes

- Add attempts to Docker executor for container not found !1995
- Use Docker volumes instead of cache containers !1989
- Use unique container names for Docker executor !1801

### Maintenance

- Fix TestScanHandlesCancelledContext having a WaitGroup without a delta and a few other log processor flaky tests !1961
- Rename `docker_helpers` to `docker` !1943
- Add retry when executing commands with kube attach !1907
- Fix golint issue for error starting with capital letter !1851
- Fix some Windows Docker executor test !1789

### Documentation changes

- Minor Update index.md !2004 (KATO Tomoyuki @tomo667a)
- Minor rewording in PROCESS.md templates !2003
- Add further checks from GitLab project !2001
- Add info that SSH is also required to be accessible in the security group !1997 (Daniel Schwiperich @d.schwiperich)
- Add Vale version text rule !1994
- Clean up note style !1993
- Fix redirected links in docs !1992
- Updates markdownlint configuration from GitLab project !1991
- Added link to the Git download page !1972
- Pull policy security concerns apply to Kubernetes executors too !1886 (Ben Bodenmiller @bbodenmiller)

### Other changes

- Clean Temporary Directories created by the Custom Executor !1978 (Mark McGuire @TronPaul)
- Fix broken master for non existent method call !1974
- Rely on `git ls-files` and `git diff` for checking mocks !1973

## v12.9.0 (2020-03-20)

### New features

- Handle 503 status when uploading artifacts and the object storage is unavailable !1887
- Add trigering of GitLab Runner UBI images pipeline !1869
- Add execution stage name in job trace !1847
- Provide rpm/deb package for arm64 and aarch64 !1826
- Expose CI_JOB_IMAGE env var on build environment !1813
- Create network per build in Docker executor !1569 (Steve Exley @steve.exley)
- Overwrite Kubernetes resource limits and requests for build container on job level !874 (Nicholas Colbert @45cali)

### Bug fixes

- Kubernetes execute commands with attach instead of exec !1775
- Retry Kubernetes commands when "error dialing backend: EOF" error is hit !1867

### Maintenance

- Upgrade Docker client to version 19.03.5 for CI image !1874
- Fix Docker machine executor test filename !1927
- Remove executor\_ prefix from filenames in the executors package !1902
- Fix 'make all' target !1900
- Replace changelog generator script !1888
- Bump MinIO to latest version !1881 (Tom Elliff @tomelliff)
- Rename build makefile target to build_all !1873
- Prevent building mips and s390x architectures by default !1872
- Make pipelines running also for X-Y-stable branches !1871
- Add double quotes around bash arguments in ci/release_Docker_images !1865
- Fix release Docker warning !1864
- Fix typo in PowerShell script comments !1862
- Simplify sha256 checksum file creation !1859
- Improve fpm detection !1858
- Replace which command usage !1857
- Convert indentation to spaces for package script !1856
- Update synced path for Windows 10 machine !1854
- Use chocolatey to install software in Vagrant boxes !1853
- Remove redundant type declaration !1852
- Bump to go 1.13.8 !1849
- Add debug logs when setting up pod !1844
- Improve message in Windows version detection !1841
- Set DinD image explicitly to 19.03.5 !1840
- Resolve data race in TestCommand_Run !1839 (Konrad Borowski @KonradBorowski)
- Use $(MAKE) instead of make !1825
- Refactor helpers/container/service pkg !1824

### Documentation changes

- Change document title to include EC2 !1912
- Fix typo in advanced configuration docs !1910 (Geo @geo4orce)
- Improve `Code format` instructions in the process documentation !1899
- Add fedora 30 to supported OS !1896
- Update docs for Windows services since we support services in network per build !1895
- Fix typo in release process docs !1891 (Ranit @ranit.appcode)
- Prevent full disk image copies in libvirt custom executor driver example !1878 (Tom Dohrmann @Freax13)
- Interactive Web terminal does not work on Windows !1877 (Ben Bodenmiller @bbodenmiller)
- List which executors are at risk of having Runner token & other project code stolen !1876 (Ben Bodenmiller @bbodenmiller)
- Allow service alias from config in Kubernetes executor !1868
- Update docs for image variable in custom executor !1866
- Remove bash from codeblock tags !1846
- Improve wording in docs/development/README.md !1837
- Document merge request title requirements and reviewing guidelines !1836
- Add documentation on building from sources !1835
- Update security docs !1833 (masOOd @masood.kamyab)
- Update the monitoring document !1831 (masOOd @masood.kamyab)
- Change flag to Docker-services !1830
- Document Windows supported versions !1533

## v12.8.0 (2020-02-22)

- Define most jobs as 'pipelines for merge requests' !1747
- Build ci images only on related file changes !1746
- Make referees package mocks to be generated by mockery !1729
- Replace Ruby:2.0/2.1 in examples and test cases with Ruby:2.6 !1749
- Update deprecation warning for runner.Docker.service !1751
- Only run image build jobs on main repo !1754
- Fix docs pipelines for merge requests !1756
- Add CI job to check for outdated mocks !1651
- Doc: Extend the compatibility section !1755
- Update `query_interval` to integer !1752
- Update outdated links it comments !1761
- Refactor referees package !1730
- Update process for issue tracker !1742
- Give custom executor scripts a name !1538
- Update only rule for building CI images !1766
- Change Runner src in vagrant configuraiton for Windows development !1767
- Fix broken CI Pipeline Badge on README !1772
- Typo/Grammar improvements in Docker.md !1757
- Fix casing on log !1774
- Fix link to Vagrant in docs/development/README.md !1773
- Add condition when custom executor term happens in documentation !1771
- Fixed kramdown error !1783
- Rename test file !1784
- Fix `--docker-services` flag in register command !1776
- add space before configuration file name in startup message !1785
- Support rate limiting headers from GitLab API !1728
- Update CHANGELOG for v12.7.1 !1787
- Delete changelog to release post script !1788
- Remove an extra '#' !1791
- Update Kubernetes.md, fix typo in `<CERTIFICATE_FILENAME>` !1802
- Update documentation template !1796
- Update AWS spot details in docs !1795
- Change the S3 release index file generator !1803
- Reduce the number for allowed data races !1804
- Fix golint issues for err msgs !1769
- Handle 422 on artifact upload !1794
- Bump Go version to 1.13.7 !1765
- Enabled shared windows runners internal beta !1764
- Fix a typo in S3 release script !1807
- Add one more fix to the S3 release !1808
- Add support for host aliases in services for Kubernetes executor !1680
- Use exec.ExitError.ExitCode() function from go 1.12 !1810
- Fix values.YAML file name in documentation !1812
- Update links to MRs in runner docs !1814
- Update removal date of feature flags to 13.0 !1818
- Increase allowed data races !1815
- Fix panic for exec command !1811
- Update GitHub.com/stretchr/testify dependencies !1806
- Add support for X-GitLab-Trace-Update-Interval header !1760
- Revert 9e1d067621855c7b75820d3a49ac82ef51e56342 !1816
- Cleanup Kubernetes versions when checking for host aliases and don't fail on parse error !1823
- Add GitLab-runner-helper binaries to S3 release !1819
- Minor fixes in advanced-configuration.md !1828
- Remove install Makefile target !1822
- Docs osx install !1829
- Set DinD image explicitly to 19.03.5 !1840
- Make pipelines running also for X-Y-stable branches !1871

## v12.7.1 (2020-01-23)

- Fix `--docker-services` flag in register command !1776

## v12.7.0 (2020-01-20)

- Fixing kramdown link error !1711
- Add caps and backtick testing to runner docs linting !1678
- Fix macOS label !1712
- Align markdownlint config to main repo version !1713
- Bump go version to 1.13.5 !1701
- Remove duplicate service description !1715
- fix(scripts): fix until typo !1717
- Use Prometheus to Query Runner Metrics Linked to Each Job !1545
- Remove unnecessary dependencies from vendor directory !1721
- Remove panic when metrics referee not configured properly !1724
- Add check for go modules !1702
- Update docs for Helm 3 !1727
- Empty Referee configuration on registration !1726
- Extract helperimage pkg outside of parent pkg !1720
- Removed --name argument from Helm install. !1718
- macOS limitations and example update !1505
- Update advanced-configuration.md - fix typo of mperiods !1722
- Fix Typos !1731
- Add a Git version caveat !1732
- Update docs for Windows to use backslashes not forwardslashes !1738
- Do not embed mutex !1734
- Refactor CI pipeline !1733
- Add missing 'needs' entry for release Docker images job template !1744
- docs: Replace Ruby:2.1/2.2 with Ruby:2.6 or 2.7 !1748
- Make 'tags: GitLab-org' a re-usable job tempalte !1745
- Change to go1.13 error wrapping !1709
- Refactor metrics referee tests !1714
- Refactor metrics referee !1725
- Copyedit doc for volumes_from in Docker executor config !1750
- Allow service alias from config in Docker executor !1673

## v12.6.0 (2019-12-22)

- Update list of fixes for Docker-machine fork !1655
- Remove outdated note regarding non-existent 1.8.x brew Go formula in contributing docs !1661
- Add manual rpm and deb installation details !1650
- Remove GetGitTLSVariables method !1663
- Link to example of how to run DinD !1515
- Update feature flag deprecation !1672
- Add timeout when sending request for certificate !1665
- Support Docker options for CPU shares and OOM adjust !1460
- Backport os.Expand from Go v1.10.8 !1677
- Switch to a specific version of govet analyzer !1690
- Update cloud.Google.com/go to v0.49.0 !1682
- Add cmd to helper image !1645
- Update blurb for when people use the issue tracker for support tickets !1691
- Fixing typos !1685
- Remove dead code !1686
- Distribute arm64 binaries !1687
- (Rebased) Update Kubernetes.md adding the missing GitLab-helper container which is ever created. !1693
- Various spelling, punctuation and readability fixes !1660
- Add docs link to arm64 manual install !1694
- Fixed empty_dir name to empty-dir !1681
- Expose image to custom executor !1666
- Reorder methods and add some more logging to RunCommand !1683
- Refactor unused parameters for multi.go !1698
- Migrate to go 1.13 and go modules !1625
- Change log message for failing to set console mode !1662
- Use time.Round from Go stdlib for web terminal !1631
- Close session server on graceful shutdown !1699
- Add deprecation warning for cmd shell in Job log !1659
- Fix rpm signing !1703
- Fix regex for finding virtualbox snapshot name and add tests !1656
- Remove file locking !1710
- Change tone of error on Windows test failure !1610
- Fix CI image build !1707

## v12.5.0 (2019-11-20)

- Update docs for Runner configuration inside of a Docker container !1613
- Remove misleading comment !1622
- Remove absolute paths from chart !1626
- Fix lint on Markdown files !1602
- Document GitLab Docker machine fork !1596
- Update redirected link !1637
- Fix certificates chain generation regression introduced with 12.4.0-rc1 !1639
- Bump Docker_MACHINE_VERSION !1595
- Fix golint issues in machine pkg !1641
- Upgrade to alpine 3.10 !1636
- Fix #4684 for K3s/containerd !1605
- Update makefile to setup dev dependencies before running tests !1589
- Fix external Helm documentation links !1644
- Update Git version for Windows dev environment !1646
- Change config lock to create a separate lock file !1647
- Add few constants to executors/custom/API !1657
- Fix bind propagation for Linux volumes !1632
- Populate a list of machines with machines that might not yet be persisted on disk !914
- Add service definition in config for Kubernetes executor !1476

## v12.4.1 (2019-10-28)

- Fix TLS chain building !1643

## v12.4.0 (2019-10-21)

- Fix err logging for runner limit !1403
- Add the note about incompatibility of session_server with Helm chart runner !1575
- Fix prepare_exec typo in docs !1576
- Docs edits to clarify feature flags motivations and usage in Runner !1568
- Change log levels for common errors !1578
- Extend custom executor config !1583
- Fix JSON inside of docs !1587
- Update link for Helm chart issue tracker !1588
- Add pipeline ID to Docker labels !1592
- Fix typo in helpers/path/windows_path !1594
- Fix broken check for Git LFS that breaks lfs pulling !1599
- Update advanced-configuration.md !1597
- Use certutil to create certificate chain for Git !1581
- Add Go Report Card badge to the README file !1601
- Add pipeline link !1608
- Rename mentions of OSX to MacOS !1440
- Enable pinentry mode to loopback for GPG signing !1614
- Update various runner doc links !1585
- Add note about IAM role usage for s3 cache configuration !1598
- Bump used Go version to 1.10.8 !1617
- Update gopkg.in/YAML.v2 !1619
- Update Prometheus libraries !1620
- Bump GitHub.com/JSON-iterator/go to 1.1.7 !1621
- Update k8s client go to 11.0 !1615
- Rename log to trace in runner docs !1616
- Change Review priority label meaning !1600
- Add timeout when waiting for the build to finish !1609

## v12.3.0 (2019-09-20)

- Change log levels for common errors !1578
- Update a redirected link !1520
- Removal of conditions which are always evaluated either to true or false !1517
- Add initial docs for best practice !1509
- Update VirtualBox executor docs !1527
- Document configuration template file feature !1522
- Rename landing page for consistency !1528
- Edit new config template file section !1529
- Update windows dev environment to Git 2.22 !1530
- Update PowerShell ErrorActionPreference documentation !1535
- Remove Debian buster from package list !1536
- Update tls-self-signed.md !1537
- Improve windows helper images build !1519
- show which service exactly is invalid !1531
- Change docs Markdown linter from mdl to markdownlint !1540
- Replace bastion with Runner Manager !1547
- Add entry to FAQ, restructure also !1539
- Change docs review and cleanup jobs to same CI stage !1543
- Docker.md: Correct Image Sizes !1542
- Add note on shell-based Docker image requirement !1459
- Fixed PowerShell commands for Windows Runner !1544
- Remove the scripting for release checklist issue creation !1556
- Use new location for Helm charts repo !1553
- Make Notes look consistent !1555
- Change markdownlint wildcard format !1554
- Edit Docker images section !1550
- Update capitalization in runner docs !1559
- Docs/update Ubuntu dev docs !1557
- Use standard commands for directory creation to make it PowerShell core compatible !1563
- Fix exiting with zero exit code when cmdlets fail !1558
- Enable support for long paths !1524
- Prevent dollar signs in shell codeblocks !1574
- Clarify feature flag usage instructions !1566
- Expose variable containing the 'short token' value !1571
- Update documentation about OffPeakTimezone !1567
- Set default PATH for helper image !1573

## v12.2.0 (2019-08-22)

- Update docs executor titles !1454
- Only default to PowerShell on Windows if no other shell is specified !1457
- Add more MDL rules !1462
- Add PROCESS.md !1410
- Fix wrong rc script for freebsd. !1418
- Allow to build development version of DEB, RPM and Docker with make !824
- Add custom executor documentation !1416
- docs: clarify the requirements for pinning !823
- Adds explanation of our review label system. !1461
- Use FreeBSD's built-in stop and status scriplets from /etc/rc.subr !757
- Fix typo on security docs !956
- Update doc about Debian version !1464
- Move note to subsection !1469
- Correct spelling in help string !1471
- Force an opt-out from Docker Machine bugsnag report !1443
- Improved go install instructions for macOS !1472
- Fix some linting issues !1424
- Make it clear what is the default shell for Windows !1474
- Add LXD example for custom executor !1439
- Add libvirt custom executor example !1456
- Update self-signed certificate docs for Windows service !1466
- Docs/update min Docker version !1480
- Docs: Fix typo in custom executor !1479
- Track Windows tests failures !1450
- Add requirements for contributing new hardware architectures !1478
- Fix Markdown in runner docs (part 1) !1483
- Fix Markdown in runner docs (part 2) !1484
- Update docs to specify default shell of OS !1485
- Further clarify Docker requirements !1486
- Fix typo and spacing in two runner docs !1487
- docs: GitLab-runner helper image has no arm64 build yet !1489
- Fix custom executor default config on register !1491
- Update Windows test failures !1490
- Expand Markdown lint rules in runner !1492
- Fix PowerShell capitalization !1497
- Quarantine more windows tests !1499
- Update tracked Windows tests failures list !1502
- Quarantine windows tests !1501
- Add docs for tls_verify config field !1493
- Reorder methods in abstract.go to bring calees closer to the callers !1481
- Update docs about bash on windows not working !1498
- Cleanup commands/config.go !1494
- Switch to DinD TLS for GitLab CI !1504
- Add .gitattributes !1122
- Prevent running multiple instances of the GitLab-runner process using the same configuration file !1496
- Update test assertion !1510
- Remove need for externally configured variable !1512
- Change CI_COMMIT_REF to CI_COMMIT_SHA in docs !1513
- Update reference to CI_COMMIT_REF to CI_COMMIT_SHA !1514
- Configuration file template for registration command !1263
- Update AWS autoscaling docs !1518
- Add test for <at> and <colon> masking !1516

## v12.1.0 (2019-07-22)

- Extend custom executor with configuration injects !1449
- Fix "WARNING: apt does not have a stable CLI interface. Use with caution in scripts" !1143
- Fix artifact uploading for Windows Docker containers !1414
- Upgrade base image for GitLab/GitLab-runner:ubuntu to Ubuntu:18.04 !1413
- Add tip to execute batch from PowerShell !1412
- Replace wget commands with curl commands !1419
- Wrap submodule command with a string !1411
- Add missing test cases for s3 IAM checks !1421
- Add Markdown linting and one rule !1422
- Fix indentation for docs !1417
- Add docs for not supporting LCOW !1415
- Disallow bare URLs from project !1425
- Update zglob !1426
- Add note in docs for mounting volumes to services !1420
- Clarify docs for `builds_dir` & `cache_dir` !1428
- Update docs to fix Markdown and square bracket use !1429
- Enforce consistent prefix for numbered lists !1435
- Remove fedora/30 from supported list !1436
- Add STOPSIGNAL to GitLab-runner Docker images !1427
- Add trace entry for Docker authConfig resolving !1431
- Enforce consistent prefix for bullet lists !1441
- Fix concurrent updates !1447
- docs: add --config for install command !1433
- Document why we no longer accept new executors !1437
- Document limitation for Windows Docker target drive !1432
- Trivial update to virtualbox.md - 'shutdown' is not the verb, barely the noun. !1445
- Update description of flag in docs !1451
- Docs: Update redirected links in runner docs !1453
- Add lint rule that headings must increment one level at a time !1452
- Add custom executor !1385

## v12.0.0 (2019-06-21)

**Release notices:**

With GitLab Runner 12.0 we're adding several breaking changes:

- [Require refspec to clone/fetch Git repository](https://gitlab.com/gitlab-org/gitlab-runner/issues/4069).
- [Change command line API for helper images usage](https://gitlab.com/gitlab-org/gitlab-runner/issues/4013).
- [Remove old cache configuration](https://gitlab.com/gitlab-org/gitlab-runner/issues/4070).
- [Remove old metrics server configuration](https://gitlab.com/gitlab-org/gitlab-runner/issues/4072).
- [Remove `FF_K8S_USE_ENTRYPOINT_OVER_COMMAND` feature flag and old behavior](https://gitlab.com/gitlab-org/gitlab-runner/issues/4073).
- [Remove support for few Linux distributions that reached EOL](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1130).
- [Remove old `git clean` flow](https://gitlab.com/gitlab-org/gitlab-runner/issues/4175).

Please look into linked issues for details.

**Release changes:**

- Support windows Docker volumes configuration !1269
- Fix PowerShell cloning !1338
- Docs: Update Docker register non-interactive command !1309
- Update mocks !1343
- Change source for go-homedir !1339
- improve MR and issues templates !1347
- docs: reuse previous clone !1346
- Prevent copy and paste error due to not existed alpine tag. !1351
- Fix typo for usage of proxies within Docker containers for runners !1342
- Add documentation for Windows Docker Executor !1345
- Fix volume mounting when mode specified !1357
- Update docs for Docker executor description !1358
- Show error when volume length is not expected !1360
- Add feature flag to mounting volumes to services !1352
- Implement session endpoint to proxy build services requests !1170
- add build info for fedora 30 !1353
- Limit `docker-windows` to Windows !1362
- Update logging key for Docker Machine !1361
- Update docs to refer to Windows Batch deprecation !1371
- Remove deprecated Git clean strategy !1370
- Remove support for deprecated metrics_server setting !1368
- Add labels to templates !1375
- Remove support for deprecated entrypoint configuration for K8S !1369
- Fix support for SELinux volume mounts & case sensitivity !1381
- Remove old Docker helper image commands !1373
- Remove support for deprecated S3 cache configuration !1367
- Added --system flag information into GitLab-runner install command !1378
- Minor Markdown fixes !1382
- Remove support for deprecated distributions !1130
- Add configuration of access_level for runners on registration !1323
- Remove doc notice for deprecated OSes !1384
- Remove deprecated clone/fetch command !1372
- Allow configuration of Pod Security Context by Kubernetes Exeutor !1036
- Fix case sensitivity for windows volumes !1389
- Accept Docker-windows as an option on register !1388
- Add documentation for windows development !1183
- Document clear-Docker-cache script !1390
- Store traces on disk !1315
- Make Git init to be quiet !1383
- Fix several typos !1392
- Make volumes to work on linux Docker on windows !1363
- Update CHANGELOG.md with 11.11.x patch releases !1393
- Dependencies license management with GitLab CI/CD !1279
- Fix default cache volume Docker-windows register !1391
- Fixed date typo for v11.11.2 CHANGELOG entry !1394
- Update GitHub.com/Microsoft/go-winio dependency !1348
- Update compatibility heading as it's no longer a chart/table !1401
- Docker Credentials helper support !1386
- Numerous typos fixed !1258
- Update some logrus fields used in Runner logs !1405
- Update osx.md so the update instructions work as well as the install instructions !1402
- Make PowerShell default for new registered Windows shell executors !1406
- Restore gofmt rules from before codeclimate update !1408
- Update logrus to v1.4.0 !1407

## v11.11.2 (2019-06-03)

- Fix support for SELinux volume mounts & case sensitivity !1381
- Fix case sensitivity for windows volumes !1389
- Update logging key for Docker Machine !1361
- Limit `docker-windows` to Windows !1362
- Make volumes to work on linux Docker on windows !1363

## v11.11.1 (2019-05-24)

- Fix volume mounting when mode specified !1357
- Add documentation for Windows Docker Executor !1345
- Add feature flag to mounting volumes to services !1352

## v11.11.0 (2019-05-22)

- Fix PowerShell cloning !1338
- Add PowerShell support for Docker Executor !1243
- Support windows Docker volumes configuration !1269
- Fix Git LFS not getting submodule objects !1298
- Add homebrew installation method for macOS runners !837
- mention the 59th second timeperiod issue in the docs !490
- Refactor macOS install instructions !1303
- Edit note on edge case !1304
- Extract unsupportedOSTypeError to errors pkg !1305
- Optimise trace handling for big traces !1292
- Cleanup feature flags mess !1312
- Add more documentation for node tolerations !1318
- Typo: varialbes -> variables !1316
- Allow to configure FF using config.TOML !1321
- Update link to the introduction of custom build directories !1302
- Allow to use FF to configure `/builds` folder !1319
- Create a single source of truth for feature flags !1313
- Clear up docs on how to select shell !1209
- Update feature flag documentation !1326
- Refactor Helper Image package to work with Kubernetes !1306
- Fix broken internal links !1332
- Refactor helperimage package tests !1327
- Change deprecation of FF_USE_LEGACY_BUILDS_DIR_FOR_Docker to 12.3 !1330
- Update cmd script example !1333
- Better explain the workflow in Docker executors doc !1310
- Exclude mock files from coverage reporting !1334
- Fix link syntax in advanced-configuration.md !1311
- Docs: Update contributing links from GitLab-ce !1308
- Update Docker executor Executor Options initialization !1296
- Add test case for Linux helper image !1335
- Extract volumes configuration to a separate struct !1261

## v11.10.0 (2019-04-22)

**Deprecations:**

All deprecations, with a detailed description, are listed at
<https://about.gitlab.com/2019/04/22/gitlab-11-10-released/#release-deprecations>

1. With version 11.10 we're deprecating the feature flag
   [FF_USE_LEGACY_GIT_CLEAN_STRATEGY](https://docs.gitlab.com/runner/configuration/feature-flags/#available-feature-flags).

**Release changes:**

- Fix Git LFS not getting submodule objects !1298
- Refactor slightly ./shells/shellstest !1237
- Fix CI_PROJECT_DIR handling !1241
- Log time took preparing executors !1196
- Restore availability of pprof in the debug server !1242
- Move variables defining .gopath to a shared place for all Windows jobs !1245
- Docs: clarify runner API registration process !1244
- add lfs support to Ubuntu Docker runner !1192
- Add information about Kaniko for Kubernetes executor !1161
- Enable the docs CI job !1251
- Rename test to be more descriptive !1249
- Create the reviewers guide base document !1233
- Update codeclimate version !1252
- Add retryable err type !1215
- Get windows tag for helper image !1239
- Remove unnecessary log alias for logrus inport !1256
- Make GitLab-runner:alpine more specific, Add link to Dockerfiles sources,... !1259
- Docs: Fix broken anchor in Docker.md !1264
- Replace the current k8s manual installation with the Helm chart !1250
- Create cache for `/builds` dir !1265
- Expose `CI_CONCURRENT_(PROJECT)_ID` !1268
- DOC: note on case-sensitive proxy variables and the need for upper and lower case versions !1248
- Add new links checker !1271
- Update log messages for listen & session address !1275
- Use delayed variable expansion for error check in cmd !1260
- Unexport common.RepoRemoteURL !1276
- Update index.md - added sudo when registering the service on macos (without... !1272
- Add new lines around lists for renderer !1278
- Fix color output on Windows !1208
- Make it again possible to disable Git LFS pull !1273
- Add cross references to Runners API !1284
- Improve support for `git clean` !1281
- Make Kubernetes executor to clone into /builds !1282
- Add option to specify clone path !1267
- Allow to disable debug tracing !1286
- Add Route Map for runner docs !1285
- Do not print remote addition failure message !1287
- Add true to the run-untagged subcommand !1288
- Cleanup k8s cleanup test !1280
- Change helper image to servercore !1290
- Add note about Git-lfs !1294

## v11.9.2 (2019-04-09)

- Fix Git LFS not getting submodule objects !1298

## v11.9.1 (2019-04-03)

- Make it again possible to disable Git LFS pull !1273
- Use delayed variable expansion for error check in cmd !1260
- Unexport common.RepoRemoteURL !1276

## v11.9.0 (2019-03-22)

**Deprecations:**

All deprecations, with a detailed description, are listed at
<https://about.gitlab.com/2019/03/22/gitlab-11-9-released/#release-deprecations>

1. With version 11.9 we're deprecating the support for Docker Executor on CentOS 6

2. With version 11.9 we've implemented a new method for cloning/fetching repositories.
   Currently GitLab Runner still respects the old configuration sent from GitLab, but with
   12.0 old methods will be removed and GitLab Runner will require at least GitLab 11.9
   to work properly.

3. With version 11.0 we've changed how the metrics server is configured for GitLab Runner.
   `metrics_server` was replaced with `listen_address`. With version 12.0 the old configuration
   option will be removed.

4. With version 11.3 we've implemented support for different remote cache providers, which
   required a change in how the cache is configured. With version 12.0 support for old
   configuration structure will be removed.

5. With version 11.4 we've fixed the way how `entrypoint:` and `command:` options of
   Extended Docker configuration (<https://docs.gitlab.com/ci/docker/using_docker_images/#extended-docker-configuration-options>)
   are being handled by Kubernetes Executor. The previous implementation was wrong and
   was making the configuration unusable in most cases. However some users could relay
   on this wrong behavior. Because of that we've added a feature flag `FF_K8S_USE_ENTRYPOINT_OVER_COMMAND`
   which, when set to `false`, could bring back the old behavior. With version 12.0 the
   feature flag as well as the old behavior will be removed.

6. Some Linux distributions for which GitLab Runner is providing DEB and RPM packages
   have reached their End of Life. With version 12.0 we'll remove support for all
   EoL distributions at the moment of 12.0 release.

7. With version 11.9 we've prepared a go-based replacement for Runner Helper commands
   executed within Docker executor inside of the Helper Image. With version 12.0
   we will remove support for old commands basing on bash scripts. This change will
   affect only the users that are configuring their custom Helper Image (the image
   will require an update to align with new requirements)

**Release changes:**

- fix(parallels): use the newer sntp command to time sync !1145
- Update Docker API verion !1187
- Update alpine images to alpine 3.9 !1197
- Fix a typo in the description of the configuration option !1205
- Document creation of Docker volumes passed with Docker exec --Docker-volumes !1120
- Correct spelling of timed out in literals !1121
- Fix spelling and other minor improvements !1207
- Migrate service wait script to Go !1195
- Docs update: Run runner on Kubernetes !1185
- Increase test timeout for shell executor !1214
- Follow style convention for documentation !1213
- Add test for runner build limit !1186
- Migrate cache bash script to Go for helper image !1201
- Document OS deprecations for 12.0 !1210
- Fix anchors in Runner documentation !1216
- Add `build_simple` to `help` make target !1212
- Split `make docker` for GitLab Runner Helper !1188
- Add windows Dockerfiles for GitLab-runner-helper !1167
- Make Runner tests working on Windows with our CI Pipeline !1219
- Fetch code from provided refspecs !1203
- Check either ntpdate command exists or not before trying to execute it !1189
- Deprecate helper image commands !1218
- Add script for building windows helper image !1178
- Fix ShellWriter.RmFile(string) for cmd shell !1226
- Mask log trace !1204
- Add note about pod annotations for more clarity !1220
- Resolve memory allocation failure when cloning repos with LFS objects bigger than available RAM !1200
- Release also on GitLab releases page !1232
- Restore availability of pprof in the debug server !1242

## v11.8.0 (2019-02-22)

- Kubernetes executor: add support for Node tolerations !941
- Update logrus version to v1.3.0 !1137
- Docs - Clarify Docker Runner Documentation !1097
- Update GitHub.com/stretchr/testify dependency !1141
- Update LICENSE file !1132
- Update example of cache config !1140
- Update documentation for autoscaling on AWS !1142
- Remove unnecessary dep constraint !1147
- readme: make author block render md !999
- Corrected note when using a config container to mount custom data volume. !1126
- Fix typo in documentation of k8s executor. !1118
- Make new runner tokens compatible with Docker-machine executor !1144
- docs: Use `sudo tee` for apt pinning. !1047
- docs: fix indendation !1081
- Updated hint on running Windows 10 shell as administrator !1136
- Fixed typo in logged information !1074
- Update registry_and_cache_servers.md !1098
- Update golang.org/x/sys !1149
- Refactor frontpage for grammar and style !1151
- Update GitHub.com/Azure/go-ansiterm dependency !1152
- Testing on windows with vagrant !1003
- Add fix for race condition in windows cache extraction !863
- Consolidate Docker API version definition !1154
- Prevent Executors from modifying Runner configuration !1134
- Update ExecutorProvider interface signature !1159
- Update logging for processing multi runner !1160
- Update Kubernetes.md - fix typo for bearer_token !1162
- Update GitHub.com/Prometheus/client_golang dep !1150
- Remove ContainerWait from Docker client !1155
- Update advanced-configuration.md: Fix blockquote not reaching the entire note !1163
- Fix docs review app URL !1169
- docs: Add a helpful command to reload config !1106
- Update AWS autoscale documentation !1166
- Refactor dockerfiles !1068
- Add link to AWS driver about default values !1171
- Add support for fedora/29 packages !1082
- Add windows server 2019 as default for windows development !1165
- Docs: Fix bad anchor links in runner docs !1177
- Improve documentation concerning proxy setting in the case of Docker-in-Docker-executor !1090
- Add few fixes to Release Checklist template !1135
- Set table to not display under TOC !1168
- Update Docker client SDK !1148
- docs: add GitLab Runner Helm Chart link !945

## v11.7.0 (2019-01-22)

- Docs: Cleaning up the executors doc !1114
- Update to testify v1.2.2 !1119
- Fix a typo in VirtualBox Executor docs !1124
- Use the term `macOS` instead of `OS X` or `OSX` !1125
- Update GitHub.com/sirupsen/logrus dependency !1129
- Docs update release checklist !1131
- Kill session when build is cancelled !1058
- Fix path separator for CI_PROJECT_DIR in Windows !1128
- Make new runner tokens compatible with Docker-machine executor !1144

## v11.6.0 (2018-12-22)

- Make compatibility chart super clear and remove old entries !1078
- Add Slack notification option for 'dep status' check failures !1072
- Docker executor: use DNS, DNSSearch and ExtraHosts settings from configuration !1075
- Fix some invalid links in documentation !1085
- Fix SC2155 where shellcheck warns about errors !1063
- Change parallel tests configuration ENV names !1095
- Improve documentation of IAM instance profile usage for caching !1071
- Remove duplicate builds_dir definition from docs !952
- Make k8s object names DNS-1123 compatible !1105
- Docs: working example of helper image with CI_RUNNER_REVISION !1032
- Docs: omit ImagePullPolicy !1107
- Disable the docs lint job for now !1112
- Docs: comment about how listen_address works !1076
- Fix the indented bullet points of the features list in documentation !1093
- Add note on the branch naming for documentation changes !1113
- Docs: add session-server link to advanced list in index !1108

## v11.5.0 (2018-11-22)

- Support RAW artifacts !1057
- Docs: changing secret variable to just variable in advanced-configuration.md !1055
- Docs: Fixing some bad links in Runner docs. !1056
- Docs: Updating Docs links from /ce to /ee !1061
- Docs: Fixing Substrakt Health URL !1064
- Add failure reason for execution timeout !1051

## v11.4.0 (2018-10-22)

- Do not create apk cache !1017
- Handle untracked files with Unicode characters in filenames. !913
- Add metrics with concurrent and limit values !1019
- Add a GitLab_runner_jobs_total metric !1018
- Add a job duration histogram metric !1025
- Filter content of X-Amz-Credential from logs !1028
- Disable escaping project bucket in cache operations !1029
- Fix example for session_server and added the note about where this section should be placed !1035
- Fix job duration counting !1033
- Log duration on job finishing log line !1034
- Allow disabling Docker entrypoint overwrite !965
- Fix command and args assignment when creating containers with K8S executor !1010
- Support JSON logging !1020
- Change image for docs link checking !1043
- Fix command that prepares the definitions of tests !1044
- Add OomKillDisable option to Docker executor !1042
- Add Docker support for interactive web terminal !1008
- Add support Docker machine web terminal support !1046

## v11.3.0 (2018-09-22)

- Fix logrus secrets cleanup !990
- Fix test failure detection !993
- Fix wrongly generated `Content-Range` header for `PATCH /api/v4/jobs/:id/trace` request !906
- Improve and fix release checklist !940
- Add ~"Git operations" label to CONTRIBUTING guide !943
- Disable few jobs for docs-/-docs branches !996
- Update release checklist issue template !995
- Fix HTTPS validation problem when SSH executor is used !962
- Reduce complexity of reported methods !997
- Update Docker images to alpine:3.8 !984
- Fail build in case of code_quality errors !986
- Add initial support for CI Web Terminal !934
- Make session and metrics server initialization logging consistent !994
- Make prepare-changelog-entries.rb script compatible with GitLab APIv4 !927
- Save compilation time always in UTC timezone !1000
- Extend debug logging for k8s executor !949
- Introduce GCS adapter for remote cache !968
- Make configuration of helper image more dynamic !1005
- Logrus upgrade - fix data race in helpers.MakeFatalToPanic() !1011
- Add few TODOs to mark things that should be cleaned in 12.0 !1013
- Update debug jobs list output !992
- Remove duplicate build_dir setting !1015
- Add step for updating Runner Helm chart !1009
- Clenup env, cli-options and deprecations of cache settings !1012

## v11.2.0 (2018-08-22)

- Fix support for Unicode variable values when Windows+PowerShell are used !960
- Update docs/executors/Kubernetes.md !957
- Fix missing code_quality widget !972
- Add `artifact` format !923
- Improve some k8s executor tests !980
- Set useragent in Kubernetes API calls !977
- Clarifying the tls-ca-file option is in the [[runners]] section !973
- Update mocks !983
- Add building to development heading !919
- Add coverage report for unit tests !928
- Add /etc/nsswitch.conf to helper on Docker executor to read /etc/hosts when upload artifacts !951
- Add busybox shell !900
- Fix support for features for shells !989
- Fix logrus secrets cleanup !990
- Fix test failure detection !993

## v11.1.0 (2018-07-22)

- Fix support for Unicode variable values when Windows+PowerShell are used !960
- Unify receivers used for 'executor' struct in ./executors/Docker/ !926
- Update Release Checklist template !898
- Cache the connectivity of live Docker Machine instances !909
- Update Kubernetes vendor to 1.10 !877
- Upgrade helper image alpine 3.7 !917
- Detect possible misplaced boolean on command line !932
- Log 'metrics_server' deprecation not only when the setting is used !939
- Speed-up ./executor/Docker/executor_Docker_command_test.go tests !937
- Remove go-bindata !831
- Fix the release of helper images script !946
- Sign RPM and DEB packages !922
- Improve Docker timeouts !963
- Wrap all Docker errors !964

## v11.0.0 (2018-06-22)

- Resolve "Invalid OffPeakPeriods value, no such file or directory." !897
- Add --paused option to register command !896
- Start rename of "metrics server" config !838
- Update virtualbox.md temporary fix for #2981 !889
- Fix panic on PatchTrace execution !905
- Do not send first PUT !908
- Rename CI_COMMIT_REF to CI_COMMIT_SHA !911
- Fix test file archiver tests !915
- Document how check_interval works !903
- Add link to development guide in readme !918
- Explain GitLab-runner workflow labels !921
- Change Prometheus metrics names !912

## v10.8.0 (2018-05-22)

- Resolve "Invalid OffPeakPeriods value, no such file or directory." !897
- Fix type in Substrakt Health company name !875
- Rename libre to core !879
- Correct hanging parenthesis in index.md !882
- Update interfaces mocks !871
- Rename keyword in Kubernetes executor documentation !880
- Temporary add 'retry: 2' for 'unit tests (no race)' job !885
- Update docs/executors/README.md !881
- Add support for fedora/27 and fedora/28 packages !883
- Update supported distribution releases !887
- Automatize release checklist issue creation !870
- Change docs license to CC BY-SA 4.0 !893
- Update Docker installation method docs !890
- Add new metrics related to jobs requesting and API usage !886

## v10.7.0 (2018-04-22)

- Rename Sirupsen/logrus library !843
- Refer to GitLab versions as libre, starter, premium, and ultimate !851
- Fix assert.Equal parameter order !854
- Upgrade Docker-machine to v0.14.0 !850
- Refactor autoscale docs !733
- Add possibility to specify memory in Docker containers !847
- Upgrade helper image to alpine 3.6 !859
- Update Docker images bases to alpine:3.7 and Ubuntu:16:04 !860
- Verify Git-lfs checksum !796
- Improve services health check !867
- Add proxy documentation !623
- Downgrade go to 1.8.7 !869
- Add support for max_job_timeout parameter in registration !846

## v10.6.0 (2018-03-22)

- Upgrade Docker-machine to v0.14.0 !850
- Upgrade helper image to alpine 3.6 !859
- Add CI_RUNNER_VERSION, CI_RUNNER_REVISION, and CI_RUNNER_EXECUTABLE_ARCH job environment variables !788
- Always prefer creating new containers when running with Docker Executor !818
- Use IAM instance profile credentials for S3 caching !646
- exec command is no longer deprecated !834
- Print a notice when skipping cache operation due to empty cache key !842
- Switch to Go 1.9.4 !827
- Move dependencies to dep !813
- Improve output of /debug/jobs/list !826
- Fix panic running Docker package tests !828
- Fixed typo in console output !845

## v10.5.0 (2018-02-22)

- Always prefer creating new containers when running with Docker Executor !818
- Improve output of /debug/jobs/list !826
- Fix panic running Docker package tests !828
- Fix Git 1.7.1 compatibility in executors/shell package tests !791
- Do not add /cache volume if already provided by the user during GitLab-runner register !807
- Change confusing Built value for development version !821
- docs: explain valid values for check_interval !801
- docs: Fix OffPeak variables list !806
- docs: Add note about GitLab-runner on the SSH host being used for uploads !817

## v10.4.0 (2018-01-22)

- Always load OS certificate pool when evaluating TLS connections !804
- Add (overwritable) pod annotations for the Kubernetes executor !666
- Docker.allowed_images can use glob syntax in config.TOML !721
- Added Docker runtime support !764
- Send `failure_reason` when updating job statues (GitLab API endpoint) !675
- Do not use `git config --local` as it's not available in Git v1.7.1 !790
- Use local GOPATH in Makefile !779
- Move Bleeding Edge release from Ubuntu/yakkety to ububut/artful !797
- Fix data race in commands package unit tests !787
- Fix data race in function common.(\*Trace).Write() !784
- Fix data races in executor/Docker package !800
- Fix data races in network package !775

## v10.3.1 (2018-01-22)

- Always load OS certificate pool when evaluating TLS connections !804

## v10.3.0 (2017-12-22)

- Do not use `git config --local` as it's not available in Git v1.7.1 !790
- new RC naming schema !780
- Stop Docker Machine before removing it !718
- add `--checkout --force` options to `git submodule update --init` !704
- Fix trailing "<nil>" in syslog logging !734
- Fix Kubernetes executor job overwritten variables behavior !739
- Add zip archive for windows release files !760
- Add Kubernetes executor connection with service account, bearer token can also be overwritten !744
- Fix SIGSEGV in Kubernetes executor Cleanup !769

## v10.2.1 (2018-01-22)

- Do not use `git config --local` as it's not available in Git v1.7.1 !790
- Always load OS certificate pool when evaluating TLS connections !804

## v10.2.0 (2017-11-22)

- Update supported platforms !712
- Fix typo in Kubernetes runner docs !714
- Add info on upgrading to Runner 10 !709
- Add some documentation for disable_cache configuration option !713
- Remove .Git/HEAD.lock before Git fetch !722
- Add helper_image option to Docker executor config !723
- Add notes about GitLab-runner inside the VM being used for uploads !719
- Fix panic when global flags are passed as command flags !726
- Update MinIO go library to v3.0.3 !707
- Label ci_runner_builds metric with runner short token !729

## v10.1.1 (2018-01-22)

- Do not use `git config --local` as it's not available in Git v1.7.1 !790
- Always load OS certificate pool when evaluating TLS connections !804

## v10.1.0 (2017-10-22)

- Allow customizing go test flags with TESTFLAGS variable !688
- Clarify that cloning a runner could be considered an attack vector !658
- Remove disable_verbose from docs !692
- Add info about pre 10.0 releases !691
- Update BurntSushi/TOML for MIT-license !695
- Expose if running in a disposable environment !690
- Adds EmptyDir support for k8s volumes !660
- Update Git-lfs to 2.3.1 !703
- Collect metrics on build stages !689
- Construct Git remote URL based on configuration !698
- Set Git SSL information only for GitLab host !687

## v10.0.2 (2017-10-04)

- Hide tokens from URLs printed in job's trace !708

## v10.0.1 (2017-09-27)

- Remove deprecation message from service management commands !699

## v10.0.0 (2017-09-22)

> **Note:** With 10.0, we've moved repository from <https://gitlab.com/gitlab-org/gitlab-ci-multi-runner>
> to <https://gitlab.com/gitlab-org/gitlab-runner>. Please update your Bookmarks!

> **Note:** Starting with 10.0, we're marking the `exec` and service-related commands as **deprecated**. They will
> be removed in one of the upcoming releases.

> **Note:** Starting with 10.0, we're marking the `docker-ssh` and `docker-ssh+machine` executors as **deprecated**.
> They will be removed in one of the upcoming releases.

> **Note:** Starting with 10.0, behavior of `register` command was slightly changed. Please look into
> <https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/657> for more details.

- Lock runners to project by default on registration !657
- Update cli library !656
- Fix RunSingleCommand race condition in waitForInterrupts !594
- Add handling of non-existing images for Docker >= 17.07 !664
- Document how to define default image to run using Kubernetes executor !668
- Specify an explicit length for Git rev-parse --short to avoid conflicts when run !672
- Add link to Kubernetes executor details !670
- Add install VirtualBox step & improve VM setup details !676
- Rename repository from GitLab-ci-multi-runner to GitLab-runner !661
- Fix variable file permission !655
- Add Release Checklist template !677
- Fix randomly failing test from commands/single_test.go !684
- Mark Docker-SSH and Docker-SSH+machine executors as DEPRECATED !681
- Mark exec and service-management commands as DEPRECATED !679
- Fix support for `tmpfs` in Docker executor config !680

## v9.5.1 (2017-10-04)

- Hide tokens from URLs printed in job's trace !708
- Add handling of non-existing images for Docker >= 17.07 !664

## v9.5.0 (2017-08-22)

- Fix allowed_images behavior !635
- Cleanup formatting on windows upgrade details !637
- Names must meet the DNS name requirements (no upper case) !636
- Execute steps for build as-is, without joining and splitting them !626
- Fix typo on killall command !638
- Fix usage of one image for multiple services in one job !639
- Update Docker Machine to 0.12.2 and add checksum checking for Docker Machine and dumb-init for official Docker images !640
- Fix services usage when service name is using variable !641
- Remove confusing compatibility check !642
- Add sysctl support for Docker executor !541
- Reduce binary size with removing debugging symbols !643
- Add support for credentials store !501
- Fix I am not sure section link !650
- Add tzdata by default to official Docker images to avoid OffPeakPeriods timezone error !649
- Fix read error from upload artifacts execution !645
- Add support for tmpfs on the job container !654
- Include note about volume path on OSX !648
- Start using 'toc' in YAML frontmatter to explicitly disable it !644

## v9.4.3 (2017-10-04)

- Hide tokens from URLs printed in job's trace !708
- Add handling of non-existing images for Docker >= 17.07 !664

## v9.4.2 (2017-08-02)

- Fix usage of one image for multiple services in one job !639
- Fix services usage when service name is using variable !641

## v9.4.1 (2017-07-25)

- Fix allowed_images behavior !635

## v9.4.0 (2017-07-22)

- Use Go 1.8 for CI !620
- Warn on archiving Git directory !591
- Add CacheClient with timeout configuration for cache operations !608
- Remove '.Git/hooks/post-checkout' hooks when using fetch strategy !603
- Fix VirtualBox and Parallels executors registration bugs !589
- Support Kubernetes PVCs !606
- Support cache policies in .GitLab-ci.yml !621
- Improve Kubernetes volumes support !625
- Adds an option `--all` to unregister command !622
- Add the technical description of version release !631
- Update documentation on building Docker images inside of a Kubernetes cluster. !628
- Support for extended Docker configuration in GitLab-ci.yml !596
- Add ServicesTmpfs options to Docker runner configuration. !605
- Fix network timeouts !634

## v9.3.0 (2017-06-22)

- Make GitLab Runner metrics HTTP endpoint default to :9252 !584
- Add handling for Git_CHECKOUT variable to skip checkout !585
- Use HTTP status code constants from net/http library !569
- Remove tls-skip-verify from advanced-configuration.md !590
- Improve Docker machine removal !582
- Add support for Docker '--cpus' option !586
- Add requests backoff mechanism !570
- Fixed doc typo, change `--service-name` to `--service` !592
- Slight fix to build/ path in multi runner documentation !598
- Move docs on private Registry to GitLab docs !597
- Install Git LFS in Helper image for X86_64 !588
- Docker entrypoint: use exec !581
- Create GitLab-runner user on alpine !593
- Move registering Runners info in a separate document !599
- Add basic support for Kubernetes volumes !516
- Add required runners.Docker section to example config. !604
- Add userns support for Docker executor !553
- Fix another regression on Docker-machine credentials usage !610
- Added ref of Docker app installation !612
- Update linux-repository.md !615

## v9.2.2 (2017-07-04)

- Fix VirtualBox and Parallels executors registration bugs !589

## v9.2.1 (2017-06-17)

- Fix regression introduced in the way how `exec` parses `.gitlab-ci.yml` !535
- Fix another regression on Docker-machine credentials usage !610

## v9.2.0 (2017-05-22)

This release introduces a change in the ordering of artifacts and cache restoring!

It may happen that someone, by mistake or by purpose, uses the same path in
`.gitlab-ci.yml` for both cache and artifacts keywords, and this could cause that
a stale cache might inadvertently override artifacts that are used across the
pipeline.

Starting with this release, artifacts are always restored after the cache to ensure
that even in edge cases you can always rely on them.

- Improve Windows runner details !514
- Add support for TLS client authentication !157
- Fix apt-get syntax to install a specific version. !563
- Add link to Using Docker Build CI docs !561
- Document the `coordinator` and make the FAQ list unordered !567
- Add links to additional Kubernetes details !566
- Add '/debug/jobs/list' endpoint that lists all handled jobs !564
- Remove .godir !568
- Add PodLabels field to Kubernetes config structure !558
- Remove the build container after execution has completed !571
- Print proper message when cache upload operation failed !556
- Remove redundant ToC from autoscale docs and add intro paragraph !574
- Make possible to compile Runner under Openbsd2 !511
- Improve Docker configuration docs !576
- Use contexes everywhere !559
- Add support for Kubernetes service account and override on GitLab-ci.YAML !554
- Restore cache before artifacts !577
- Fix link to the LICENSE file. !579

## v9.1.3 (2017-07-04)

- Fix VirtualBox and Parallels executors registration bugs !589

## v9.1.2 (2017-06-17)

- Print proper message when cache upload operation fails !556
- Fix regression introduced in the way how `exec` parses `.gitlab-ci.yml` !535

## v9.1.1 (2017-05-02)

- Fix apt-get syntax to install a specific version. !563
- Remove the build container after execution has completed !571

## v9.1.0 (2017-04-22)

- Don't install docs for the fpm Gem !526
- Mention tagged S3 sources in installation documentation !513
- Extend documentation about accessing Docker services !527
- Replace b.CurrentStage with b.CurrentState where it was misused !530
- Docker provider metrics cleanups and renaming !531
- Replace godep with govendor !505
- Add histogram metrics for Docker machine creation !533
- Fix cache containers dicsovering regression !534
- Add urls to environments created with CI release jobs !537
- Remove unmanaged Docker images sources !538
- Speed up CI pipeline !536
- Add job for checking the internal docs links !542
- Mention Runner -> GitLab compatibility concerns after 9.0 release !544
- Log error if API v4 is not present (GitLab CE/EE is older than 9.0) !528
- Cleanup variables set on GitLab already !523
- Add faq entry describing how to handle missing zoneinfo.zip problem !543
- Add documentation on how Runner uses MinIO library !419
- Update Docker.md - typo in runners documentation link !546
- Add log_level option to config.TOML !524
- Support private registries with Kubernetes !551
- Cleanup Kubernetes typos and wording !550
- Fix runner crashing on builds helper collect !529
- Config docs: Fix syntax in example TOML for Kubernetes !552
- Docker: Allow to configure shared memory size !468
- Return error for cache-extractor command when S3 cache source returns 404 !429
- Add executor stage to ci_runner_builds metric's labels !548
- Don't show image's ID when it's the same as image's name !557
- Extended verify command with runner selector !532
- Changed information line logged by Runner while unregistering !540
- Properly configure connection timeouts and keep-alives !560
- Log fatal error when concurrent is less than 1 !549

## v9.0.4 (2017-05-02)

- Fix apt-get syntax to install a specific version. !563
- Remove the build container after execution has completed !571

## v9.0.3 (2017-04-21)

- Fix runner crashing on builds helper collect !529
- Properly configure connection timeouts and keep-alives !560

## v9.0.2 (2017-04-06)

- Speed up CI pipeline !536

## v9.0.1 (2017-04-05)

- Don't install docs for the fpm Gem !526
- Mention tagged S3 sources in installation documentation !513
- Replace b.CurrentStage with b.CurrentState where it was misused !530
- Replace godep with govendor !505
- Fix cache containers dicsovering regression !534
- Add urls to environments created with CI release jobs !537
- Mention Runner -> GitLab compatibility concerns after 9.0 release !544
- Log error if API v4 is not present (GitLab CE/EE is older than 9.0) !528

## v9.0.0 (2017-03-22)

- Change dependency from `github.com/fsouza/go-dockerclient` to `github.com/docker/docker/client`" !301
- Update Docker-machine version to fix coreos provision !500
- Cleanup windows install docs !497
- Replace io.Copy with stdcopy.StdCopy for Docker output handling !503
- Fixes typo: current to concurrent. !508
- Modifies autoscale algorithm example !509
- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313
- Fix indentation of 'image_pull_secrets' in Kubernetes configuration example !512
- Show Docker image ID in job's log !507
- Fix word consistency in autoscaling docs !519
- Rename the binary on download to use GitLab-runner as command !510
- Improve details around limits !502
- Switch from CI API v1 to API v4 !517
- Make it easier to run tests locally !506
- Kubernetes private credentials !520
- Limit number of concurrent requests to builds/register.JSON !518
- Remove deprecated Kubernetes executor configuration fields !521
- Drop Kubernetes executor 'experimental' notice !525

## v1.11.5 (2017-07-04)

- Fix VirtualBox and Parallels executors registration bugs !589

## v1.11.4 (2017-04-28)

- Fixes test that was failing 1.11.3 release

## v1.11.3 (2017-04-28)

- Add urls to environments created with CI release jobs !537
- Speed up CI pipeline !536
- Fix runner crashing on builds helper collect !529

## v1.11.2 (2017-04-04)

- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313
- Don't install docs for the fpm Gem !526
- Mention tagged S3 sources in installation documentation !513
- Limit number of concurrent requests to builds/register.JSON !518
- Replace b.CurrentStage with b.CurrentState where it was misused !530

## v1.11.1 (2017-03-03)

- Update Docker-machine version to fix coreos provision !500

## v1.11.0 (2017-02-22)

- Fix S3 and packagecloud uploads step in release process !455
- Add Ubuntu/yakkety to packages generation list !458
- Reduce size of GitLab-runner-helper images !456
- Fix crash on machine creation !461
- Rename 'Build (succeeded|failed)' to 'Job (succeeded|failed)' !459
- Fix race in helpers/Prometheus/log_hook.go: Fire() method !463
- Fix missing VERSION on Mac build !465
- Added post_build_script to call scripts after user-defined build scripts !460
- Fix offense reported by vet. Add vet to 'code style' job. !477
- Add the runner name to the first line of log output, after the version !473
- Make CI_DEBUG_TRACE working on Windows CMD !483
- Update packages targets !485
- Update Makefile (fix permissions on /usr/share/GitLab-runner/) !487
- Add timezone support for OffPeak intervals !479
- Set Git_SUBMODULE_STRATEGY=SubmoduleNone when Git_STRATEGY=GitNone !480
- Update maintainers information !489

## v1.10.8 (2017-04-04)

- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313
- Don't install docs for the fpm Gem !526
- Mention tagged S3 sources in installation documentation !513
- Limit number of concurrent requests to builds/register.JSON !518
- Replace b.CurrentStage with b.CurrentState where it was misused !530

## v1.10.7 (2017-03-03)

- Update Docker-machine version to fix coreos provision !500

## v1.10.6 (2017-02-22)

- Update Makefile (fix permissions on /usr/share/GitLab-runner/) !487

## v1.10.5 (2017-02-20)

- Update packages targets !485

## v1.10.4 (2017-01-31)

- Fix race in helpers/Prometheus/log_hook.go: Fire() method !463

## v1.10.3 (2017-01-27)

- Fix crash on machine creation !461

## v1.10.2 (2017-01-26)

- Add Ubuntu/yakkety to packages generation list !458
- Reduce size of GitLab-runner-helper images !456

## v1.10.1 (2017-01-23)

- Fix S3 and packagecloud uploads step in release process !455

## v1.10.0 (2017-01-22)

- Make /usr/share/GitLab-runner/clear-Docker-cache script /bin/sh compatible !427
- Handle Content-Type header with charset information !430
- Don't raise error if machines directory is missing on machines listing !433
- Change digital ocean autoscale to use stable coreos channel !434
- Fix package's scripts permissions !440
- Use -q flag instead of --format. !442
- Kubernetes termination grace period !383
- Check if directory exists before recreating it with Windows CMD !435
- Add '--run-tagged-only' cli option for runners !438
- Add armv6l to the ARM replacements list for Docker executor helper image !446
- Add configuration options for Kubernetss resource requests !391
- Add poll interval and timeout parameters for Kubernetes executor !384
- Add support for Git_SUBMODULE_STRATEGY !443
- Create index file for S3 downloads !452
- Add Prometheus metric that counts number of catched errors !439
- Exclude unused options from AbstractExecutor.Build.Options !445
- Update Docker Machine in official Runner images to v0.9.0 !454
- Pass ImagePullSecrets for Kubernetes executor !449
- Add Namespace overwrite possibility for Kubernetes executor !444

## v1.9.10 (2017-03-23)

- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313

## v1.9.9 (2017-03-03)

- Update Docker-machine version to fix coreos provision !500

## v1.9.8 (2017-02-22)

- Update Makefile (fix permissions on /usr/share/GitLab-runner/) !487

## v1.9.7 (2017-02-20)

- Update packages targets !485

## v1.9.6 (2017-01-25)

- Add Ubuntu/yakkety to packages generation list !458

## v1.9.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.9.4 (2017-01-15)

- Add armv6l to the ARM replacements list for Docker executor helper image !446

## v1.9.3 (2017-01-14)

- Fix package's scripts permissions !440
- Check if directory exists before recreating it with Windows CMD !435

## v1.9.2 (2017-01-04)

- Handle Content-Type header with charset information !430
- Don't raise error if machines directory is missing on machines listing !433

## v1.9.1 (2016-12-24)

- Make /usr/share/GitLab-runner/clear-Docker-cache script /bin/sh compatible !427

## v1.9.0 (2016-12-22)

- Add pprof HTTP endpoints to metrics server !398
- Add a multiple Prometheus metrics: !401
- Split prepare stage to be: prepare, Git_clone, restore_cache, download_artifacts !406
- Update CONTRIBUTING.md to refer to go 1.7.1 !409
- Introduce Docker.Client timeouts !411
- Allow network-sourced variables to specify that they should be files !413
- Add a retry mechanism to prevent failed clones in builds !399
- Remove shallow.lock before fetching !407
- Colorize log entries for cmd and PowerShell !400
- Add section describing Docker usage do Kubernetes executor docs !394
- FreeBSD runner installation docs update !387
- Update prompts for register command !377
- Add volume_driver Docker configuration file option !365
- Fix bug permission denied on ci build with external cache !347
- Fix entrypoint for alpine image !346
- Add windows vm checklist for virtualbox documentation !348
- Clarification around authentication with the Kubernetes executor !296
- Fix Docker hanging for Docker-engine 1.12.4 !415
- Use lib machine to fetch a list of Docker-machines !418
- Cleanup Docker cache clear script !388
- Allow the --limit option to control the number of jobs a single runner will run !369
- Store and send last_update value with API calls against GitLab !410
- Add graceful shutdown documentation !421
- Add Kubernete Node Selector !328
- Push prebuilt images to dockerhub !420
- Add path and share cache settings for S3 cache !423
- Remove unnecessary warning about using image with the same ID as provided !424
- Add a link where one can download the packages directly !292
- Kubernetes executor - use pre-build container !425

## v1.8.8 (2017-02-22)

- Update Makefile (fix permissions on /usr/share/GitLab-runner/) !487

## v1.8.7 (2017-02-20)

- Update packages targets !485

## v1.8.6 (2017-01-25)

- Add Ubuntu/yakkety to packages generation list !458

## v1.8.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.8.4 (2017-01-15)

- Add armv6l to the ARM replacements list for Docker executor helper image !446

## v1.8.3 (2017-01-14)

- Fix package's scripts permissions !440
- Check if directory exists before recreating it with Windows CMD !435

## v1.8.2 (2017-01-04)

- Handle Content-Type header with charset information !430

## v1.8.1 (2016-11-29)

- Rrefactor the private container registry docs !392
- Make pull policies usage clear !393

## v1.8.0 (2016-11-22)

- Fix {Bash,Cmd,Ps}Writer.IfCmd to escape its arguments !364
- Fix path to runners-SSH page !368
- Add initial Prometheus metrics server to runner manager !358
- Add a global index.md for docs !371
- Ensure that all builds are executed on tagged runners !374
- Fix broken documentation links !382
- Bug Fix: use a regex to pull out the service and version in the splitServiceAndVersion method !376
- Add FAQ entry about handling the service logon failure on Windows !385
- Fix "unit tests" random failures !370
- Use correct constant for Kubernetes ressource limits. !367
- Unplug stalled endpoints !390
- Add PullPolicy config option for Kubernetes !335
- Handle received 'failed' build state while patching the trace !366
- Add support for using private Docker registries !386

## v1.7.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.7.4 (2017-01-15)

- Add armv6l to the ARM replacements list for Docker executor helper image !446

## v1.7.3 (2017-01-14)

- Fix package's scripts permissions !440
- Check if directory exists before recreating it with Windows CMD !435

## v1.7.2 (2017-01-04)

- Handle Content-Type header with charset information !430

## v1.7.1 (2016-10-25)

- Fix {Bash,Cmd,Ps}Writer.IfCmd to escape its arguments !364

## v1.7.0 (2016-10-21)

- Improve description of --s3-bucket-location option !325
- Use Go 1.7 !323
- Add changelog entries generation script !322
- Add Docker_images release step to CI pipeline !333
- Refactor shell executor tests !334
- Introduce Git_STRATEGY=none !332
- Introduce a variable to enable shell tracing on bash, cmd.exe and PowerShell.exe !339
- Try to load the InCluster config first, if that fails load kubectl config !327
- Squash the "No TLS connection state" warning !343
- Add a benchmark for helpers.ShellEscape and optimise it !351
- Godep: update GitHub.com/Sirupsen/logrus to v0.10.0 !344
- Use Git clone --no-checkout and Git checkout --force !341
- Change machine.machineDetails to machine.Details !353
- Make runner name lowercase to work with GCE restrictions !297
- Add per job before_script handling for exec command !355
- Add OffPeak support for autoscaling !345
- Prevent caching failures from marking a build as failed !359
- Add missed "server" command for MinIO in autoscaled S3 cache tutorial !361
- Add a section for Godep in CONTRIBUTING.md !302
- Add a link to all install documentation files describing how to obtain a registration token !362
- Improve registration behavior !356
- Add the release process description !176
- Fix documentation typo in docs/configuration/advanced-configuration.md !354
- Fix data races around runner health and build stats !352

## v1.6.1 (2016-09-30)

- Add changelog entries generation script !322
- Add Docker_images release step to CI pipeline !333

## v1.6.0 (2016-09-22)

- Remove an unused method from the Docker executor !280
- Add note about certificate concatenation !278
- Restore 755 mode for GitLab-runner-service script !283
- Remove Git-lfs from Docker helper images !288
- Improve Kubernetes support !277
- docs: update troubleshooting section in development. !286
- Windows installation, added a precision on the install command (issue related #1265) !223
- Autodetect "/ci" in URL !289
- Defer removing failed containers until Cleanup() !281
- fix typo in tls-self-signed.md !294
- Improve CI tests !276
- Generate a BuildError when Docker/Kubernetes image is missing !295
- cmd.exe: Caret-escape parentheses when not inside double quotes !284
- Fixed some spelling/grammar mistakes. !291
- Update Go instructions in README !175
- Add APT pinning configuration for Debian in installation docs !303
- Remove YAML v1 !307
- Add options to runner configuration to specify commands executed before code clone and build !106
- Add RC tag support and fix version discovering !312
- Pass all configured CA certificates to builds !299
- Use Git-init templates (clone) and Git config without --global (fetch) to disable recurseSubmodules !314
- Improve Docker machine logging !234
- Add possibility to specify a list of volumes to inherit from another container !236
- Fix range mismatch handling error while patch tracing !319
- Add Docker+machine and Kubernetes executors to "I'm not sure" part of executors README.md !320
- Remove ./Git/index.lock before fetching !316

## v1.5.3 (2016-09-13)

- Fix Caret-escape parentheses when not inside double quotes for Windows cmd
- Remove LFS from prebuilt images

## v1.5.2 (2016-08-24)

(no changes)

## v1.5.1 (2016-08-24)

- Fix file mode of GitLab-runner-service script !283

## v1.5.0 (2016-08-22)

- Update vendored TOML !258
- Release armel instead arm for Debian packages !264
- Improve concurrency of Docker+machine executor !254
- Use .xz for prebuilt Docker images to reduce binary size and provisioning speed of Docker Engines !249
- Remove vendored test files !271
- Update GitLab-runner-service to return 1 when no Host or PORT is defined !253
- Log caching URL address
- Retry executor preparation to reduce system failures !244
- Fix missing entrypoint script in alpine Dockerfile !248
- Suppress all but the first warning of a given type when extracting a ZIP file !261
- Mount /builds folder to all services when used with Docker Executor !272
- Cache Docker client instances to avoid a file descriptor leak !260
- Support bind mount of `/builds` folder !193

## v1.4.3 (2016-09-13)

- Fix Caret-escape parentheses when not inside double quotes for Windows cmd
- Remove LFS from prebuilt images

## v1.4.2 (2016-08-10)

- Fix abort mechanism when patching trace

## v1.4.1 (2016-07-25)

- Fix panic while artifacts handling errors

## v1.4.0 (2016-07-22)

- Add Sentry support
- Add support for cloning VirtualBox VM snapshots as linked clones
- Add support for `security_opt` Docker configuration parameter in Docker executor
- Add first integration tests for executors
- Add many logging improvements (add more details to some logs, move some logs to Debug level, refactorize logger etc.)
- Make final build trace upload be done before cleanup
- Extend support for caching and artifacts to all executors
- Improve support for Docker Machine
- Improve build aborting
- Refactor common/version
- Use `environment` feature in `.gitlab-ci.yml` to track latest versions for Bleeding Edge and Stable
- Fix Absolute method for absolute path discovering for bash
- Fix zombie issues by using dumb-init instead of GitHub.com/ramr/go-reaper

## v1.3.5 (2016-09-13)

- Fix Caret-escape parentheses when not inside double quotes for Windows cmd

## v1.3.4 (2016-07-25)

- Fix panic while artifacts handling errors

## v1.3.3 (2016-07-15)

- Fix zombie issue by using dumb-init

## v1.3.2 (2016-06-28)

- Fix architecture detection bug introduced in 1.3.1

## v1.3.1 (2016-06-24)

- Detect architecture if not given by Docker Engine (versions before 1.9.0)

## v1.3.0 (2016-06-22)

- Add incremental build trace update
- Add possibility to specify CpusetCpus, Dns and DnsSearch for Docker containers created by runners
- Add a custom `User-Agent` header with version number and runtime information (go version, platform, os)
- Add artifacts expiration handling
- Add artifacts handling for failed builds
- Add customizable `check_interval` to set how often to check GitLab for a new builds
- Add Docker Machine IP address logging
- Make Docker Executor ARM compatible
- Refactor script generation to make it fully on-demand
- Refactor runnsers Acquire method to improve performance
- Fix branch name setting at compile time
- Fix panic when generating log message if provision of node fails
- Fix Docker host logging
- Prevent leaking of goroutines when aborting builds
- Restore valid version info in --help message
- [Experimental] Add `GIT_STRATEGY` handling - clone/fetch strategy configurable per job
- [Experimental] Add `GIT_DEPTH` handling - `--depth` parameter for `git fetch` and `git clone`

## v1.2.0 (2016-05-22)

- Use Go 1.6
- Add `timeout` option for the `exec` command
- Add runtime platform information to debug log
- Add `docker-machine` binary to Runner's official Docker images
- Add `build_current` target to Makefile - to build only a binary for used architecture
- Add support for `after_script`
- Extend version information when using `--version` flag
- Extend artifacts download/upload logs with more response data
- Extend unregister command to accept runner name
- Update shell detection mechanism
- Update the GitHub.com/ayufan/golag-kardianos-service dependency
- Replace ANSI_BOLD_YELLOW with ANSI_YELLOW color for logging
- Reconcile VirtualBox status constants with VBoxManage output values
- Make checkout quiet
- Make variables to work at job level in exec mode
- Remove "user mode" warning when running in a system mode
- Create `gitlab-runner` user as a system account
- Properly create `/etc/gitlab-runner/certs` in Runner's official Docker images
- Disable recursive submodule fetchin on fetching changes
- Fix nil casting issue on Docker client creation
- Fix used build platforms for `gox`
- Fix a limit problems when trying to remove a non-existing machines
- Fix S3 caching issues
- Fix logging messages on artifacts dowloading
- Fix binary panic while using VirtualBox executor with no `vboxmanage` binary available

## v1.1.4 (2016-05-14)

- Create /etc/GitLab-runner/certs
- Exclude architectures from GOX, rather then including
- Update mimio-go to a newest version
- Regression: Implement CancelRequest to fix S3 caching support
- Fix: Skip removal of machine that doesn't exist (autoscaling)

## v1.1.3 (2016-04-14)

- Regression: On Linux use `sh -s /bin/bash user -c` instead of `sh user -c`. This fixes non-login for user.
- Regression: Fix user mode warning
- Fix: vet installation
- Fix: nil casting issue on Docker client creation
- Fix: Docker client download issue

## v1.1.2 (2016-04-06)

- Regression: revert shell detection mechanism and limit it only to Docker

## v1.1.1 (2016-04-06)

- Fix: use different shell detection mechanism
- Regression: support for `gitlab-runner exec`
- Regression: support for login/non-login shell for Bash

## v1.1.0 (2016-03-29)

- Use Go 1.5
- Change license to MIT
- Add Docker-machine based auto-scaling for Docker executor
- Add support for external cache server
- Add support for `sh`, allowing to run builds on images without the `bash`
- Add support for passing the artifacts between stages
- Add `docker-pull-policy`, it removes the `docker-image-ttl`
- Add `docker-network-mode`
- Add `git` to GitLab-runner:alpine
- Add support for `CapAdd`, `CapDrop` and `Devices` by Docker executor
- Add support for passing the name of artifacts archive (`artifacts:name`)
- Add support for running runner as system service on OSX
- Refactor: The build trace is now implemented by `network` module
- Refactor: Remove CGO dependency on Windows
- Fix: Create alternative aliases for Docker services (uses `-`)
- Fix: VirtualBox port race condition
- Fix: Create cache for all builds, including tags
- Fix: Make the shell executor more verbose when the process cannot be started
- Fix: Pass GitLab-ci.yml variables to build container created by Docker executor
- Fix: Don't restore cache if not defined in GitLab-ci.yml
- Fix: Always use `json-file` when starting Docker containers
- Fix: Error level checking for Windows Batch and PowerShell

## v1.0.4 (2016-02-10)

- Fix support for Windows PowerShell

## v1.0.3 (2016-02-08)

- Fix support for Windows Batch
- Remove Git index lock file: this solves problem with Git checkout being terminated
- Hijack Docker.Client to use keep-alives and to close extra connections

## v1.0.2 (2016-01-27)

- Fix bad warning about not found untracked files
- Don't print error about existing file when restoring the cache
- When creating ZIP archive always use forward-slashes and don't permit encoding absolute paths
- Prefer to use `path` instead of `filepath` which is platform specific: solves the Docker executor on Windows

## v1.0.1 (2016-01-24)

- Use nice log formatting for command line tools
- Don't ask for services during registration (we prefer the .GitLab-ci.yml)
- Create all directories when extracting the file

## v1.0.0 (2016-01-22)

- Add `gitlab-runner exec` command to easy running builds
- Add `gitlab-runner status` command to easy check the status of the service
- Add `gitlab-runner list` command to list all runners from config file
- Allow to specify `ImageTTL` for configuration the frequency of Docker image re-pulling (see advanced-configuration)
- Inject TLS certificate chain for `git clone` in build container, the GitLab-runner SSL certificates are used
- Remove TLSSkipVerify since this is unsafe option
- Add go-reaper to make GitLab-runner to act as init 1 process fixing zombie issue when running Docker container
- Create and send artifacts as zip files
- Add internal commands for creating and extracting archives without the system dependencies
- Add internal command for uploading artifacts without the system dependencies
- Use umask in Docker build containers to fix running jobs as specific user
- Fix problem with `cache` paths never being archived
- Add support for [`cache:key`](https://docs.gitlab.com/ci/yaml/#cachekey)
- Add warnings about using runner in `user-mode`
- Push packages to all upcoming distributions (Debian/Ubuntu/Fedora)
- Rewrite the shell support adding all features to all shells (makes possible to use artifacts and caching on Windows)
- Complain about missing caching and artifacts on some executors
- Added VirtualBox executor
- Embed prebuilt Docker build images in runner binary and load them if needed
- Make possible to cache absolute paths (unsafe on shell executor)

## v0.7.2 (2015-11-25)

- Adjust `umask` for build image
- Use absolute path when executing archive command
- Fix regression when variables were not passed to service container
- Fix duplicate files in cache or artifacts archive

## v0.7.1 (2015-11-22)

- Fix caching support
- Suppress tar verbose output

## v0.7.0 (2015-11-21)

- Refactor code structure
- Refactor bash script adding pre-build and post-build steps
- Add support for build artifacts
- Add support for caching build directories
- Add command to generate archive with cached folders or artifacts
- Use separate containers to run pre-build (Git cloning), build (user scripts) and post-build (uploading artifacts)
- Expand variables, allowing to use $CI_BUILD_TAG in image names, or in other variables
- Make shell executor to use absolute path for project dir
- Be strict about code formatting
- Move network related code to separate package
- Automatically load TLS certificates stored in /etc/GitLab-runner/certs/<hostname>.crt
- Allow to specify tls-ca-file during registration
- Allow to disable tls verification during registration

## v0.6.2 (2015-10-22)

- Fix PowerShell support
- Make more descriptive pulling message
- Add version check to Makefile

## v0.6.1 (2015-10-21)

- Revert: Fix tags handling when using Git fetch: fetch all tags and prune the old ones

## v0.6.0 (2015-10-09)

- Fetch Docker auth from ~/.Docker/config.JSON or ~/.dockercfg
- Added support for NTFSSecurity PowerShell module to address problems with long paths on Windows
- Make the service startup more readable in case of failure: print a nice warning message
- Command line interface for register and run-single accepts all possible config parameters now
- Ask about tags and fix prompt to point to GitLab.com/ci
- Pin to specific Docker API version
- Fix Docker volume removal issue
- Add :latest to imageName if missing
- Pull Docker images every minute
- Added support for SIGQUIT to allow to gracefully finish runner: runner will not accept new jobs, will stop once all current jobs are finished.
- Implicitly allow images added as services
- Evaluate script command in subcontext, making it to close stdin (this change since 0.5.x where the separate file was created)
- Pass container labels to Docker
- Force to use go:1.4 for building packages
- Fix tags handling when using Git fetch: fetch all tags and prune the old ones
- Remove Docker socket from GitLab/GitLab-runner images
- Pull (update) images and services every minute
- Ignore options from Coordinator that are null
- Provide FreeBSD binary
- Use -ldflags for versioning
- Update go packages
- Fix segfault on service checker container
- WARNING: By default allow to override image and services

## v0.5.5 (2015-08-26)

- Fix cache_dir handling

## v0.5.4 (2015-08-26)

- Update go-dockerclient to fix problems with creating Docker containers

## v0.5.3 (2015-08-21)

- Pin to specific Docker API version
- Fix Docker volume removal issue

## v0.5.2 (2015-07-31)

- Fixed CentOS6 service script
- Fixed documentation
- Added development documentation
- Log service messages always to syslog

## v0.5.1 (2015-07-22)

- Update link for Docker configuration

## v0.5.0 (2015-07-21)

- Allow to override image and services for Docker executor from Coordinator
- Added support for additional options passed from coordinator
- Added support for receiving and defining allowed images and services from the Coordinator
- Rename GitLab_ci_multi_runner to GitLab-runner
- Don't require config file to exist in order to run runner
- Change where config file is stored: /etc/GitLab-runner/config.TOML (*nix, root), ~/.GitLab-runner/config.TOML (*nix, user)
- Create config on service install
- Require root to control service on Linux
- Require to specify user when installing service
- Run service as root, but impersonate as --user when executing shell scripts
- Migrate config.TOML from user directory to /etc/GitLab-runner/
- Simplify service installation and upgrade
- Add --provides and --replaces to package builder
- PowerShell: check exit code in writeCommandChecked
- Added installation tests
- Add runner alpine-based image
- Send executor features with RunnerInfo
- Verbose mode by using `echo` instead of `set -v`
- Colorize bash output
- Set environment variables from bash script: this fixes problem with su
- Don't cache Dockerfile VOLUMEs
- Pass (public) environment variables received from Coordinator to service containers

## v0.4.2

- Force GC cycle after processing build
- Use log-level set to info, but also make `Checking for builds: nothing` being print as debug
- Fix memory leak - don't track references to builds

## v0.4.1

- Fixed service reregistration for RedHat systems

## v0.4.0

- Added CI=true and GitLab_CI=true to environment variables
- Added output_limit (in kilobytes) to runner config which allows to enlarge default build log size
- Added support for custom variables received from CI
- Added support for SSH identity file
- Optimize build path to make it shorter, more readable and allowing to fix shebang issue
- Make the debug log human readable
- Make default build log limit set to 4096 (4MB)
- Make default concurrent set to 1
- Make default limit for runner set to 1 during registration
- Updated kardianos service to fix OSX service installation
- Updated logrus to make console output readable on Windows
- Change default log level to warning
- Make selection of forward or back slashes dependent by shell not by system
- Prevent runner to be stealth if we reach the MaxTraceOutputSize
- Fixed Windows Batch script when builds are located on different drive
- Fixed Windows runner
- Fixed installation scripts path
- Fixed wrong architecture for i386 Debian packages
- Fixed problem allowing commands to consume build script making the build to succeed even if not all commands were executed

## v0.3.4 (2015-06-15)

- Create path before clone to fix Windows issue
- Added CI=true and GitLab_CI=true
- Fixed wrong architecture for i386 Debian packages

## v0.3.3 (2015-05-11)

- Push package to Ubuntu/vivid and ol/6 and ol/7

## v0.3.2 (2015-05-03)

- Fixed Windows batch script generator

## v0.3.1 (2015-05-03)

- Remove clean_environment (it was working only for shell scripts)
- Run bash with --login (fixes missing .profile environment)

## v0.3.0 (2015-05-03)

- Added repo slug to build path
- Build path includes repository hostname
- Support TLS connection with Docker
- Default concurrent limit is set to number of CPUs
- Make most of the config options optional
- Rename setup/delete to register/unregister
- Checkout as detached HEAD (fixes compatibility with older Git versions)
- Update documentation

## v0.2.0 (2015-04-23)

- Added delete and verify commands
- Limit build trace size (1MB currently)
- Validate build log to contain only valid UTF-8 sequences
- Store build log in memory
- Integrate with ci.GitLab.com
- Make packages for ARM and CentOS 6 and provide beta version
- Store Docker cache in separate containers
- Support host-based volumes for Docker executor
- Don't send build trace if nothing changed
- Refactor build class

## v0.1.17 (2015-04-15)

- Fixed high file descriptor usage that could lead to error: too many open files

## v0.1.16 (2015-04-13)

- Fixed systemd service script

## v0.1.15 (2015-04-11)

- Fix order of executor commands
- Fixed service creation options
- Fixed service installation on OSX

## v0.1.14 (2015-04-07)

- Use custom kardianos/service with enhanced service scripts
- Remove all system specific packages and use universal for package manager

## v0.1.13 (2015-04-01)

- Added abstraction over shells
- Moved all bash specific stuff to shells/bash.go
- Select default shell for OS (bash for Unix, batch for Windows)
- Added Windows Cmd support
- Added Windows PowerShell support
- Added the kardianos/service which allows to easily run GitLab-ci-multi-runner as service on different platforms
- Unregister Parallels VMs which are invalid
- Delete Parallels VM if it doesn't contain snapshots
- Fixed concurrency issue when assigning unique names

## v0.1.12 (2015-03-20)

- Abort all jobs if interrupt or SIGTERM is received
- Runner now handles HUP and reloads config on-demand
- Refactored runner setup allowing to non-interactive configuration of all questioned parameters
- Added CI_PROJECT_DIR environment variable
- Make golint happy (in most cases)

## v0.1.11 (2015-03-11)

- Package as .deb and .rpm and push it to packagecloud.io (for now)

## v0.1.10 (2015-03-11)

- Wait for Docker service to come up (Loïc Guitaut)
- Send build log as early as possible

## v0.1.9 (2015-03-10)

- Fixed problem with resetting Ruby environment

## v0.1.8 (2015-03-10)

- Allow to use prefixed services
- Allow to run on Heroku
- Inherit environment variables by default for shell scripts
- Mute Git messages during checkout
- Remove some unused internal messages from build log

## v0.1.7 (2015-02-19)

- Fixed Git checkout

## v0.1.6 (2015-02-17)

- Remove Docker containers before starting job

## v0.1.5 (2015-02-14)

- Added Parallels executor which can use snapshots for fast revert (only OSX supported)
- Refactored sources

## v0.1.4 (2015-02-01)

- Remove Job and merge it into Build
- Introduce simple API server
- Ask for services during setup

## v0.1.3 (2015-01-29)

- Optimize setup
- Optimize multi-runner setup - making it more concurrent
- Send description instead of hostname during registration
- Don't ask for tags

## v0.1.2 (2015-01-27)

- Make it work on Windows

## v0.1.1 (2015-01-27)

- Added Docker services

## v0.1.0 (2015-01-27)

- Initial public release
