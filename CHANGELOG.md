## v15.8.0 (2023-01-19)

### New features

- Add system_id to Prometheus metrics !3825
- Send system_id in jobs requests !3817
- Prepare register command to fail if runner server-side configuration options are passed together with a new glrt- token !3805
- Add nesting client to support VM-isolated build environments !3654
- #27863 Add mac address with isolation !3454 (Artem Makhno @artem.makhno.softsmile)
- Display system_id on build log !3852

### Bug fixes

- Fix doublestar implementation to use paths relative to working directory !3849
- Fix windows integration tests failure check !3846
- Re-merge "Artifact/cache helpers now use POSIX shell syntax for expansion" !3833
- powershell: fix unwanted progress streams leaking to output !3831
- Fix skipped windows integration tests !3830
- Fix relative URL path handling with clone_url !3815
- Prevent new autoscaler thrashing instances !3813
- Add a check for any artifact paths that do not fall within the project directory or its subpaths !3757
- Use exec mode to create the scripts in attach mode !3751
- powershell: Fix stdin handling with scripts !3843

### Maintenance

- Revert "Fix go.mod to downgrade doublestar to v1.3.0 to be same as main" !3842
- Add pwsh to supported shells for docker-windows executor !3829
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
- Clarify that gitlab-runner is required for both download and upload !3834 (Dillon Amburgey @dillon4)
- Clarify variable type !3824
- Docs surround KUBERNETES_ values with quotes !3820
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

- powershell: Fix stdin handling with scripts !3843

## v15.7.2 (2023-01-13)

### Bug fixes

- Fix relative URL path handling with clone_url !3815
- powershell: fix unwanted progress streams leaking to output !3831
- Re-merge "Artifact/cache helpers now use POSIX shell syntax for expansion" !3833

## v15.7.1 (2022-12-19)

### Bug fixes

- Revert automate for which supported distro releases we create packages. !3794

## v15.7.0 (2022-12-17)

### New features

- Add PrivilegedServices option for allowing/disallowing docker services to be privileged !2652
- Add support for Windows Server 21H2 !3746
- Generate global system ID !3758
- Add start_type to virtualbox configuration !2558 
- Update secret resolver to return raw & masked variables !3750
- Allow Executors to clone via SSH !3518 
- Add docker support for `IpcMode` for IPC namespace sharing !3781 
- Expose the build timeout as an environment variable !3778
- Improve Runner's API health checking and handling !3658

## v15.6.3 (2023-01-19)

### Bug fixes

- powershell: Fix stdin handling with scripts !3843

## v15.6.2 (2023-01-13)

### Bug fixes

- powershell: fix unwanted progress streams leaking to output !3831

## v15.6.1 (2022-11-24)

### Bug fixes

- Fix cache config needing to be provided !3747
- Add gitlab-runner user during ubi-fips image building !3725
- Fix kubernetes pod labels overwrite !3582
- Correctly handle expansion of job file variables, and variables that reference file variables !3613
- Artifact/cache helpers now use POSIX shell syntax for expansion !3752

### Maintenance

- Upgrade github.com/urfave/cli to 1.22.10 !3744
- Unit test to catch urfave bug !3749
- Makefile.build.mk: allow building for arm64 without overriding ARCH !3498
- Renovate Go version !3768
- Add warning about using SIGTERM/SIGINT over SIGQUIT !3769
- Update golang Docker tag to v1.18.9 !3776
- Automate for which supported distro releases we create packages. !3756
- Fix silent docker images build failure and retry buildx !3786
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
- Allow multiple paths in GIT_SUBMODULE_PATHS !3675
- Capture helper service logs into job/tasks main trace log !3680
- Add a feature flag to disable resolving of TLS chain !3699
- Adds proper handling of ExecutorProviders initialization and shutdown !3657

### Bug fixes

- Detect Windows build 10.0.19042 as 20H2 !3694
- Force powershell/pwsh input/output encoding to UTF-8 !3707
- Skip non-regular files for artifact metadata generator inclusion !3709
- Filter kubernetes trace to remove newline added for long logs in attach mode !3691
- Enable powershell via stdin by default !3728
- Kubernetes executor: redial backend on internal server errors !3732

### Maintenance

- Update redhat/ubi8 Docker tag to v8.7-929 !3738
- Add OS versions supported by packagecloud 3.0.6 release !3734
- Add tests for kubernetes scheduler name config !3643
- Update Go distribution to version 1.18.8 !3720
- Update logging levels from Debug to Info !3710
- Move autoscaler Acquire() to the ExecutorProvider !3660
- Document internal Executor Interface !3291
- Update git to 2.38.1 and git-lfs to 3.2.0 to address CVE-2022-29187 !3674
- Switch to markdownlint-cli2 !3683 
- Ensure `go-fips` container is rebuilt when the version of Go is updated !3685
- Add logging in UpdateJob to include checksum and bytesize !3693
- Update taskscaler to newer version !3706
- Skip docker Test_CaptureServiceLogs integration tests on windows !3703
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
- Add helm repo update command to Kubernetes install docs !3736 
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
- Adds deprecation notes for docker-ssh and docker-ssh+machine executors !3714
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
- Add GIT_SUBMODULE_DEPTH variable !3651
- Add support for PAT masking in trace !3639

### Bug fixes

- Set all existing variables into the build container !3607
- Add pgrep to ubi-fips image !3625
- Standardize Attestation Artifact Names and Permissions !3650
- Do not expand some CMD variables https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38

### Maintenance

- Upgrade Go to version 1.18.6 !3589
- Add TMPDIR to test's env allowlist !3603
- Go 1.18 mod tidy !3619
- Drop runtime.GC() after every check !3595 
- Upgrade Go FIPS image version to 1.18 !3624
- Add internal autoscaler executor provider unit tests !3633
- Only generate mocks that are actually used in tests !3630
- Fix incorrect spelling of acquisition !3621
- Add User config setting for docker executor !2913
- Upgrade Go FIPS image version to 1.18.7 !3640
- Upgrade Go distribution to version 1.18.7 !3656

### Documentation changes

- Added GitLab Runner to title !3618
- Clarify k8s executor overrides per CI/CD job !3626
- Add note about docker-in-docker !3628
- Fix indentation for [runners.cache] in kubernetes docs !3634
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

- Do not expand variables in Command https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38

## v15.4.0 (2022-09-21)

### New features

- Add renovate support !3592

### Bug fixes

- Reset token in config template when set !3593
- Remove reliance on text/transform for trace masking !3482

### Maintenance

- Update instructions with new menu title !3599
- Update project for latest Vale and Markdownlint tooling and rules !3598
- Docs: Small edit to language !3596
- Updated title to match left nav !3588
- Delete tmp/gitlab-test directory. !3585
- Updated title to match our standards !3584
- Allow setting of Docker volume label mode independent of read/write mode !3580
- Improve clarity of runner metrics examples !3578
- Remove 'respectively' and 'please note' !3574
- Add io error to troubleshooting section !3573
- Docs: Adding details about GitOps configuration for agent !3572
- Fix runners location in docs !3555
- Add path implementation to support Windows docker from unix !3344
- Update redhat/ubi8 Docker tag to v8.6-943 !3605
- Update alpine Docker tags !3604

### Security fixes

- Upgrade prometheus/client-golang from v1.1.0 to v1.11.1

## v15.3.3 (2022-11-11)

### New features

- Add a feature flag to disable resolving of TLS chain !3699

## v15.3.2 (2022-09-21)

### Security fixes

- Do not expand variables in Command https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/38

## v15.3.1 (2022-09-21)

### Security fixes

- Upgrade prometheus/client-golang from v1.1.0 to v1.11.1

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
- Revert github.com/urfave/cli back to v1.20.0 !3539
- Improve error message when there's a conflict between `pull_policy` and `allowed_pull_policies` settings !3526
- Sanitize user-provided custom build directory before passing it forward !3360

### Maintenance

- Docs: Remove old install page !3563
- Update default label for documentation MR template !3559
- Promote gitlab.MultiLineLinks to error !3554 (Niklas @Taucher2003)
- Fix links split across multiple lines in Runner repo !3553
- Add note on GitLab instance pre-requisite for using Runners - docs !3549
- Update Markdownlint and Vale configuration !3548
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

- Update github.com/containerd/containerd dependency !3525
- Rename DEBUG env var to RUNNER_DEBUG !3497

### Maintenance

- Push image on registry during release stage only when enabled !3528
- Fix version history formatting !3523
- Upgrade Go to 1.17.9 in project !3515
- Disable push to ECR in all cases !3514
- Make resource checking disabled by default !3513
- Fix DEB_PLATFORMS definition in the Makefile !3510
- Monitor docker-machine provision failed state !3355 (StoneMan @Wenyuyang)
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
- Remove columns to correct rendering config.toml, CLI options and ENV variable for the register table !3508
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
- Add note about gitlab-runner-fips !3487
- Update minio-go dependency to fix FIPS endpoints !3484
- The context of the language would suggest the plural form of this noun. !3483
- Fixed a broken link for FIPS RHEL runner !3481 (Brock R @fearthebadger)
- Clarify on Docker engine version requirement !3479
- Expand variables for Pod volume subPath and mountpath config !3478
- Update documentation on interactive web terminal support for helm chart !3477
- Add upgrade code sample for arm64 !3475
- Fix error in oc create configmap command - docs !3471
- Remove windows server 2004/20H2/21H1 related tests from community MR pipeline !3467
- Do not retry artifact download on 401 response !3461
- Modify doc mentions of RedHat to Red Hat !3459 (lousyd @lousyd)
- Update project to use latest linting images from gitlab-docs !3452
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
- Implement allowed_pull_policies in config.toml !3422
- Implement supporting pull_policy from jobs !3412
- Allow to overwrite Pod labels in the Kubernetes executor !3352 (Mathieu Parent @sathieu)
- Add a flag to `gitlab-runner exec` to specify the CI/CD config file !3246 (Alexis Jeandeau @jeandeaual)
- Use GCP metadata server and sign blob API for GCS cache url !3231 (Jasper Maes @jlemaes)
- Complete the example configuration for gcp cache !2956 (Edward Smit @edwardsmit)
- Support Priority Class Name for Kubernetes executor !2685 (ayoub mrini @ayoubmrini424)

## v15.0.0 (2022-05-19)

### Security fixes

- Improve sensitive URL parameter masking !3404

### Bug fixes

- Allow S3 cache's AuthenticationType to be provided case-insensitively !3446

### Maintenance

- Update git-lfs to 2.13.3 !3458
- Add TestMachineIdleLimits in the windows 21h1 test failure !3457
- Repair redirected links !3456
- Add history to docs for kubernetes pull policy !3455 (Raimund Hook @stingrayza)
- Run bleeding edge windows builds for security pipelines as well !3449
- Fix minor grammatical error. !3448 (Crafton Williams @crafton)
- Fix windows 21H1 pushing helper images and integration tests !3447
- Delete trailing whitespace !3443
- Fix alpine-latest pipelines for pwsh and prevent this happening on main in the future !3442
- Moved content to executor pages !3440
- Add instructions for how to specify what user a job is run as via docker executor !3438
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
- Fix indentation for docker run runner example !3419
- Register runner with renamed paused argument !3414
- Enable CGO_ENABLED by default in golang-fips compiler !3413
- Change amazonec2-security-group to XXXX in example !3411
- Check serviceaccount and imagepullsecret availability before creating pod !3399
- Make clear-docker-cache script to work for Docker versions below 17.06.1 !3394 (Roland Hügli @rhuegli)
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

- add tip for windows docker permissions !3397
- Add newline between the command and the output when collapsible section is enabled !3389 (Thomas Chandelle @tchandelle)
- Increase token short length if it includes prefix !3373

### Maintenance

- Update lint-markdown image for docs !3408
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
- Add support for docker client version negotiation !3322
- docs: update region specific s3 endpoint urls !2975 (Casey Vockrodt @casey.vockrodt)
- Add archiver staging directory option to runner helper !3403

### GitLab Runner distribution

- Add amazon/2 RPM distribution to the release list !3378

### Documentation changes

- Add Kubernetes operator installation and uninstallation docs and updated OpenShift docs !3388
- Add runner registration related properties to OpenShift Operator !3386 (Em Karisch @QuingKhaos)
- Support docker container custom labels !3304 (aylinsenadogan @aylinsenadogan)
- Update release process link in readme !3319 (Théo DELCEY @theodelcey)

## v14.9.0 (2022-03-21)

### New features

- Add posix shell quoting implementation !3367

### Bug fixes

- Use token from job payload when composing repository URL based on clone_url !3366
- Upgrade minio to v7.0.24 pre-release, for IAM timeout fix !3354
- Upgrade fastzip to v0.1.9, fixes invalid timestamps !3353
- Update network responses to support 64-bit Job IDs !3346
- Upgrade fastzip to v0.1.8 !3333
- Allow changing shell executor with pwsh user !3298
- Remove bashisms from Bash shell implementation !3014 (Neil Roza @realtime-neil)

### Maintenance

- Update stringData for Custom TLS cert !3372
- Add default issue & MR templates !3368
- Docs: Added fleet management link !3364
- Add link to AWS driver docs in gitlab docker machine - docs !3363
- Change fleet scaling to best practices for runner shared services - docs !3362
- Docs: kubernetes volumes are mounted on services !3361 (Quentin Barbe @forty1)
- Add warning about enabling debug logging !3359
- Add links to clarify AWS and docker credentials requirements and clarification on image: tag !3358
- Add link to docker machine fork - docs !3357
- Edited for style !3351
- Run trigger-UBI-images-build job also for patch release tags !3350
- Update runner registration failure log message !3349
- Add runner registration message section - docs !3348
- Move Path interface to docker volume consumer !3343
- Neaten helpers/path unix path impl !3342
- Fix misleading error during cache restoration !3340
- Clean up docs redirects - 2022-02-22 !3339
- Make ssh command/executor shell agnostic !3337
- Remove redundant shell config environment property !3336
- Updated agent for Kubernetes !3334
- Update CI toolchain versions !3330
- Upgrade Docker to 20.10.12 !3328
- Support Vault EE namespaces !3320 (Aleksander Zak @aleksanderzak)
- Add Debian bullseye to supported versions !3318
- Add post_clone_script hook !3211 (Dan Rice @dnrce)
- Docs: Update kubernetes key file format !3097 (Brandon Hee @brandonhee)
- fix grammatical error !2896 (James Dube @jamesdube)

### GitLab Runner distribution

- Fixes version definition in VERSION file !3371
- Align debian releases for stable and Bleeding Edge versions !3335

### Documentation changes

- Add support for Kubernetes runtime class !2326
- Add docs about security risks for using cache and the git_strategy=fetch !3365

## v14.8.0 (2022-02-20)

### New features

- Allow specifying maintenance-note on runner registration !3268
- Support Apple Silicon (darwin/arm64) !2274
- Add variable support for services (Stefano Tenuta @ST-Apps1) !3158

### Bug fixes

- Fix artifacts upload redirection support !3308
- Handle redirects on artifact uploads !3303
- Introduce non-reusable docker cache volumes !3269
- Merge the config template before asking the user for configuration !2561 (Matthias Baur @m.baur)
- Make use of build requests/limits for build permission init container !3321

### Maintenance

- Add details to docs on CI_SERVER_TLS_CA_FILE !3332 (Ben Bodenmiller @bbodenmiller)
- Ensure shell writers terminate with newline flush !3329
- Upgrade Go to 1.17.7 !3327
- Install supported Go version for Windows prior to testing !3324
- Upgrade minio to v7.0.21 !3323
- Fix milestone ship date error for the idlescalefactor feature - docs !3317
- Remove vendor/ directory !3314
- Divide packages buildling jobs in the pipeline even more !3313
- Use latest docs linting image for Markdown !3312
- Docs: Update shell descriptions to use full names !3310 (Neil Roza @realtime-neil)
- Bump version of Go for project to 1.17.6 !3305
- Fix Azure caching example config !3300 (Stefan Asseg @stefanasseg)
- Encourage use of K8s secrets !3299 (Christian Mäder @nxt.cma)
- Update interactive example that was incorrectly set to non-interactive !3297 (Arran Walker @ajwalker)
- Update support for session_server using helm chart !3296
- Cleanup cache proxy pattern !3294
- Adds details about how to limit the number of VMs when autoscaling !3289
- Update linting configuration from GitLab project !3288
- Replace ruby:2.6 in examples and test cases with ruby:2.7 !3287
- Update runner security docs !3279
- Update Page with more common -machine-machine-options for use with docker and amazon ec2 instances. !3259
- Add information on how to connect to S3 from Runners on Amazon EKS with IAM Role for ServiceAccount !3251
- Add version number to windows helper image tags !3217 (Florian Greinacher @fgreinacher)
- Update docs/executors/shell.md !3208
- To disable wait_for_services_timeout use -1 not 0 !3207
- Add support for extra submodule update flags !3192 (Nejc Habjan @nejc)
- Clarify that listed limitations are specific to Windows !3155
- Ensure proper assumptions !3038 (Deniz Adrian @zined)
- Update the security caveats about the usage of privileged mode !2482
- Add debian/bullseye to packagecloud DEB_PLATFORMS !2940 (James Addison @jayaddison-collabora)

### Documentation changes

- Add details on concurrent parameter for docker executor - docs !3286
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
- Restore git 1.8.3.1 tests !3278
- Fix tests using gitlab-grack submodule !3272
- Clarify how to configure network mode with docker executor !3264
- Update golangci-lint !3261
- Pass UPSTREAM_CI_COMMIT_REF to incept tests !3257
- Update sentry library from raven-go to sentry-go !3199 (Markus Legner @mlegner)
- Bump used Go version to 1.17 !3112
- Show error details for failed artifact uploads !3240

### GitLab Runner distribution

- Fix the 'stable gitlab release' job !3252

### Documentation changes

- Point to Gitlab maintained fork of Docker Machine !3276 (Thameez Bodhanya @thameezbo)
- Release of a FIPS Compliant runner !3274
- Adds note about 5 GB S3 cache limit !3266
- Added troubleshooting steps !3273
- Fix broken external links !3270 (Niklas @Taucher2003)
- Update to mention centos stream 8 instead of centos linux 8 !3267 (Ondřej Budai @ondrejbudai)
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
- Add note about running docker runner with docker-machine functionality !3236 (Ihor Martyniuk @enoot)
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
- Add idle gitlab_runner_jobs metric per runner !3202
- Fix links to shared runners documentation !3201
- Add openssl command to download the cert !3200
- Improve Runner container image size for ubuntu and alpine !3185 (Furkan Türkal @Dentrax)
- Autoscale VMs based on a percentage of in-use VMs !3179
- Use native go errors and drop pkg/errors !3104 (feistel   @feistel)
- Fix the 'stable gitlab release' job !3252

### GitLab Runner distribution

- Push stable images built on security fork to canonical repository !3242
- Update the GitLab Release job !3228

### Documentation changes

- Update lint-html image for docs !3239
- Docs: Added OpenSSL SSL_connect: SSL_ERROR_SYSCALL troubleshooting topic !3229
- Docs: Add pod cleanup info in the Kubernetes doc !3224
- Update docs for installing runner from binary !3222 (Wojciech Pater @wpater)
- Changed symbol in docs table !3220
- Add Native Windows OpenSSH Server and Powershell support for Virtualbox and Parallels executors !3176 (Guillaume Chauvel @guillaume.chauvel)

## v14.5.2 (2021-12-10)

### Security fixes

- Fix `syscall.forkExec` calling `close(fd=0)` on pipe error [!44](https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/44)

## v14.5.1 (2021-12-01)

### Security fixes

- Limit docker executor's container reads to prevent memory exhaustion [!37](https://gitlab.com/gitlab-org/security/gitlab-runner/-/merge_requests/37)

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
- Fix gitlab grack to use our own repositories !3187
- Use newer docs linting image !3186
- Update changelog generator configuration !3183
- Fix docker pulling image integration test !3182
- Break out shell blocks to allow copy from button !3181
- Add troubleshooting info to Runner installation with Agent !3180
- Log errors when failing to close docker client !3178
- gitlab-runner Dockerfile: clear /tmp of ubuntu docker image !3177 (Yalcin Ozhabes @trim_the_main)
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
- Suppress git hints about branch naming standards !3148
- Update golang-cli-helpers library, support env namespaces !3147
- Handle situations when neither `ntpdate` nor `sntp` is available !3143 (Alexander Kutelev @kutelev)
- Docs: Small edits to enhance readability !3137 (Ankita Singh @ankita.singh.200020)
- Better support for Powershell on Kubernetes !3119
- Do not pass in bash detection script into build container !3095
- Kubernetes executor should only used SharedBuildsDir behaviour when it is required !3078 (David Alger @davidalger)
- [DOCS] - Improve image pull secrets documentation clarity !3047 (Aaron Johnson @acjohnson1985)
- Document how to run jobs as non-root user for kubernetes and kubernetes with helm !2900
- Allow finer-grained control over pod grace periods. !2130 (Dominic Bevacqua @dbevacqua)

### GitLab Runner distribution

- Provide docker images for alpine 3.12 (default), 3.13 and 3.14. !3122

## v14.4.0 (2021-10-25)

### Security fixes

- Sanitize git folder after each build !3134

### Bug fixes

- Add Kubernetes pod label sanitization !3054 (Theodor van Nahl @t.vannahl)

### Maintenance

- Revert "Merge branch 'alpine-3-13' into 'main'" !3157
- Consider all docker pull image system error as runner script failure !3142
- Docker Executor: use Stop for graceful shutdown !3128 (Aaron Friel @frieltwochairs)
- Update to minio-go v7.0.13 !3120 (Philip Schwartz @pschwar1)
- Explicit configuration for cache s3 authentication type !3117
- refactor: remove osext dependency !3101 (feistel @feistel)
- Respect Docker Runtime setting for services !3063 (Jakob-Niklas See @networkException)

### GitLab Runner distribution

- Split packagecloud release by distribution flavor !3146

### Documentation changes

- Mark URLs compatible with markdownlint-cli 0.29.0 !3154
- Remove Fedora 34 from list of packages - docs !3151
- Fixed docs crosslink from Advanced Config !3149 (Raimund Hook @stingrayza)
- Update Autoscale config due to docker machine deprecation docs !3144
- Compatibility chart update !3141 (Alexander Kutelev @kutelev)
- Update docker_machine.md - docs !3140
- Change description for gitlab_runner_limit !3139
- docs: Delete link to gitlab.com-config (target does not exist) !3138 (Stefan Schmalzhaf @the_s)
- Fix yaml indention of GCS secretName !3136 (Kate @kate_stack11)
- Replace incorrect terminology (storage -> bandwidth) !3135 (Jay Williams @codingJWilliams)
- Docs: Updated Microsoft Service policy links !3133
- Runner: fix some broken external links !3127
- Additional step when install GitLab Runner using KAS !3126
- Added info about code handling for Windows runner !1975

## v14.3.0 (2021-09-21)

### New features

- Cleanup build directory with feature flag !3065
- Consider only docker image pull system error as runner-system-failure !3060

### Security fixes

- Restrict accepted metric listener HTTP methods !3109

### Bug fixes

- Fix docker-machine executor check to reduce warning log spam for no runners able to process a job !3106 (Thomas Scully @tscully49)

### Maintenance

- Turn on FF_SCRIPT_SECTIONS for GitLab Runner pipelines !3124
- Expose runner limit error message on registration !3108
- Split linux packages into multiple jobs !3105
- Upgrade minio to v7 !3099
- Update runner docs tests !3096
- Remove docker-machine feature flag !3093
- Improve log line decoding for kubernetes !3091
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
- Reword so that docker services list "images" instead of "applications" !3094
- Adds Linux register command for behind a proxy !3087 (Rui Duarte @P0w3rb0y)
- Add info for DOCKER_HOST value in Using docker:dind !3085
- Added docker image build steps for Alpine !3076
- Add doc in FAQ about running Elasticsearch !3110
- Fix typo in security process !3092

## v14.2.0 (2021-08-22)

### Bug fixes

- Do not execute checks for windows integration test in docs only default branch pipeline !3070
- Limit job log to ensure it contains UTF-8 valid data !3037
- Fix ubuntu helper image builds to use correct platform (not always amd64) !3032 (Sneha Kanekar @skanekar1)
- Fix trace short writes when large masks are configured !2979
- Fix cleaning of removed sub-submodules when using fetch strategy !2883 (DmtiryK @dkozlov)

### Maintenance

- Update trace force send interval to be dynamically adjusted based on update interval !3064
- Update rules for windows tests to fix docs pipeline !3062
- wrap each line in a script block as a section !3051
- Add new histogram metrics to docker+machine executor !3050
- Do not ignore failure in Windows jobs due to timeout !3042
- Fix release job to use JOB-TOKEN !3041
- Support of kubernetes lifecycle hooks !3036
- Add all of gl-docsteam to docs CODEOWNERS !3026
- Add Evan and Marcel to docs CODEOWNERS !3025
- Use CI_JOB_TOKEN to create releases !3023
- Explicitly set kubernetes pull image failure as script failure !3015
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
- Docs: Add how to log in as current user in the Terminal so gitlab-runner installs properly !3055
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
- Enable container scanning for gitlab runner !3027
- Garbage collection supports for kubernetes executor !2983
- Fix flakiness of the TestAttachReconnectReadLogs test !2954

## v14.1.0 (2021-07-20)

### Bug fixes

- Fix trace short writes for large log lines !2993
- Confirm if docker is installed in `clear-docker-cache` !2961

### Maintenance

- Add CODEOWNERS for docs !3017 (Ben Bodenmiller @bbodenmiller)
- Add TestBuildOnCustomDirectory/pwsh as test failure on windows 20h2 and 2004 and TestMachineIdleLimits on 1809 !3011
- Allow KUBECONFIG and GITLAB_CI env in integration tests !3010
- Fix vendor out of sync !3008
- Use image's Powershell Core for Windows tests !3005
- Remove explicit use of GOROOT/GOPATH now that we're using Go modules !3002
- Remove unneeded test configuration !3001
- Fail k8s integration tests when the check command fails !2999
- Fix on-demand releasing of helper images !2998
- Stop considering docker pull image as runner system failure !2995
- Skip docker-machine provision on failure by default !2986
- Fix make prepare_index read GPG_KEY from file !2985
- Fail CI build if test failures not updated !2976
- Only print necessary env vars in tests !2971
- Update environment name for Linux docker images !2970
- Don't run fuzz variable mask test for docs !2965
- Add environment for gitlab stable release !2962
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
- Update feature flag FF_SKIP_DOCKER_MACHINE_PROVISION_ON_CREATION_FAILURE grammar !3000
- Docs: Complete sentence, link to general SSL troubleshooting info !2994
- Update runner readmes to index !2990
- Added note for Overwriting Kubernetes Namespace section !2984
- Mention liveness project when adding Windows runners !2981
- Add details on how to assign Runner Manager to security fork project !2974
- Docs: Updated Shell topic titles to be more clear !2972
- Update kubernetes execution strategy documentation !2966
- Fix outdated VS Code package recommendation !2964
- Add docs about DEB/RPM packages signature verification !2963
- Docs: Specify exact Parallels product names !2960
- Provide JSON job response file for custom executor. !2912 (Paul Bryant @paulbry)
- Add instructions for proxying the GitLab registry !2865
- Fix typo/incorrect grammar !2842 (Per Lundberg @perlun)

## v14.0.0 (2021-06-19)

### New features

- Send GPU config string !2848
- Add support for selective git submodule paths inclusion/exclusion !2249

### Bug fixes

- Fix race blocking goroutine in shell executor !2910
- Order masked values by length to prevent longer values being partially revealed !2892
- Kubernetes attach strategy hangs when log file is deleted !2824

### Maintenance

- Enable kubernetes attach strategy by default !2955
- Add ASDF .tool-versions file !2948
- Make check test directives depend on prepare_done !2947
- Fix broken test output produced by MakeFatalToPanic !2929
- Use main branch for docs reviews !2925
- Disable windows anti-malware monitoring !2920
- Remove FF_RESET_HELPER_IMAGE_ENTRYPOINT feature flag !2906
- Remove legacy process termination for shell executor !2905
- Pull helper image from GitLab registry by default !2904
- Pwsh shell support for kubernetes when legacy execution strategy ff is set to false !2902
- Remove offpeak settings docker autoscaling !2897
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
- Remove ubuntu/eoan package !2888
- Publish Windows helper image :latest tags !2879
- Add Ubuntu-based runner-helper image !2835

### Documentation changes

- Add troubleshooting note on gitlab-runner symlink removal !2953
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
- Add troubleshooting steps to gitlab runner operator !2901
- Fix external links !2895
- Fixed typo in docker runner documentation !2891
- Use DNS option in TOML to avoid proxy and route in docs !2815
- Change order of steps to configure autoscaling !2665
- Update docs/executors/kubernetes.md !1903

### Other changes

- Remove conversion between failed and cancelled buildStage for prometheus metrics !2932
- Delete unused 1909 allowed test failures !2928
- Updated test case names for TestBuildJobStatusEnvVars !2907 (listout @listout)
- Specify the working version for lsif-go image !2898
- Remove /usr/lib/gitlab-runner symlink from packages !2890
- Make git-lfs tar checksum usage coherent !2268

## v13.12.0 (2021-05-20)

### New features

- Support git strategy with kubernetes executor !2862

### Bug fixes

- Add utf-8 invalid replacement encoder to trace transformers !2881
- Pass Powershell scripts as a file to shell executor !2874
- Add new eval execution strategy for capturing exit code !2818

### Maintenance

- Revert "Publish Windows helper image :latest tags" !2880
- Use latest docs linting images for testing !2877
- Ensure Docker client is in experimental mode !2870
- Improve trace masking performance !2863
- Use Powershell for resolving paths !2836
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
- Compile gitlab-runner binary for shell integration tests !2820
- Don't return error when checking feature flag !2812
- Simplify the triggerring of 'gitlab-docs' Review App pipelines !2809
- Fix git 1.8.3.1 job errors !2791
- Fix job duration value when in log format is json !2787
- Add support for CSI volumes !2784 (Brandon Butler @brandonbutler)
- Move Kubernetes integration tests to separate file !2779
- Support for env variables expansion for image name for kubernetes executor !2778
- Segregate integration tests in docker executor !2776
- Remove test with int overflow !2597 (Fábio Matavelli @fabiomatavelli)
- Adding git depth for submodules !2107 (Nico Bollen @bollenn)

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
- Add feature flags in config.toml !2811
- Update links to redirected files !2808
- Add a note to docs on usage of docker script from `13.9` !2806
- Remove docker-machine provision on creation failure !2805
- Improve documentation for GPUs for all executors !2804
- Update redirected links in runner docs !2802
- Add troubleshooting section in kubernetes executor documentation !2799 (Vincent Firmin @winkies)
- Edited for style and consistency !2777
- Document how to customize environment variables for Runner !2775

### Other changes

- Update warning message URL for DockerHub !2844

## v13.10.0 (2021-03-21)

### Bug fixes

- Don't print DockerHub helper image warning when custom image is defined !2761
- Allow graceful termination on Windows !2739

### Maintenance

- Include symbols in gitlab-runner binary !2800
- Move process package integration tests to own files !2794
- Update `code navigation` job definition !2792
- Rename shell_writer_test.go to reflect use !2782
- Move virtualbox executor integration tests to own file !2781
- Move parallels executor integration tests to own file !2780
- Update trace limit wording !2765
- Update of docker error message !2759
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
- Fix example gcp zone for docker+machine config !2771
- Runner: Update spelling exceptions list !2770
- Docs for installing runner on a separate machine !2767
- Update docs/monitoring/README.md !2766
- Fix misspelling of "Force" in Powershell examples !2764 (Gabriel Smith @yodal\_)
- Add runner execution flow diagram !2760
- Fix duplicate labels in CONTRIBUTING.md !2747 (h.yoshida @hirokiyoshida837)
- Add backticks around --google-accelerator docs !2742
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
- Update docker script default to docker prune volumes !2720
- Default to no Docker image compression in local environment !2717
- pwsh scripts can be passed over STDIN on shell executor !2715
- Update github.com/docker/cli dependency !2714
- Add artifact and cache download progress meter !2708
- Remove requirement for Docker daemon experimental mode from image build scripts !2707
- Fix the image that is used to create the pwsh tag !2706
- Exclude out/binaries/gitlab-runner-helper from binaries artifacts !2703
- Improve logging to packagecloud push !2702
- Upgrade Powershell Core to 7.1.1 !2696
- Make TestHelperImageRegistry not need real prebuilt images !2682
- Add test for permissions container in k8s !2676
- Add object storage cache credentials adapter !2674
- Add artifact/cache upload progress meter !2670
- Refactor Docker pull logic into dedicated package !2659
- Update to Docker client v20.10.2 !2658
- Update gitlab-terminal package !2656
- Create separate helper image with Powershell Core !2641
- Prioritize helper image specified in config to change K8s log dir permissions !2578 (naruhito @naruhito1)
- Remove helpers/trace redundant io.Pipe use !2464
- Kubernetes tests simplify build creation !2445
- Report deleted pods as a system failure with attach strategy !2444
- Fix incorrect path/filepath use !2313
- Improve docker cleanup script to also include old builds and images !2310
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
- do not link to unmaintained docker image cleanup app !2712 (Antoine Beaupré @anarcat)
- Fix formatting of FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY !2701 (Ben Bodenmiller @bbodenmiller)
- Clarify download instructions !2700
- Replace x86 and amd64 with 32-bit and 64-bit terminology for download !2699
- Add buildImage in the default openshift operator example !2698
- 1/3 Add crosslink to Kubernetes Agent docs !2697
- docs: Clarify self-signed certs on windows !2695 (Stefan Schmalzhaf @the_s)
- Docs: Fix minor whitespace inconsistency !2694 (Stefan Schmalzhaf @the_s)
- 27451 Fix Documentation - podAnnotation should be a TOML table !2692 (Benjamin Souty @B-Souty)
- Split docs linting jobs !2689
- Docs: Links documentation to working example for CMD Shell usage on Windows GitLab Runners where only PowerShell can be the default !2687
- Documentation - Supported OS updates !2683
- Whole hour periods for autoscale !2681
- Mention version sync on first sign of trouble !2680
- Fix typo in kubernetes.md !2675
- Removed extra spaces !2672
- Update install runner on Kubernetes install page - docs !2668
- Simplification of dind service section !2663 (Keith Kirkwood @keithkirkwood)
- Instructions for installing dependencies on CentOS. !2619 (David Hannasch @dHannasch)
- Include in docs details about the updated script !2586
- Changed recommendation to instance type in docs to a smaller one !2579 (Jan Pobořil @janpoboril)
- Document known race condition about helm upgrade !2541
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
- Add tests to Powershell shell !2634
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
- Update docs for the new gitlab runner operator !2640
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
- Update kubernetes.md to reflect https://gitlab.com/charts/gitlab-runner/merge_requests/34 !1470

### Other changes

- Add missing entry to 13.6 changelog !2642

## v13.7.0 (2020-12-21)

### Security fixes

- Updating min TLS version to 1.2 !2576
- Replace umask usage with files permission change when a non-root image used !2539

### Bug fixes

- Upgrade fastzip to v0.1.4 !2605
- Remove .git/config.lock in build directory !2580
- Fix attempting Kubernetes docker registry secret cleanup on failed creation !2429

### Maintenance

- Gracefully fail unexpected Stream() calls !2609
- Update lowest git version support inside of CI !2600
- windows: Don't log crypto/x509: system root pool warning !2595
- Add .editorconfig !2588
- Use helper image to change K8s log dir permissions !2573
- Fix check_modules command !2572
- Replace assert.True and errors.Is with assert.ErrorAs/ErrorIs !2571
- Exclude secure jobs from docs pipelines !2564
- Submit exit code back to Rails when a job fails !2562
- Fix dead URL in docker.go !2557 (Victor Mireyev @AmbientLighter)
- Pin StefanScherer/windows_2019_docker box to 2020.04.15 !2555
- Pull helper image from GitLab.com registry !2554
- Update testify package to version supporting errors.Is/As directly !2537
- Introduce docker internal user package !2534
- Introduce docker internal exec package !2533
- Send build trace bytesize in the final build update !2521
- Support Pod DNS policy for Kubernetes executor !2477
- Support Pod DNS Config and Policy for Kubernetes executor !2473
- Add support for Windows Server Core 2004 !2459 (Raphael Gozzo @raphaelgz)
- Ensure that runner is unregistered on registration failure !2447
- Make runner-and-helper-docker-host use host arch and os. !2432 (Horatiu Eugen Vlad @hvlad)
- Improve cache upload speed !2358 (Erik Lindahl @erik.lindahl)
- Disable syslogging by default for systemd systems !2333 (Matthias Baur @m.baur)

### GitLab Runner distribution

- Publish docker images to ECR public !2608
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
- Fix indentation of runners.cache in kubernetes.md !2592 (Yorgos Oikonomou @yorgos..oik)
- Fixed Vale future tense issues !2585
- Fixed Vale future tense errors !2584
- Moved Kubernetes keywords into sub-tables !2583
- Commented out modal install window details (2 of 2) !2577
- Fix trailing space issues in docs !2569
- Fix broken links in the gitlab-runner docs !2568
- Fix typo in monitoring documentation !2556 (Horst Gutmann @h.gutmann)
- Add documentation on how to add a new Windows version !2498
- Updated compatibility table !2489
- Update index page for style !2484
- Allow specifying `basefolder` when creating virtualbox VM !2461 (Jack Dunn @JackDunnNZ)
- Runner guided install (2 of 2) !2460
- Allow to set extra hosts on kubernetes executor !2446 (Horatiu Eugen Vlad @hvlad)
- Updates documentation to highlight that SELinux can cause errors in the "Prepare Environment" state !2309 (Sean McNamara @seanmcn)
- Update AWS autoscale docs for clarity !1820
- Update generated PowerShell script example mkdir !1565
- Add advice on network segmentation !1404

### Other changes

- Update GitLab Changelog configuration !2615
- Remove product from product-categories URL from template !2611

## v13.6.0 (2020-11-21)

### New features

- Add labels to cache-init docker container !2412
- Expose custom executor services with $CI_JOB_SERVICES !1827 (Jovan Marić @jovanmaric)
- Enable PowerShell Core support in Docker-Windows executor !2492

### Maintenance

- Expose ci job services as custom !2550
- Publish helper images to registry.gitlab.com !2540
- Allow user to define command and entrypoint to services from config !2525
- Consolidate helper exe location for Dockerfile build !2501
- Fix Azure cache uploads using Go Cloud !2500
- Fix definition of security related jobs !2499
- Move doc/dependency_decisions.yml file to a better place !2485
- Fix TestBuildCancel from timing out !2468
- Teach artifact/cache commands about the archive interface !2467
- Improve build logging testing !2465
- Skip CleanupFileVariables stage if no file variables !2456
- Change in interactive --url question to match docs !2431
- Added SubPath support to Kubernetes volume definitions !2424 (Matt Mikitka @mmikitka)
- Add fastzip archiver/extractor !2210
- Implement archiver/extractor interface !2195
- Manage driver defined job variables in custom executor !2032 (Paul Bryant @paulbry)
- Update doc about release windows image script !1561

### Documentation changes

- More Vale rules updates !2552
- Clarify windows install instructions !2549
- synchronize Vale rules and fix !2547
- Add reference to config.toml for setting docker image pull policy - docs !2545
- Remove extra parentheses !2542 (Ben Bodenmiller @bbodenmiller)
- Docs: Rename and redirect docs/install/registry_and_cache_servers.md !2535
- Add stage / group metadata to docs pages !2528
- Add mention that registry mirror is started as HTTP not HTTPS !2527
- Elaborate on Docker mirror, and link to docker doc !2526
- Docs: Redirected custom executor index page !2522
- Docs: Changed bullets to a table !2517
- Added docs for using a configuration template in the helm chart !2503
- Update vale rules !2502
- Use latest docs linting image !2497
- Docs: Updated top-level page !2496
- Update link to runner helper image in documentation !2494 (botayhard @botayhard)
- Change mention of custom cache containers to volumes !2491
- Add missing supported architectures for Runner helper !2490
- Update [runners.machine] section in Autoscaling GitLab Runner on AWS EC2 documentation !2480
- Provide a full list of metrics available for GitLab runners in the documentation !2479
- Clarify how service_account in toml is used !2476 (Ben Bodenmiller @bbodenmiller)
- Introduce usage of Runner Manager terminology !2474
- Docs: Revamp Runner home page !2472
- Update Kubernetes' documentation to include ephemeral storage requests/limits !2457
- Add kubernetes runners allowPrivilegeEscalation security context configuration !2430 (Horatiu Eugen Vlad @hvlad)
- Update Runner registry and cache documentation page !2386
- Cap maximum Docker Machine provisioning rate !1038 (Joel Low @lowjoel)

## v13.5.0 (2020-10-20)

### New features

- Allow runner to archive cache on failure !2416
- Add job status environment variables !2342
- Add labels to docker cache volumes !2334
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
- Use google driver for examples !2442
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
- Docs: Update docker tables to clarify what's supported !2404
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
- Fix Windows runner helper docker container !2379
- Fix metric reading race conditions !2360
- Record only first resolved credentials for each Docker registry !2357
- Ensure Powershell file variables contain no BOM !2320

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
- Add kubernetes node affinities settings !2324 (Alexander Petermann @lexxxel)
- Re-enable windows docker tests !2308
- Use new function to create Docker client !2299
- Add Secrets entry to job payload structures !2288
- Remove redundant docker executor integration tests !2211
- Add missing assert for mock !2116
- Allow overwriting Service and Helper container resources !2108 (Renan Gonçalves @renan.saddam)
- Use parallel compression and decompression for gzip archives and caches !2055 (Ben Boeckel @ben.boeckel)
- Add variable to enable fallback cache key !1534 (Erik Lindahl @erik.lindahl)
- Print Docker image digest !1380 (David Nyström @nysan)

### Documentation changes

- Update docs-lint job to use latest image. !2398
- Add note not to use AWS security group ID with docker machine !2396
- Docs: improve documentation grammar !2395 (Jonston Chan @JonstonChan)
- Fix grammar in documentation index page !2394 (AmeliaYura @AmeliaYura)
- Add documentation on how to use Ubuntu image in kubernetes !2393
- adding a tip on configuring timestamp in docker runner !2391
- Docs: Fix misspelled word !2383
- Update Vale and Markdownlint rules !2380
- Docs: Fix minor typo in Registering runners page !2376
- Add Azure Blob Storage support for cache !2366
- Add note to docs about using shell executor when building macOS/iOS apps !2365
- Cleaned up some of the wording for macOS install !2364
- Document node affinity !2363
- Change order of headers in exec docs !2362
- Docs: Edited Fargate doc !2355
- Fix broken link !2354
- Update kubernetes.md documentation replace example gitlabUrl !2353 (Tyler Wellman @tylerwel)
- Fix section numbering in docs/development !2349
- CONTRIBUTING.md: fix FreeBSD label !2348 (Kenyon Ralph @kenyon)
- Use `shell` instead of `bash` for markdown !2345
- Update Registering Runners page !2337
- Add documentation for configuring private registries with imagePullSecrets !2131 (Tom Bruyninx @TomBrx)

### Other changes

- Clarify --help text for --id flag !2385

## v13.3.0 (2020-08-20)

### Bug fixes

- Install Runner in /usr/bin and helper in /usr/lib in Linux !2329
- Fix powershell #requires use !2318
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
- Run docker import for helper-dockerarchive-host !2275
- Update changelog generator to accept new labels !2271
- Fix typo in DUMB_INIT_S390X_CHECKSUM variable name !2270
- Cache GOCACHE in CI !2187
- Enable DAG for some jobs !2076
- Upgrade git version !2306
- Update Ubuntu Docker container to Ubuntu 20.04 !2286 (Markus Teufelberger @markusteufelberger)
- Log additional docker-machine prep/cleanup info !2277

### Documentation changes

- Synchronize lint rules and fix where required !2341
- Fix name script !2339 (Andros Fenollosa @tanrax)
- Document how to renew GPG key !2336
- Update Documentation template to reflect standard !2332
- Fix broken external links !2331
- Document security release process !2322
- Fix incorrect Fargate cluster name !2321 (Matt Breden @mattbred56)
- Added specific token steps !2317
- Update docs.gitlab-ci.yml to use trigger-build script !2311
- Add content describing Runner behavior for changes to config.toml - docs !2307
- Made links descriptive !2302
- Creation of Openshift Runner doc. !2296
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
- Leverage docker buildx for the helper image and build for s390x !2206
- Enable PowerShell Core support in Shell Executor !2199
- Build and release binary for s390x !2196
- Label docker networks in the same way as containers !1930
- Tag helper image with runner version !1919 (Fábio Matavelli @fabiomatavelli)

### Bug fixes

- Fix kubernetes runner timeout when the image name is invalid !2197 (Matthias van de Meent @matthias.vandemeent)
- Update Git TLS settings to be configured for repo URL, not GitLab URL !2111
- Fix support for UNC paths in Powershell executor !1976 (Pedro Pombeiro @pedropombeiro)
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
- Fix helper-docker target !2226
- Fix code navigation job to wait until the image job is done !2221
- Fix a spelling error in the bug template and tidy up some other wording !2219
- Standardize Makefile target names !2216
- Fix data race in TestDockerCommandBuildCancel !2208
- Add native code intelligence block to CI !2201
- Speed up `clone test repo` job !2192
- Fix flaky TestListenReadLines in log processor !2191
- Run kubernetes integration tests !2155
- Enable unparam linter and fix reported errors !2135
- Enable errcheck linter !2134
- Fix Dockerfile issues reported by halolint !2106
- Fix out-of-date test expectations !2012
- Update entrypoint shebang for docker images !1780 (J0WI @J0WI)
- Reduced layer count on Windows helper images !1777 (Alexander Kutelev @kutelev)
- Update to alpine v3.12 !1763

### Documentation changes

- Docs: Updated note to add install from UI instructions !2264
- update "screenshot" of running gitlab-runner without arguments. from version 1.0.0 to 13.0 !2262 (@mxschumacher @mxschumacher)
- Session server listen on IPv4 and IPv6 !2260
- Update documentation for helper image tags !2258
- Synchronize lint rules !2254
- Update custom executor docs with `step_*` !2253
- Docs: Fixed git commands !2244 (Stefan Zehe @szehe)
- Docs: Updated broken links !2240
- Adjust metadata and move page !2235
- Docs: fix broken external links !2234
- Fix Debian container path and SSH port in the Autoscaling GitLab CI on AWS Fargate guide !2230
- New config for Vale and markdownlint !2214
- Note that Interactive Web terminal don't work with helm yet !2189 (Ben Bodenmiller @bbodenmiller)
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
- Support runner predefined variables inside overwrite variables kubernetes !2069
- Add Centos8 and Ubuntu 19.10 & 20.04 packages !2002
- Change default git fetch flags allowing user to overwrite them !2000 (Łukasz Groszkowski @falxcerebri)
- Run any step from job response in a separate BuildSection !1963

### Bug fixes

- Fix missing logs from docker executor !2101
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
- Rename network manager file for docker executor !2147
- Enable staticcheck linter !2136
- Update GitLab CI image to include git lfs !2124
- Implement Is for \*BuildError !2121
- Update log message for failure of removing network for build !2119 (Max Wittig @max-wittig)
- Change license management to use rules !2096
- Use Docker client's ContainerWait !2073
- Use taskkill windows !1797
- Cleanup dependencies for alpine based docker image !1778 (J0WI @J0WI)

### Documentation changes

- Add all Vale rules from main GitLab project !2203
- Docs: Fix distribution order !2200 (Martin @C0rn3j)
- Update the register page to use the correct Docker registration commands - docs !2186
- Sync spelling exceptions list from GitLab project !2184
- Docs: fix broken links in Runner docs !2183
- Remove reference to lack of arm64 Docker images !2178
- Fix documentation toml examples with [[runners.machine.autoscaling]] !2177
- Update GitLab Runner in a container documentation to prevent errors in using the Runner image - docs !2175
- Docs: Edited runners.cache.s3 details !2167
- Add example logs for `runner` and `json` log-format options - docs !2163
- Adds workaround for env vars in config.toml !2156
- Update redirected links !2152
- Add Docker to capitalization rules !2146
- Include MachineName and MachineDriver in autoscaling example !2140
- Specify pull policy for kubernetes executor !2129
- Improve Batch deprecated details !2128 (Ben Bodenmiller @bbodenmiller)
- docs: Link to example of how to color PowerShell output !2127 (Ben Bodenmiller @bbodenmiller)
- Docs: removed ubuntu from LXD instructions !2126
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
- Rename ill-named script variable in release_docker_images !2173
- Change alpine mirrors to default mirrors for arm/arm64 !2165
- Skip flaky log processor test TestResumesFromCorrectSinceTimeAfterSuccessThenFailure !2151
- Enable gocritic linter !2145
- Return error from k8s `limits` function when parsing resource limits !2144
- Upgrade golangci-lint to v1.27.0 !2139
- Pass an explicit context path to docker build in `build_ci_image` !2133
- Fix error when using attach strategy and ErrSkipBuildStage is returned when generating script !2123
- Revert removal of Windows Batch support !2112
- Do not log warning if trace update interval header value is empty !2103
- Add retries for runner system failures in CI !2098
- Remove `--kubernetes-services` command line flag !2074
- More verbose logging for artifact uploading !2052 (Sashi @ksashikumar)
- Fix file name typo !2049
- Unify docker registry authentication in docker and kubernetes executors !2048
- Improve kubernetes executor attach strategy command execution and handling by using a new read-logs command in the helper image !2038
- Remove superfluous packages from ubuntu based docker image !1781 (J0WI @J0WI)

## v13.0.1 (2020-06-01)

### Bug fixes

- Fix missing logs from docker executor !2101

## v13.0.0 (2020-05-20)

### Breaking Changes

- Remove support for --docker-services flag on register command !2036
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
- Add arm64 Docker images for gitlab/gitlab-runner !1861
- Make docker machine configuration more elastic !1980
- Add support for `direct_download` artifacts !2093

### Bug fixes

- Fix duplicate volume check with trailing slash !2050
- Fix permissions of docker volumes created by Runner !2047
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
- Pin CI jobs to gitlab-org runners !1979
- Replace Code Climate with golangci-lint !1956
- Change license management to use rules !2096

### Documentation changes

- Update capitalization configuration !2084
- Update proxy.md documentation for grammar and clarity !2071 (Kade Cole @kadecole)
- Add link to AWS Fargate documentation page !2070
- Adds the link to new AWS Fargate page !2068
- Add more Vale rules to project !2061
- Remove tip alert box !2054
- Added Kaniko reference materials to Runner helm charts page !2039
- Sync Vale substitutions rules from GitLab project !2029
- Update PowerShell documentation to include video and working example project. !2028
- Handle situation where vale docs-lint error is overwritten by markdownlint success !2025
- Update faq to include firewall troubleshooting !2023
- Add recommended extensions for VS Code !2022
- Move documentation linting to Makefile !2021
- Add section about using TLS with custom CA in regular build scripts !2018
- Sync Markdownlint settings from GitLab project !2015
- Fixed helm search command !2007 (penguindustin @penguindustin)
- Improve signals documentation and add a best practice for graceful shutdown !1988
- Make docker machine configuration more elastic !1980
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
- Fix some Windows docker executor test !1789

### Documentation changes

- Minor Update index.md !2004 (KATO Tomoyuki @tomo667a)
- Minor rewording in PROCESS.md templates !2003
- Add further checks from GitLab project !2001
- Add info that ssh is also required to be accessible in the security group !1997 (Daniel Schwiperich @d.schwiperich)
- Add Vale version text rule !1994
- Clean up note style !1993
- Fix redirected links in docs !1992
- Updates Markdownlint configuration from GitLab project !1991
- Added link to the git download page !1972
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
- Create network per build in docker executor !1569 (Steve Exley @steve.exley)
- Overwrite kubernetes resource limits and requests for build container on job level !874 (Nicholas Colbert @45cali)

### Bug fixes

- Kubernetes execute commands with attach instead of exec !1775
- Retry kubernetes commands when "error dialing backend: EOF" error is hit !1867

### Maintenance

- Upgrade docker client to version 19.03.5 for CI image !1874
- Fix docker machine executor test filename !1927
- Remove executor\_ prefix from filenames in the executors package !1902
- Fix 'make all' target !1900
- Replace changelog generator script !1888
- Bump Minio to latest version !1881 (Tom Elliff @tomelliff)
- Rename build makefile target to build_all !1873
- Prevent building mips and s390x architectures by default !1872
- Make pipelines running also for X-Y-stable branches !1871
- Add double quotes around bash arguments in ci/release_docker_images !1865
- Fix release docker warning !1864
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
- Change flag to docker-services !1830
- Document Windows supported versions !1533

## v12.8.0 (2020-02-22)

- Define most jobs as 'pipelines for merge requests' !1747
- Build ci images only on related file changes !1746
- Make referees package mocks to be generated by mockery !1729
- Replace ruby:2.0/2.1 in examples and test cases with ruby:2.6 !1749
- Update deprecation warning for runner.docker.service !1751
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
- Typo/Grammar improvements in docker.md !1757
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
- Update kubernetes.md, fix typo in `<CERTIFICATE_FILENAME>` !1802
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
- Fix values.yaml file name in documentation !1812
- Update links to MRs in runner docs !1814
- Update removal date of feature flags to 13.0 !1818
- Increase allowed data races !1815
- Fix panic for exec command !1811
- Update github.com/stretchr/testify dependencies !1806
- Add support for X-GitLab-Trace-Update-Interval header !1760
- Revert 9e1d067621855c7b75820d3a49ac82ef51e56342 !1816
- Cleanup kubernetes versions when checking for host aliases and don't fail on parse error !1823
- Add gitlab-runner-helper binaries to S3 release !1819
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
- Removed --name argument from helm install. !1718
- macOS limitations and example update !1505
- Update advanced-configuration.md - fix typo of mperiods !1722
- Fix Typos !1731
- Add a git version caveat !1732
- Update docs for Windows to use backslashes not forwardslashes !1738
- Do not embed mutex !1734
- Refactor CI pipeline !1733
- Add missing 'needs' entry for release docker images job template !1744
- docs: Replace ruby:2.1/2.2 with ruby:2.6 or 2.7 !1748
- Make 'tags: gitlab-org' a re-usable job tempalte !1745
- Change to go1.13 error wrapping !1709
- Refactor metrics referee tests !1714
- Refactor metrics referee !1725
- Copyedit doc for volumes_from in Docker executor config !1750
- Allow service alias from config in Docker executor !1673

## v12.6.0 (2019-12-22)

- Update list of fixes for docker-machine fork !1655
- Remove outdated note regarding non-existent 1.8.x brew Go formula in contributing docs !1661
- Add manual rpm and deb installation details !1650
- Remove GetGitTLSVariables method !1663
- Link to example of how to run DinD !1515
- Update feature flag deprecation !1672
- Add timeout when sending request for certificate !1665
- Support docker options for CPU shares and OOM adjust !1460
- Backport os.Expand from Go v1.10.8 !1677
- Switch to a specific version of govet analyzer !1690
- Update cloud.google.com/go to v0.49.0 !1682
- Add cmd to helper image !1645
- Update blurb for when people use the issue tracker for support tickets !1691
- Fixing typos !1685
- Remove dead code !1686
- Distribute arm64 binaries !1687
- (Rebased) Update kubernetes.md adding the missing gitlab-helper container which is ever created. !1693
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
- Fix lint on markdown files !1602
- Document GitLab docker machine fork !1596
- Update redirected link !1637
- Fix certificates chain generation regression introduced with 12.4.0-rc1 !1639
- Bump DOCKER_MACHINE_VERSION !1595
- Fix golint issues in machine pkg !1641
- Upgrade to alpine 3.10 !1636
- Fix #4684 for K3s/containerd !1605
- Update makefile to setup dev dependencies before running tests !1589
- Fix external Helm documentation links !1644
- Update Git version for Windows dev environment !1646
- Change config lock to create a separate lock file !1647
- Add few constants to executors/custom/api !1657
- Fix bind propagation for Linux volumes !1632
- Populate a list of machines with machines that might not yet be persisted on disk !914
- Add service definition in config for Kubernetes executor !1476

## v12.4.1 (2019-10-28)

- Fix TLS chain building !1643

## v12.4.0 (2019-10-21)

- Fix err logging for runner limit !1403
- Add the note about incompatibility of session_server with helm chart runner !1575
- Fix prepare_exec typo in docs !1576
- Docs edits to clarify feature flags motivations and usage in Runner !1568
- Change log levels for common errors !1578
- Extend custom executor config !1583
- Fix JSON inside of docs !1587
- Update link for Helm chart issue tracker !1588
- Add pipeline ID to docker labels !1592
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
- Update gopkg.in/yaml.v2 !1619
- Update prometheus libraries !1620
- Bump github.com/json-iterator/go to 1.1.7 !1621
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
- Update windows dev environment to git 2.22 !1530
- Update PowerShell ErrorActionPreference documentation !1535
- Remove debian buster from package list !1536
- Update tls-self-signed.md !1537
- Improve windows helper images build !1519
- show which service exactly is invalid !1531
- Change docs markdown linter from mdl to markdownlint !1540
- Replace bastion with Runner Manager !1547
- Add entry to FAQ, restructure also !1539
- Change docs review and cleanup jobs to same CI stage !1543
- docker.md: Correct Image Sizes !1542
- Add note on shell-based Docker image requirement !1459
- Fixed powershell commands for Windows Runner !1544
- Remove the scripting for release checklist issue creation !1556
- Use new location for helm charts repo !1553
- Make Notes look consistent !1555
- Change markdownlint wildcard format !1554
- Edit Docker images section !1550
- Update capitalization in runner docs !1559
- Docs/update ubuntu dev docs !1557
- Use standard commands for directory creation to make it powershell core compatible !1563
- Fix exiting with zero exit code when cmdlets fail !1558
- Enable support for long paths !1524
- Prevent dollar signs in shell codeblocks !1574
- Clarify feature flag usage instructions !1566
- Expose variable containing the 'short token' value !1571
- Update documentation about OffPeakTimezone !1567
- Set default PATH for helper image !1573

## v12.2.0 (2019-08-22)

- Update docs executor titles !1454
- Only default to Powershell on Windows if no other shell is specified !1457
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
- Docs/update min docker version !1480
- Docs: Fix typo in custom executor !1479
- Track Windows tests failures !1450
- Add requirements for contributing new hardware architectures !1478
- Fix markdown in runner docs (part 1) !1483
- Fix markdown in runner docs (part 2) !1484
- Update docs to specify default shell of OS !1485
- Further clarify Docker requirements !1486
- Fix typo and spacing in two runner docs !1487
- docs: gitlab-runner helper image has no arm64 build yet !1489
- Fix custom executor default config on register !1491
- Update Windows test failures !1490
- Expand markdown lint rules in runner !1492
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
- Prevent running multiple instances of the gitlab-runner process using the same configuration file !1496
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
- Upgrade base image for gitlab/gitlab-runner:ubuntu to ubuntu:18.04 !1413
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
- Update docs to fix markdown and square bracket use !1429
- Enforce consistent prefix for numbered lists !1435
- Remove fedora/30 from supported list !1436
- Add STOPSIGNAL to gitlab-runner docker images !1427
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

- [Require refspec to clone/fetch git repository](https://gitlab.com/gitlab-org/gitlab-runner/issues/4069).
- [Change command line API for helper images usage](https://gitlab.com/gitlab-org/gitlab-runner/issues/4013).
- [Remove old cache configuration](https://gitlab.com/gitlab-org/gitlab-runner/issues/4070).
- [Remove old metrics server configuration](https://gitlab.com/gitlab-org/gitlab-runner/issues/4072).
- [Remove `FF_K8S_USE_ENTRYPOINT_OVER_COMMAND` feature flag and old behavior](https://gitlab.com/gitlab-org/gitlab-runner/issues/4073).
- [Remove support for few Linux distributions that reached EOL](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1130).
- [Remove old `git clean` flow](https://gitlab.com/gitlab-org/gitlab-runner/issues/4175).

Please look into linked issues for details.

**Release changes:**

- Support windows docker volumes configuration !1269
- Fix powershell cloning !1338
- Docs: Update docker register non-interactive command !1309
- Update mocks !1343
- Change source for go-homedir !1339
- improve MR and issues templates !1347
- docs: reuse previous clone !1346
- Prevent copy and paste error due to not existed alpine tag. !1351
- Fix typo for usage of proxies within docker containers for runners !1342
- Add documentation for Windows Docker Executor !1345
- Fix volume mounting when mode specified !1357
- Update docs for docker executor description !1358
- Show error when volume length is not expected !1360
- Add feature flag to mounting volumes to services !1352
- Implement session endpoint to proxy build services requests !1170
- add build info for fedora 30 !1353
- Limit `docker-windows` to Windows !1362
- Update logging key for Docker Machine !1361
- Update docs to refer to Windows Batch deprecation !1371
- Remove deprecated git clean strategy !1370
- Remove support for deprecated metrics_server setting !1368
- Add labels to templates !1375
- Remove support for deprecated entrypoint configuration for K8S !1369
- Fix support for SELinux volume mounts & case sensitivity !1381
- Remove old docker helper image commands !1373
- Remove support for deprecated S3 cache configuration !1367
- Added --system flag information into gitlab-runner install command !1378
- Minor markdown fixes !1382
- Remove support for deprecated distributions !1130
- Add configuration of access_level for runners on registration !1323
- Remove doc notice for deprecated OSes !1384
- Remove deprecated clone/fetch command !1372
- Allow configuration of Pod Security Context by Kubernetes Exeutor !1036
- Fix case sensitivity for windows volumes !1389
- Accept docker-windows as an option on register !1388
- Add documentation for windows development !1183
- Document clear-docker-cache script !1390
- Store traces on disk !1315
- Make git init to be quiet !1383
- Fix several typos !1392
- Make volumes to work on linux docker on windows !1363
- Update CHANGELOG.md with 11.11.x patch releases !1393
- Dependencies license management with GitLab CI/CD !1279
- Fix default cache volume docker-windows register !1391
- Fixed date typo for v11.11.2 CHANGELOG entry !1394
- Update github.com/Microsoft/go-winio dependency !1348
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
- Make volumes to work on linux docker on windows !1363

## v11.11.1 (2019-05-24)

- Fix volume mounting when mode specified !1357
- Add documentation for Windows Docker Executor !1345
- Add feature flag to mounting volumes to services !1352

## v11.11.0 (2019-05-22)

- Fix PowerShell cloning !1338
- Add PowerShell support for Docker Executor !1243
- Support windows docker volumes configuration !1269
- Fix git lfs not getting submodule objects !1298
- Add homebrew installation method for macOS runners !837
- mention the 59th second timeperiod issue in the docs !490
- Refactor macOS install instructions !1303
- Edit note on edge case !1304
- Extract unsupportedOSTypeError to errors pkg !1305
- Optimise trace handling for big traces !1292
- Cleanup feature flags mess !1312
- Add more documentation for node tolerations !1318
- Typo: varialbes -> variables !1316
- Allow to configure FF using config.toml !1321
- Update link to the introduction of custom build directories !1302
- Allow to use FF to configure `/builds` folder !1319
- Create a single source of truth for feature flags !1313
- Clear up docs on how to select shell !1209
- Update feature flag documentation !1326
- Refactor Helper Image package to work with Kubernetes !1306
- Fix broken internal links !1332
- Refactor helperimage package tests !1327
- Change deprecation of FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER to 12.3 !1330
- Update cmd script example !1333
- Better explain the workflow in Docker executors doc !1310
- Exclude mock files from coverage reporting !1334
- Fix link syntax in advanced-configuration.md !1311
- Docs: Update contributing links from gitlab-ce !1308
- Update docker executor Executor Options initialization !1296
- Add test case for Linux helper image !1335
- Extract volumes configuration to a separate struct !1261

## v11.10.0 (2019-04-22)

**Deprecations:**

All deprecations, with a detailed description, are listed at
https://about.gitlab.com/2019/04/22/gitlab-11-10-released/#release-deprecations

1. With version 11.10 we're deprecating the feature flag
   [FF_USE_LEGACY_GIT_CLEAN_STRATEGY](https://docs.gitlab.com/runner/configuration/feature-flags.html#available-feature-flags).

**Release changes:**

- Fix git lfs not getting submodule objects !1298
- Refactor slightly ./shells/shellstest !1237
- Fix CI_PROJECT_DIR handling !1241
- Log time took preparing executors !1196
- Restore availability of pprof in the debug server !1242
- Move variables defining .gopath to a shared place for all Windows jobs !1245
- Docs: clarify runner api registration process !1244
- add lfs support to ubuntu docker runner !1192
- Add information about Kaniko for Kubernetes executor !1161
- Enable the docs CI job !1251
- Rename test to be more descriptive !1249
- Create the reviewers guide base document !1233
- Update codeclimate version !1252
- Add retryable err type !1215
- Get windows tag for helper image !1239
- Remove unnecessary log alias for logrus inport !1256
- Make gitlab-runner:alpine more specific, Add link to Dockerfiles sources,... !1259
- Docs: Fix broken anchor in docker.md !1264
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
- Add note about git-lfs !1294

## v11.9.2 (2019-04-09)

- Fix git lfs not getting submodule objects !1298

## v11.9.1 (2019-04-03)

- Make it again possible to disable Git LFS pull !1273
- Use delayed variable expansion for error check in cmd !1260
- Unexport common.RepoRemoteURL !1276

## v11.9.0 (2019-03-22)

**Deprecations:**

All deprecations, with a detailed description, are listed at
https://about.gitlab.com/2019/03/22/gitlab-11-9-released/#release-deprecations

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
   Extended Docker configuration (https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#extended-docker-configuration-options)
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
- Update docker API verion !1187
- Update alpine images to alpine 3.9 !1197
- Fix a typo in the description of the configuration option !1205
- Document creation of Docker volumes passed with docker exec --docker-volumes !1120
- Correct spelling of timed out in literals !1121
- Fix spelling and other minor improvements !1207
- Migrate service wait script to Go !1195
- Docs update: Run runner on kubernetes !1185
- Increase test timeout for shell executor !1214
- Follow style convention for documentation !1213
- Add test for runner build limit !1186
- Migrate cache bash script to Go for helper image !1201
- Document OS deprecations for 12.0 !1210
- Fix anchors in Runner documentation !1216
- Add `build_simple` to `help` make target !1212
- Split `make docker` for GitLab Runner Helper !1188
- Add windows Dockerfiles for gitlab-runner-helper !1167
- Make Runner tests working on Windows with our CI Pipeline !1219
- Fetch code from provided refspecs !1203
- Check either ntpdate command exists or not before trying to execute it !1189
- Deprecate helper image commands !1218
- Add script for building windows helper image !1178
- Fix ShellWriter.RmFile(string) for cmd shell !1226
- Mask log trace !1204
- Add note about pod annotations for more clarity !1220
- Resolve memory allocation failure when cloning repos with LFS objects bigger than available RAM !1200
- Release also on gitlab releases page !1232
- Restore availability of pprof in the debug server !1242

## v11.8.0 (2019-02-22)

- Kubernetes executor: add support for Node tolerations !941
- Update logrus version to v1.3.0 !1137
- Docs - Clarify Docker Runner Documentation !1097
- Update github.com/stretchr/testify dependency !1141
- Update LICENSE file !1132
- Update example of cache config !1140
- Update documentation for autoscaling on AWS !1142
- Remove unnecessary dep constraint !1147
- readme: make author block render md !999
- Corrected note when using a config container to mount custom data volume. !1126
- Fix typo in documentation of k8s executor. !1118
- Make new runner tokens compatible with docker-machine executor !1144
- docs: Use `sudo tee` for apt pinning. !1047
- docs: fix indendation !1081
- Updated hint on running Windows 10 shell as administrator !1136
- Fixed typo in logged information !1074
- Update registry_and_cache_servers.md !1098
- Update golang.org/x/sys !1149
- Refactor frontpage for grammar and style !1151
- Update github.com/Azure/go-ansiterm dependency !1152
- Testing on windows with vagrant !1003
- Add fix for race condition in windows cache extraction !863
- Consolidate docker API version definition !1154
- Prevent Executors from modifying Runner configuration !1134
- Update ExecutorProvider interface signature !1159
- Update logging for processing multi runner !1160
- Update kubernetes.md - fix typo for bearer_token !1162
- Update github.com/prometheus/client_golang dep !1150
- Remove ContainerWait from docker client !1155
- Update advanced-configuration.md: Fix blockquote not reaching the entire note !1163
- Fix docs review app URL !1169
- docs: Add a helpful command to reload config !1106
- Update AWS autoscale documentation !1166
- Refactor dockerfiles !1068
- Add link to AWS driver about default values !1171
- Add support for fedora/29 packages !1082
- Add windows server 2019 as default for windows development !1165
- Docs: Fix bad anchor links in runner docs !1177
- Improve documentation concerning proxy setting in the case of docker-in-docker-executor !1090
- Add few fixes to Release Checklist template !1135
- Set table to not display under TOC !1168
- Update Docker client SDK !1148
- docs: add GitLab Runner Helm Chart link !945

## v11.7.0 (2019-01-22)

- Docs: Cleaning up the executors doc !1114
- Update to testify v1.2.2 !1119
- Fix a typo in VirtualBox Executor docs !1124
- Use the term `macOS` instead of `OS X` or `OSX` !1125
- Update github.com/sirupsen/logrus dependency !1129
- Docs update release checklist !1131
- Kill session when build is cancelled !1058
- Fix path separator for CI_PROJECT_DIR in Windows !1128
- Make new runner tokens compatible with docker-machine executor !1144

## v11.6.0 (2018-12-22)

- Make compatibility chart super clear and remove old entries !1078
- Add slack notification option for 'dep status' check failures !1072
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
- Add a gitlab_runner_jobs_total metric !1018
- Add a job duration histogram metric !1025
- Filter content of X-Amz-Credential from logs !1028
- Disable escaping project bucket in cache operations !1029
- Fix example for session_server and added the note about where this section should be placed !1035
- Fix job duration counting !1033
- Log duration on job finishing log line !1034
- Allow disabling docker entrypoint overwrite !965
- Fix command and args assignment when creating containers with K8S executor !1010
- Support json logging !1020
- Change image for docs link checking !1043
- Fix command that prepares the definitions of tests !1044
- Add OomKillDisable option to Docker executor !1042
- Add docker support for interactive web terminal !1008
- Add support docker machine web terminal support !1046

## v11.3.0 (2018-09-22)

- Fix logrus secrets cleanup !990
- Fix test failure detection !993
- Fix wrongly generated `Content-Range` header for `PATCH /api/v4/jobs/:id/trace` request !906
- Improve and fix release checklist !940
- Add ~"git operations" label to CONTRIBUTING guide !943
- Disable few jobs for docs-/-docs branches !996
- Update release checklist issue template !995
- Fix HTTPS validation problem when ssh executor is used !962
- Reduce complexity of reported methods !997
- Update docker images to alpine:3.8 !984
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
- Add step for updating Runner helm chart !1009
- Clenup env, cli-options and deprecations of cache settings !1012

## v11.2.0 (2018-08-22)

- Fix support for Unicode variable values when Windows+PowerShell are used !960
- Update docs/executors/kubernetes.md !957
- Fix missing code_quality widget !972
- Add `artifact` format !923
- Improve some k8s executor tests !980
- Set useragent in Kubernetes API calls !977
- Clarifying the tls-ca-file option is in the [[runners]] section !973
- Update mocks !983
- Add building to development heading !919
- Add coverage report for unit tests !928
- Add /etc/nsswitch.conf to helper on docker executor to read /etc/hosts when upload artifacts !951
- Add busybox shell !900
- Fix support for features for shells !989
- Fix logrus secrets cleanup !990
- Fix test failure detection !993

## v11.1.0 (2018-07-22)

- Fix support for Unicode variable values when Windows+PowerShell are used !960
- Unify receivers used for 'executor' struct in ./executors/docker/ !926
- Update Release Checklist template !898
- Cache the connectivity of live Docker Machine instances !909
- Update kubernetes vendor to 1.10 !877
- Upgrade helper image alpine 3.7 !917
- Detect possible misplaced boolean on command line !932
- Log 'metrics_server' deprecation not only when the setting is used !939
- Speed-up ./executor/docker/executor_docker_command_test.go tests !937
- Remove go-bindata !831
- Fix the release of helper images script !946
- Sign RPM and DEB packages !922
- Improve docker timeouts !963
- Wrap all docker errors !964

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
- Explain gitlab-runner workflow labels !921
- Change Prometheus metrics names !912

## v10.8.0 (2018-05-22)

- Resolve "Invalid OffPeakPeriods value, no such file or directory." !897
- Fix type in Substrakt Health company name !875
- Rename libre to core !879
- Correct hanging parenthesis in index.md !882
- Update interfaces mocks !871
- Rename keyword in kubernetes executor documentation !880
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
- Refer to gitlab versions as libre, starter, premium, and ultimate !851
- Fix assert.Equal parameter order !854
- Upgrade docker-machine to v0.14.0 !850
- Refactor autoscale docs !733
- Add possibility to specify memory in Docker containers !847
- Upgrade helper image to alpine 3.6 !859
- Update docker images bases to alpine:3.7 and ubuntu:16:04 !860
- Verify git-lfs checksum !796
- Improve services health check !867
- Add proxy documentation !623
- Downgrade go to 1.8.7 !869
- Add support for max_job_timeout parameter in registration !846

## v10.6.0 (2018-03-22)

- Upgrade docker-machine to v0.14.0 !850
- Upgrade helper image to alpine 3.6 !859
- Add CI_RUNNER_VERSION, CI_RUNNER_REVISION, and CI_RUNNER_EXECUTABLE_ARCH job environment variables !788
- Always prefer creating new containers when running with Docker Executor !818
- Use IAM instance profile credentials for S3 caching !646
- exec command is no longer deprecated !834
- Print a notice when skipping cache operation due to empty cache key !842
- Switch to Go 1.9.4 !827
- Move dependencies to dep !813
- Improve output of /debug/jobs/list !826
- Fix panic running docker package tests !828
- Fixed typo in console output !845

## v10.5.0 (2018-02-22)

- Always prefer creating new containers when running with Docker Executor !818
- Improve output of /debug/jobs/list !826
- Fix panic running docker package tests !828
- Fix git 1.7.1 compatibility in executors/shell package tests !791
- Do not add /cache volume if already provided by the user during gitlab-runner register !807
- Change confusing Built value for development version !821
- docs: explain valid values for check_interval !801
- docs: Fix OffPeak variables list !806
- docs: Add note about gitlab-runner on the SSH host being used for uploads !817

## v10.4.0 (2018-01-22)

- Always load OS certificate pool when evaluating TLS connections !804
- Add (overwritable) pod annotations for the kubernetes executor !666
- docker.allowed_images can use glob syntax in config.toml !721
- Added docker runtime support !764
- Send `failure_reason` when updating job statues (GitLab API endpoint) !675
- Do not use `git config --local` as it's not available in git v1.7.1 !790
- Use local GOPATH in Makefile !779
- Move Bleeding Edge release from ubuntu/yakkety to ububut/artful !797
- Fix data race in commands package unit tests !787
- Fix data race in function common.(\*Trace).Write() !784
- Fix data races in executor/docker package !800
- Fix data races in network package !775

## v10.3.1 (2018-01-22)

- Always load OS certificate pool when evaluating TLS connections !804

## v10.3.0 (2017-12-22)

- Do not use `git config --local` as it's not available in git v1.7.1 !790
- new RC naming schema !780
- Stop Docker Machine before removing it !718
- add `--checkout --force` options to `git submodule update --init` !704
- Fix trailing "<nil>" in syslog logging !734
- Fix kubernetes executor job overwritten variables behavior !739
- Add zip archive for windows release files !760
- Add kubernetes executor connection with service account, bearer token can also be overwritten !744
- Fix SIGSEGV in kubernetes executor Cleanup !769

## v10.2.1 (2018-01-22)

- Do not use `git config --local` as it's not available in git v1.7.1 !790
- Always load OS certificate pool when evaluating TLS connections !804

## v10.2.0 (2017-11-22)

- Update supported platforms !712
- Fix typo in Kubernetes runner docs !714
- Add info on upgrading to Runner 10 !709
- Add some documentation for disable_cache configuration option !713
- Remove .git/HEAD.lock before git fetch !722
- Add helper_image option to docker executor config !723
- Add notes about gitlab-runner inside the VM being used for uploads !719
- Fix panic when global flags are passed as command flags !726
- Update minio go library to v3.0.3 !707
- Label ci_runner_builds metric with runner short token !729

## v10.1.1 (2018-01-22)

- Do not use `git config --local` as it's not available in git v1.7.1 !790
- Always load OS certificate pool when evaluating TLS connections !804

## v10.1.0 (2017-10-22)

- Allow customizing go test flags with TESTFLAGS variable !688
- Clarify that cloning a runner could be considered an attack vector !658
- Remove disable_verbose from docs !692
- Add info about pre 10.0 releases !691
- Update BurntSushi/toml for MIT-license !695
- Expose if running in a disposable environment !690
- Adds EmptyDir support for k8s volumes !660
- Update git-lfs to 2.3.1 !703
- Collect metrics on build stages !689
- Construct git remote URL based on configuration !698
- Set git SSL information only for gitlab host !687

## v10.0.2 (2017-10-04)

- Hide tokens from URLs printed in job's trace !708

## v10.0.1 (2017-09-27)

- Remove deprecation message from service management commands !699

## v10.0.0 (2017-09-22)

> **Note:** With 10.0, we've moved repository from https://gitlab.com/gitlab-org/gitlab-ci-multi-runner
> to https://gitlab.com/gitlab-org/gitlab-runner. Please update your Bookmarks!

> **Note:** Starting with 10.0, we're marking the `exec` and service-related commands as **deprecated**. They will
> be removed in one of the upcoming releases.

> **Note:** Starting with 10.0, we're marking the `docker-ssh` and `docker-ssh+machine` executors as **deprecated**.
> They will be removed in one of the upcoming releases.

> **Note:** Starting with 10.0, behavior of `register` command was slightly changed. Please look into
> https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/657 for more details.

- Lock runners to project by default on registration !657
- Update cli library !656
- Fix RunSingleCommand race condition in waitForInterrupts !594
- Add handling of non-existing images for Docker >= 17.07 !664
- Document how to define default image to run using Kubernetes executor !668
- Specify an explicit length for git rev-parse --short to avoid conflicts when run !672
- Add link to Kubernetes executor details !670
- Add install VirtualBox step & improve VM setup details !676
- Rename repository from gitlab-ci-multi-runner to gitlab-runner !661
- Fix variable file permission !655
- Add Release Checklist template !677
- Fix randomly failing test from commands/single_test.go !684
- Mark docker-ssh and docker-ssh+machine executors as DEPRECATED !681
- Mark exec and service-management commands as DEPRECATED !679
- Fix support for `tmpfs` in docker executor config !680

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
- Start using 'toc' in yaml frontmatter to explicitly disable it !644

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
- Warn on archiving git directory !591
- Add CacheClient with timeout configuration for cache operations !608
- Remove '.git/hooks/post-checkout' hooks when using fetch strategy !603
- Fix VirtualBox and Parallels executors registration bugs !589
- Support Kubernetes PVCs !606
- Support cache policies in .gitlab-ci.yml !621
- Improve kubernetes volumes support !625
- Adds an option `--all` to unregister command !622
- Add the technical description of version release !631
- Update documentation on building docker images inside of a kubernetes cluster. !628
- Support for extended docker configuration in gitlab-ci.yml !596
- Add ServicesTmpfs options to Docker runner configuration. !605
- Fix network timeouts !634

## v9.3.0 (2017-06-22)

- Make GitLab Runner metrics HTTP endpoint default to :9252 !584
- Add handling for GIT_CHECKOUT variable to skip checkout !585
- Use HTTP status code constants from net/http library !569
- Remove tls-skip-verify from advanced-configuration.md !590
- Improve docker machine removal !582
- Add support for Docker '--cpus' option !586
- Add requests backoff mechanism !570
- Fixed doc typo, change `--service-name` to `--service` !592
- Slight fix to build/ path in multi runner documentation !598
- Move docs on private Registry to GitLab docs !597
- Install Git LFS in Helper image for X86_64 !588
- Docker entrypoint: use exec !581
- Create gitlab-runner user on alpine !593
- Move registering Runners info in a separate document !599
- Add basic support for Kubernetes volumes !516
- Add required runners.docker section to example config. !604
- Add userns support for Docker executor !553
- Fix another regression on docker-machine credentials usage !610
- Added ref of Docker app installation !612
- Update linux-repository.md !615

## v9.2.2 (2017-07-04)

- Fix VirtualBox and Parallels executors registration bugs !589

## v9.2.1 (2017-06-17)

- Fix regression introduced in the way how `exec` parses `.gitlab-ci.yml` !535
- Fix another regression on docker-machine credentials usage !610

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
- Add links to additional kubernetes details !566
- Add '/debug/jobs/list' endpoint that lists all handled jobs !564
- Remove .godir !568
- Add PodLabels field to Kubernetes config structure !558
- Remove the build container after execution has completed !571
- Print proper message when cache upload operation failed !556
- Remove redundant ToC from autoscale docs and add intro paragraph !574
- Make possible to compile Runner under Openbsd2 !511
- Improve docker configuration docs !576
- Use contexes everywhere !559
- Add support for kubernetes service account and override on gitlab-ci.yaml !554
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
- Extend documentation about accessing docker services !527
- Replace b.CurrentStage with b.CurrentState where it was misused !530
- Docker provider metrics cleanups and renaming !531
- Replace godep with govendor !505
- Add histogram metrics for docker machine creation !533
- Fix cache containers dicsovering regression !534
- Add urls to environments created with CI release jobs !537
- Remove unmanaged docker images sources !538
- Speed up CI pipeline !536
- Add job for checking the internal docs links !542
- Mention Runner -> GitLab compatibility concerns after 9.0 release !544
- Log error if API v4 is not present (GitLab CE/EE is older than 9.0) !528
- Cleanup variables set on GitLab already !523
- Add faq entry describing how to handle missing zoneinfo.zip problem !543
- Add documentation on how Runner uses Minio library !419
- Update docker.md - typo in runners documentation link !546
- Add log_level option to config.toml !524
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
- Update docker-machine version to fix coreos provision !500
- Cleanup windows install docs !497
- Replace io.Copy with stdcopy.StdCopy for docker output handling !503
- Fixes typo: current to concurrent. !508
- Modifies autoscale algorithm example !509
- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313
- Fix indentation of 'image_pull_secrets' in kubernetes configuration example !512
- Show Docker image ID in job's log !507
- Fix word consistency in autoscaling docs !519
- Rename the binary on download to use gitlab-runner as command !510
- Improve details around limits !502
- Switch from CI API v1 to API v4 !517
- Make it easier to run tests locally !506
- Kubernetes private credentials !520
- Limit number of concurrent requests to builds/register.json !518
- Remove deprecated kubernetes executor configuration fields !521
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
- Limit number of concurrent requests to builds/register.json !518
- Replace b.CurrentStage with b.CurrentState where it was misused !530

## v1.11.1 (2017-03-03)

- Update docker-machine version to fix coreos provision !500

## v1.11.0 (2017-02-22)

- Fix S3 and packagecloud uploads step in release process !455
- Add ubuntu/yakkety to packages generation list !458
- Reduce size of gitlab-runner-helper images !456
- Fix crash on machine creation !461
- Rename 'Build (succeeded|failed)' to 'Job (succeeded|failed)' !459
- Fix race in helpers/prometheus/log_hook.go: Fire() method !463
- Fix missing VERSION on Mac build !465
- Added post_build_script to call scripts after user-defined build scripts !460
- Fix offense reported by vet. Add vet to 'code style' job. !477
- Add the runner name to the first line of log output, after the version !473
- Make CI_DEBUG_TRACE working on Windows CMD !483
- Update packages targets !485
- Update Makefile (fix permissions on /usr/share/gitlab-runner/) !487
- Add timezone support for OffPeak intervals !479
- Set GIT_SUBMODULE_STRATEGY=SubmoduleNone when GIT_STRATEGY=GitNone !480
- Update maintainers information !489

## v1.10.8 (2017-04-04)

- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313
- Don't install docs for the fpm Gem !526
- Mention tagged S3 sources in installation documentation !513
- Limit number of concurrent requests to builds/register.json !518
- Replace b.CurrentStage with b.CurrentState where it was misused !530

## v1.10.7 (2017-03-03)

- Update docker-machine version to fix coreos provision !500

## v1.10.6 (2017-02-22)

- Update Makefile (fix permissions on /usr/share/gitlab-runner/) !487

## v1.10.5 (2017-02-20)

- Update packages targets !485

## v1.10.4 (2017-01-31)

- Fix race in helpers/prometheus/log_hook.go: Fire() method !463

## v1.10.3 (2017-01-27)

- Fix crash on machine creation !461

## v1.10.2 (2017-01-26)

- Add ubuntu/yakkety to packages generation list !458
- Reduce size of gitlab-runner-helper images !456

## v1.10.1 (2017-01-23)

- Fix S3 and packagecloud uploads step in release process !455

## v1.10.0 (2017-01-22)

- Make /usr/share/gitlab-runner/clear-docker-cache script /bin/sh compatible !427
- Handle Content-Type header with charset information !430
- Don't raise error if machines directory is missing on machines listing !433
- Change digital ocean autoscale to use stable coreos channel !434
- Fix package's scripts permissions !440
- Use -q flag instead of --format. !442
- Kubernetes termination grace period !383
- Check if directory exists before recreating it with Windows CMD !435
- Add '--run-tagged-only' cli option for runners !438
- Add armv6l to the ARM replacements list for docker executor helper image !446
- Add configuration options for Kubernetss resource requests !391
- Add poll interval and timeout parameters for Kubernetes executor !384
- Add support for GIT_SUBMODULE_STRATEGY !443
- Create index file for S3 downloads !452
- Add Prometheus metric that counts number of catched errors !439
- Exclude unused options from AbstractExecutor.Build.Options !445
- Update Docker Machine in official Runner images to v0.9.0 !454
- Pass ImagePullSecrets for Kubernetes executor !449
- Add Namespace overwrite possibility for Kubernetes executor !444

## v1.9.10 (2017-03-23)

- Force-terminate VirtualBox and Parallels VMs so snapshot restore works properly !313

## v1.9.9 (2017-03-03)

- Update docker-machine version to fix coreos provision !500

## v1.9.8 (2017-02-22)

- Update Makefile (fix permissions on /usr/share/gitlab-runner/) !487

## v1.9.7 (2017-02-20)

- Update packages targets !485

## v1.9.6 (2017-01-25)

- Add ubuntu/yakkety to packages generation list !458

## v1.9.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.9.4 (2017-01-15)

- Add armv6l to the ARM replacements list for docker executor helper image !446

## v1.9.3 (2017-01-14)

- Fix package's scripts permissions !440
- Check if directory exists before recreating it with Windows CMD !435

## v1.9.2 (2017-01-04)

- Handle Content-Type header with charset information !430
- Don't raise error if machines directory is missing on machines listing !433

## v1.9.1 (2016-12-24)

- Make /usr/share/gitlab-runner/clear-docker-cache script /bin/sh compatible !427

## v1.9.0 (2016-12-22)

- Add pprof HTTP endpoints to metrics server !398
- Add a multiple prometheus metrics: !401
- Split prepare stage to be: prepare, git_clone, restore_cache, download_artifacts !406
- Update CONTRIBUTING.md to refer to go 1.7.1 !409
- Introduce docker.Client timeouts !411
- Allow network-sourced variables to specify that they should be files !413
- Add a retry mechanism to prevent failed clones in builds !399
- Remove shallow.lock before fetching !407
- Colorize log entries for cmd and powershell !400
- Add section describing docker usage do Kubernetes executor docs !394
- FreeBSD runner installation docs update !387
- Update prompts for register command !377
- Add volume_driver Docker configuration file option !365
- Fix bug permission denied on ci build with external cache !347
- Fix entrypoint for alpine image !346
- Add windows vm checklist for virtualbox documentation !348
- Clarification around authentication with the Kubernetes executor !296
- Fix docker hanging for docker-engine 1.12.4 !415
- Use lib machine to fetch a list of docker-machines !418
- Cleanup docker cache clear script !388
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

- Update Makefile (fix permissions on /usr/share/gitlab-runner/) !487

## v1.8.7 (2017-02-20)

- Update packages targets !485

## v1.8.6 (2017-01-25)

- Add ubuntu/yakkety to packages generation list !458

## v1.8.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.8.4 (2017-01-15)

- Add armv6l to the ARM replacements list for docker executor helper image !446

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
- Fix path to runners-ssh page !368
- Add initial Prometheus metrics server to runner manager !358
- Add a global index.md for docs !371
- Ensure that all builds are executed on tagged runners !374
- Fix broken documentation links !382
- Bug Fix: use a regex to pull out the service and version in the splitServiceAndVersion method !376
- Add FAQ entry about handling the service logon failure on Windows !385
- Fix "unit tests" random failures !370
- Use correct constant for kubernetes ressource limits. !367
- Unplug stalled endpoints !390
- Add PullPolicy config option for kubernetes !335
- Handle received 'failed' build state while patching the trace !366
- Add support for using private docker registries !386

## v1.7.5 (2017-01-21)

- Update Docker Machine in official Runner images to v0.9.0 !454

## v1.7.4 (2017-01-15)

- Add armv6l to the ARM replacements list for docker executor helper image !446

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
- Add docker_images release step to CI pipeline !333
- Refactor shell executor tests !334
- Introduce GIT_STRATEGY=none !332
- Introduce a variable to enable shell tracing on bash, cmd.exe and powershell.exe !339
- Try to load the InCluster config first, if that fails load kubectl config !327
- Squash the "No TLS connection state" warning !343
- Add a benchmark for helpers.ShellEscape and optimise it !351
- Godep: update github.com/Sirupsen/logrus to v0.10.0 !344
- Use git clone --no-checkout and git checkout --force !341
- Change machine.machineDetails to machine.Details !353
- Make runner name lowercase to work with GCE restrictions !297
- Add per job before_script handling for exec command !355
- Add OffPeak support for autoscaling !345
- Prevent caching failures from marking a build as failed !359
- Add missed "server" command for minio in autoscaled S3 cache tutorial !361
- Add a section for Godep in CONTRIBUTING.md !302
- Add a link to all install documentation files describing how to obtain a registration token !362
- Improve registration behavior !356
- Add the release process description !176
- Fix documentation typo in docs/configuration/advanced-configuration.md !354
- Fix data races around runner health and build stats !352

## v1.6.1 (2016-09-30)

- Add changelog entries generation script !322
- Add docker_images release step to CI pipeline !333

## v1.6.0 (2016-09-22)

- Remove an unused method from the Docker executor !280
- Add note about certificate concatenation !278
- Restore 755 mode for gitlab-runner-service script !283
- Remove git-lfs from docker helper images !288
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
- Add APT pinning configuration for debian in installation docs !303
- Remove yaml v1 !307
- Add options to runner configuration to specify commands executed before code clone and build !106
- Add RC tag support and fix version discovering !312
- Pass all configured CA certificates to builds !299
- Use git-init templates (clone) and git config without --global (fetch) to disable recurseSubmodules !314
- Improve docker machine logging !234
- Add possibility to specify a list of volumes to inherit from another container !236
- Fix range mismatch handling error while patch tracing !319
- Add docker+machine and kubernetes executors to "I'm not sure" part of executors README.md !320
- Remove ./git/index.lock before fetching !316

## v1.5.3 (2016-09-13)

- Fix Caret-escape parentheses when not inside double quotes for Windows cmd
- Remove LFS from prebuilt images

## v1.5.2 (2016-08-24)

(no changes)

## v1.5.1 (2016-08-24)

- Fix file mode of gitlab-runner-service script !283

## v1.5.0 (2016-08-22)

- Update vendored toml !258
- Release armel instead arm for Debian packages !264
- Improve concurrency of docker+machine executor !254
- Use .xz for prebuilt docker images to reduce binary size and provisioning speed of Docker Engines !249
- Remove vendored test files !271
- Update gitlab-runner-service to return 1 when no Host or PORT is defined !253
- Log caching URL address
- Retry executor preparation to reduce system failures !244
- Fix missing entrypoint script in alpine Dockerfile !248
- Suppress all but the first warning of a given type when extracting a ZIP file !261
- Mount /builds folder to all services when used with Docker Executor !272
- Cache docker client instances to avoid a file descriptor leak !260
- Support bind mount of `/builds` folder !193

## v1.4.3 (2016-09-13)

- Fix Caret-escape parentheses when not inside double quotes for Windows cmd
- Remove LFS from prebuilt images

## v1.4.2 (2016-08-10)

- Fix abort mechanism when patching trace

## v1.4.1 (2016-07-25)

- Fix panic while artifacts handling errors

## v1.4.0 (2016-07-22)

- Add sentry support
- Add support for cloning VirtualBox VM snapshots as linked clones
- Add support for `security_opt` docker configuration parameter in docker executor
- Add first integration tests for executors
- Add many logging improvements (add more details to some logs, move some logs to Debug level, refactorize logger etc.)
- Make final build trace upload be done before cleanup
- Extend support for caching and artifacts to all executors
- Improve support for Docker Machine
- Improve build aborting
- Refactor common/version
- Use `environment` feature in `.gitlab-ci.yml` to track latest versions for Bleeding Edge and Stable
- Fix Absolute method for absolute path discovering for bash
- Fix zombie issues by using dumb-init instead of github.com/ramr/go-reaper

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
- Add possibility to specify CpusetCpus, Dns and DnsSearch for docker containers created by runners
- Add a custom `User-Agent` header with version number and runtime information (go version, platform, os)
- Add artifacts expiration handling
- Add artifacts handling for failed builds
- Add customizable `check_interval` to set how often to check GitLab for a new builds
- Add docker Machine IP address logging
- Make Docker Executor ARM compatible
- Refactor script generation to make it fully on-demand
- Refactor runnsers Acquire method to improve performance
- Fix branch name setting at compile time
- Fix panic when generating log message if provision of node fails
- Fix docker host logging
- Prevent leaking of goroutines when aborting builds
- Restore valid version info in --help message
- [Experimental] Add `GIT_STRATEGY` handling - clone/fetch strategy configurable per job
- [Experimental] Add `GIT_DEPTH` handling - `--depth` parameter for `git fetch` and `git clone`

## v1.2.0 (2016-05-22)

- Use Go 1.6
- Add `timeout` option for the `exec` command
- Add runtime platform information to debug log
- Add `docker-machine` binary to Runner's official docker images
- Add `build_current` target to Makefile - to build only a binary for used architecture
- Add support for `after_script`
- Extend version information when using `--version` flag
- Extend artifacts download/upload logs with more response data
- Extend unregister command to accept runner name
- Update shell detection mechanism
- Update the github.com/ayufan/golag-kardianos-service dependency
- Replace ANSI_BOLD_YELLOW with ANSI_YELLOW color for logging
- Reconcile VirtualBox status constants with VBoxManage output values
- Make checkout quiet
- Make variables to work at job level in exec mode
- Remove "user mode" warning when running in a system mode
- Create `gitlab-runner` user as a system account
- Properly create `/etc/gitlab-runner/certs` in Runner's official docker images
- Disable recursive submodule fetchin on fetching changes
- Fix nil casting issue on docker client creation
- Fix used build platforms for `gox`
- Fix a limit problems when trying to remove a non-existing machines
- Fix S3 caching issues
- Fix logging messages on artifacts dowloading
- Fix binary panic while using VirtualBox executor with no `vboxmanage` binary available

## v1.1.4 (2016-05-14)

- Create /etc/gitlab-runner/certs
- Exclude architectures from GOX, rather then including
- Update mimio-go to a newest version
- Regression: Implement CancelRequest to fix S3 caching support
- Fix: Skip removal of machine that doesn't exist (autoscaling)

## v1.1.3 (2016-04-14)

- Regression: On Linux use `sh -s /bin/bash user -c` instead of `sh user -c`. This fixes non-login for user.
- Regression: Fix user mode warning
- Fix: vet installation
- Fix: nil casting issue on docker client creation
- Fix: docker client download issue

## v1.1.2 (2016-04-06)

- Regression: revert shell detection mechanism and limit it only to Docker

## v1.1.1 (2016-04-06)

- Fix: use different shell detection mechanism
- Regression: support for `gitlab-runner exec`
- Regression: support for login/non-login shell for Bash

## v1.1.0 (2016-03-29)

- Use Go 1.5
- Change license to MIT
- Add docker-machine based auto-scaling for docker executor
- Add support for external cache server
- Add support for `sh`, allowing to run builds on images without the `bash`
- Add support for passing the artifacts between stages
- Add `docker-pull-policy`, it removes the `docker-image-ttl`
- Add `docker-network-mode`
- Add `git` to gitlab-runner:alpine
- Add support for `CapAdd`, `CapDrop` and `Devices` by docker executor
- Add support for passing the name of artifacts archive (`artifacts:name`)
- Add support for running runner as system service on OSX
- Refactor: The build trace is now implemented by `network` module
- Refactor: Remove CGO dependency on Windows
- Fix: Create alternative aliases for docker services (uses `-`)
- Fix: VirtualBox port race condition
- Fix: Create cache for all builds, including tags
- Fix: Make the shell executor more verbose when the process cannot be started
- Fix: Pass gitlab-ci.yml variables to build container created by docker executor
- Fix: Don't restore cache if not defined in gitlab-ci.yml
- Fix: Always use `json-file` when starting docker containers
- Fix: Error level checking for Windows Batch and PowerShell

## v1.0.4 (2016-02-10)

- Fix support for Windows PowerShell

## v1.0.3 (2016-02-08)

- Fix support for Windows Batch
- Remove git index lock file: this solves problem with git checkout being terminated
- Hijack docker.Client to use keep-alives and to close extra connections

## v1.0.2 (2016-01-27)

- Fix bad warning about not found untracked files
- Don't print error about existing file when restoring the cache
- When creating ZIP archive always use forward-slashes and don't permit encoding absolute paths
- Prefer to use `path` instead of `filepath` which is platform specific: solves the docker executor on Windows

## v1.0.1 (2016-01-24)

- Use nice log formatting for command line tools
- Don't ask for services during registration (we prefer the .gitlab-ci.yml)
- Create all directories when extracting the file

## v1.0.0 (2016-01-22)

- Add `gitlab-runner exec` command to easy running builds
- Add `gitlab-runner status` command to easy check the status of the service
- Add `gitlab-runner list` command to list all runners from config file
- Allow to specify `ImageTTL` for configuration the frequency of docker image re-pulling (see advanced-configuration)
- Inject TLS certificate chain for `git clone` in build container, the gitlab-runner SSL certificates are used
- Remove TLSSkipVerify since this is unsafe option
- Add go-reaper to make gitlab-runner to act as init 1 process fixing zombie issue when running docker container
- Create and send artifacts as zip files
- Add internal commands for creating and extracting archives without the system dependencies
- Add internal command for uploading artifacts without the system dependencies
- Use umask in docker build containers to fix running jobs as specific user
- Fix problem with `cache` paths never being archived
- Add support for [`cache:key`](http://doc.gitlab.com/ce/ci/yaml/index.html#cachekey)
- Add warnings about using runner in `user-mode`
- Push packages to all upcoming distributions (Debian/Ubuntu/Fedora)
- Rewrite the shell support adding all features to all shells (makes possible to use artifacts and caching on Windows)
- Complain about missing caching and artifacts on some executors
- Added VirtualBox executor
- Embed prebuilt docker build images in runner binary and load them if needed
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
- Use separate containers to run pre-build (git cloning), build (user scripts) and post-build (uploading artifacts)
- Expand variables, allowing to use $CI_BUILD_TAG in image names, or in other variables
- Make shell executor to use absolute path for project dir
- Be strict about code formatting
- Move network related code to separate package
- Automatically load TLS certificates stored in /etc/gitlab-runner/certs/<hostname>.crt
- Allow to specify tls-ca-file during registration
- Allow to disable tls verification during registration

## v0.6.2 (2015-10-22)

- Fix PowerShell support
- Make more descriptive pulling message
- Add version check to Makefile

## v0.6.1 (2015-10-21)

- Revert: Fix tags handling when using git fetch: fetch all tags and prune the old ones

## v0.6.0 (2015-10-09)

- Fetch docker auth from ~/.docker/config.json or ~/.dockercfg
- Added support for NTFSSecurity PowerShell module to address problems with long paths on Windows
- Make the service startup more readable in case of failure: print a nice warning message
- Command line interface for register and run-single accepts all possible config parameters now
- Ask about tags and fix prompt to point to gitlab.com/ci
- Pin to specific Docker API version
- Fix docker volume removal issue
- Add :latest to imageName if missing
- Pull docker images every minute
- Added support for SIGQUIT to allow to gracefully finish runner: runner will not accept new jobs, will stop once all current jobs are finished.
- Implicitly allow images added as services
- Evaluate script command in subcontext, making it to close stdin (this change since 0.5.x where the separate file was created)
- Pass container labels to docker
- Force to use go:1.4 for building packages
- Fix tags handling when using git fetch: fetch all tags and prune the old ones
- Remove docker socket from gitlab/gitlab-runner images
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

- Update go-dockerclient to fix problems with creating docker containers

## v0.5.3 (2015-08-21)

- Pin to specific Docker API version
- Fix docker volume removal issue

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
- Rename gitlab_ci_multi_runner to gitlab-runner
- Don't require config file to exist in order to run runner
- Change where config file is stored: /etc/gitlab-runner/config.toml (*nix, root), ~/.gitlab-runner/config.toml (*nix, user)
- Create config on service install
- Require root to control service on Linux
- Require to specify user when installing service
- Run service as root, but impersonate as --user when executing shell scripts
- Migrate config.toml from user directory to /etc/gitlab-runner/
- Simplify service installation and upgrade
- Add --provides and --replaces to package builder
- Powershell: check exit code in writeCommandChecked
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

- Added CI=true and GITLAB_CI=true to environment variables
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
- Fixed wrong architecture for i386 debian packages
- Fixed problem allowing commands to consume build script making the build to succeed even if not all commands were executed

## v0.3.4 (2015-06-15)

- Create path before clone to fix Windows issue
- Added CI=true and GITLAB_CI=true
- Fixed wrong architecture for i386 debian packages

## v0.3.3 (2015-05-11)

- Push package to ubuntu/vivid and ol/6 and ol/7

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
- Checkout as detached HEAD (fixes compatibility with older git versions)
- Update documentation

## v0.2.0 (2015-04-23)

- Added delete and verify commands
- Limit build trace size (1MB currently)
- Validate build log to contain only valid UTF-8 sequences
- Store build log in memory
- Integrate with ci.gitlab.com
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
- Added the kardianos/service which allows to easily run gitlab-ci-multi-runner as service on different platforms
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

- Wait for docker service to come up (Loïc Guitaut)
- Send build log as early as possible

## v0.1.9 (2015-03-10)

- Fixed problem with resetting ruby environment

## v0.1.8 (2015-03-10)

- Allow to use prefixed services
- Allow to run on Heroku
- Inherit environment variables by default for shell scripts
- Mute git messages during checkout
- Remove some unused internal messages from build log

## v0.1.7 (2015-02-19)

- Fixed git checkout

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
