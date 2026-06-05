---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "구성, 인증서, 자동 크기 조정, 프록시 설정."
title: GitLab 러너 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너 구성하는 방법을 알아봅니다.

- [고급 구성 옵션](advanced-configuration.md):  [`config.toml`](https://github.com/toml-lang/toml) 구성 파일을 사용하여 러너 설정을 편집합니다.
- [자체 서명된 인증서 사용](tls-self-signed.md):  GitLab 서버에 연결할 때 TLS 피어를 확인하는 인증서를 구성합니다.
- [Docker Machine으로 자동 크기 조정](autoscale.md):  Docker Machine으로 자동으로 생성된 머신에서 작업을 실행합니다.
- [AWS EC2에서 GitLab 러너 자동 크기 조정](runner_autoscale_aws/_index.md):  자동으로 크기 조정된 AWS EC2 인스턴스에서 작업을 실행합니다.
- [AWS Fargate에서 GitLab CI 자동 크기 조정](runner_autoscale_aws_fargate/_index.md):  AWS Fargate 드라이버를 GitLab 사용자 지정 실행기와 함께 사용하여 AWS ECS에서 작업을 실행합니다.
- [그래픽 처리 장치](gpus.md):  GPU를 사용하여 작업을 실행합니다.
- [init 시스템](init.md):  GitLab 러너는 운영 체제에 따라 init 서비스 파일을 설치합니다.
- [지원되는 셸](../shells/_index.md):  셸 스크립트 생성기를 사용하여 다양한 시스템에서 빌드를 실행합니다.
- [보안 고려 사항](../security/_index.md):  GitLab 러너로 작업을 실행할 때 잠재적 보안 영향을 인식합니다.
- [러너 모니터링](../monitoring/_index.md):  러너의 동작을 모니터링합니다.
- [Docker 캐시 자동으로 정리](../executors/docker.md#clear-the-docker-cache):  디스크 공간이 부족한 경우 cron 작업을 사용하여 오래된 컨테이너 및 볼륨을 정리합니다.
- [프록시 뒤에서 실행되도록 GitLab 러너 구성](proxy.md):  Linux 프록시를 설정하고 GitLab 러너를 구성합니다. 이 설정은 Docker 실행기에서 잘 작동합니다.
- [Oracle Cloud Infrastructure(OCI)용 GitLab 러너 구성](oracle_cloud_performance.md):  OCI에서 GitLab 러너 성능을 최적화합니다.
- [속도 제한 요청 처리](proxy.md#handling-rate-limited-requests).
- [GitLab 러너 Operator 구성](configuring_runner_operator.md).
