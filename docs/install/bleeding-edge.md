---
last_updated: 2017-10-09
---

# GitLab Runner bleeding edge releases

CAUTION: **Warning:**
These are the latest, probably untested releases of GitLab Runner built straight
from `master` branch. Use at your own risk.

## Download the standalone binaries

* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-linux-386
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-linux-amd64
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-linux-arm
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-darwin-386
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-darwin-amd64
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-windows-386.exe
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-windows-amd64.exe
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-freebsd-386
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-freebsd-amd64
* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-freebsd-arm

You can then run the Runner with:
```bash
chmod +x gitlab-runner-linux-amd64
./gitlab-runner-linux-amd64 run
```

## Download one of the packages for Debian or Ubuntu

* https://gitlab-runner-downloads.s3.amazonaws.com/master/deb/gitlab-runner_i386.deb
* https://gitlab-runner-downloads.s3.amazonaws.com/master/deb/gitlab-runner_amd64.deb
* https://gitlab-runner-downloads.s3.amazonaws.com/master/deb/gitlab-runner_arm.deb
* https://gitlab-runner-downloads.s3.amazonaws.com/master/deb/gitlab-runner_armhf.deb

You can then install it with:
```bash
dpkg -i gitlab-runner_386.deb
```

## Download one of the packages for RedHat or CentOS

* https://gitlab-runner-downloads.s3.amazonaws.com/master/rpm/gitlab-runner_i686.rpm
* https://gitlab-runner-downloads.s3.amazonaws.com/master/rpm/gitlab-runner_amd64.rpm
* https://gitlab-runner-downloads.s3.amazonaws.com/master/rpm/gitlab-runner_arm.rpm
* https://gitlab-runner-downloads.s3.amazonaws.com/master/rpm/gitlab-runner_armhf.rpm

You can then install it with:
```bash
rpm -i gitlab-runner_386.rpm
```

## Download any other tagged release

Simply replace `master` with either `tag` (v0.2.0 or 0.4.2) or `latest` (the latest
stable). For a list of tags see <https://gitlab.com/gitlab-org/gitlab-runner/tags>.
For example:

* https://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-linux-386
* https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-386
* https://gitlab-runner-downloads.s3.amazonaws.com/v0.2.0/binaries/gitlab-runner-linux-386

If you have problem downloading through `https`, fallback to plain `http`:

* http://gitlab-runner-downloads.s3.amazonaws.com/master/binaries/gitlab-runner-linux-386
* http://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-386
* http://gitlab-runner-downloads.s3.amazonaws.com/v0.2.0/binaries/gitlab-runner-linux-386
