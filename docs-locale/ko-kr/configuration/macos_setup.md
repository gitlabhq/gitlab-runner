---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: macOS 러너 설정
---

macOS 러너에서 CI/CD 작업을 실행하려면 다음 단계를 순서대로 완료합니다.

작업을 완료하면 러너가 macOS 머신에서 실행되며 개별 러너가 작업을 처리할 준비가 완료됩니다.

- 시스템 셸을 Bash로 변경합니다.
- Homebrew, rbenv, 러너를 설치합니다.
- rbenv를 구성하고 Ruby를 설치합니다.
- Xcode를 설치합니다.
- 러너를 등록합니다.
- CI/CD를 구성합니다.

## 필수 요구 사항 {#prerequisites}

시작하기 전에:

- 최신 버전의 macOS를 설치합니다. 이 가이드는 11.4에서 개발되었습니다.
- 머신에 터미널 또는 SSH 액세스 권한이 있는지 확인합니다.

## 시스템 셸을 Bash로 변경 {#change-the-system-shell-to-bash}

최신 버전의 macOS는 Zsh을 기본 셸로 사용합니다. 하지만 러너의 셸 실행기는 CI/CD 스크립트가 올바르게 실행되도록 Bash이 필요합니다. 많은 스크립트가 Bash 특정 구문과 기능을 사용하기 때문입니다.

1. 머신에 연결하고 기본 셸을 확인합니다:

   ```shell
   echo $SHELL
   ```

1. 결과가 `/bin/bash`이 아니면 다음을 실행하여 셸을 변경합니다:

   ```shell
   chsh -s /bin/bash
   ```

1. 비밀번호를 입력합니다.
1. 터미널을 다시 시작하거나 SSH를 사용하여 다시 연결합니다.
1. `echo $SHELL`을 다시 실행합니다. 결과는 `/bin/bash`이어야 합니다.

## Homebrew, rbenv 및 GitLab 러너 설치 {#install-homebrew-rbenv-and-gitlab-runner}

러너는 머신에 연결하고 작업을 실행하기 위해 특정 환경 옵션이 필요합니다.

1. [Homebrew 패키지 관리자](https://brew.sh/)를 설치합니다:

   ```shell
   /bin/bash -c "$(curl "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")"
   ```

1. [`rbenv`](https://github.com/rbenv/rbenv)를 설정합니다. 이는 Ruby 버전 관리자이며 GitLab 러너도 설정합니다:

   ```shell
   brew install rbenv gitlab-runner
   brew services start gitlab-runner
   ```

## rbenv 구성 및 Ruby 설치 {#configure-rbenv-and-install-ruby}

이제 rbenv를 구성하고 Ruby를 설치합니다.

1. Bash 환경에 rbenv를 추가합니다:

   ```shell
   echo 'if which rbenv > /dev/null; then eval "$(rbenv init -)"; fi' >> ~/.bash_profile
   source ~/.bash_profile
   ```

1. Ruby 3.3.x를 설치하고 머신의 전역 기본값으로 설정합니다:

   ```shell
   rbenv install 3.3.4
   rbenv global 3.3.4
   ```

## Xcode 설치 {#install-xcode}

이제 Xcode를 설치하고 구성합니다.

1. 다음 위치 중 하나로 이동하여 Xcode를 설치합니다:

   - Apple App Store입니다.
   - [Apple Developer Portal](https://developer.apple.com/)입니다.
   - [`xcode-install`](https://github.com/xcpretty/xcode-install). 이 프로젝트는 명령줄에서 다양한 Apple 종속성을 더 쉽게 다운로드할 수 있도록 합니다.

1. 라이센스에 동의하고 권장되는 추가 구성 요소를 설치합니다. Xcode를 열고 프롬프트를 따르거나 터미널에서 다음 명령을 실행하여 이를 수행할 수 있습니다:

   ```shell
   sudo xcodebuild -runFirstLaunch
   ```

1. 활성 개발자 디렉토리를 업데이트하여 빌드 중에 Xcode가 적절한 명령줄 도구를 로드하도록 합니다:

   ```shell
   sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
   ```

### 프로젝트 러너 생성 및 등록 {#create-and-register-a-project-runner}

이제 [프로젝트 러너를 생성 및 등록](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token)합니다.

러너를 생성하고 등록할 때:

- GitLab에서 `macos` 태그를 추가하여 macOS 작업이 이 macOS 머신에서 실행되도록 합니다.
- 명령줄에서 `shell`을 [실행기](../executors/_index.md)로 선택합니다.

러너를 등록한 후 성공 메시지가 명령줄에 표시됩니다:

```shell
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

러너를 보려면:

1. 상단 표시줄에서 **검색 또는 이동**을 선택하고 프로젝트 또는 그룹을 찾습니다.
1. **설정 > CI/CD**를 선택합니다.
1. **러너**를 확장합니다.

### CI/CD 구성 {#configure-cicd}

GitLab 프로젝트에서 CI/CD를 구성하고 빌드를 시작합니다. 이 샘플 `.gitlab-ci.yml` 파일을 사용할 수 있습니다. 태그가 러너를 등록할 때 사용한 태그와 일치하는지 확인합니다.

```yaml
stages:
  - build
  - test

variables:
  LANG: "en_US.UTF-8"

before_script:
  - gem install bundler
  - bundle install
  - gem install cocoapods
  - pod install

build:
  stage: build
  script:
    - bundle exec fastlane build
  tags:
    - macos

test:
  stage: test
  script:
    - bundle exec fastlane test
  tags:
    - macos
```

macOS 러너가 이제 프로젝트를 빌드합니다.
