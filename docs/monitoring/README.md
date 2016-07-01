# GitLab Runner Monitoring

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Embedded HTTP Statistics Server](#embedded-http-statistics-server)
  - [Configuration of HTTP Statistics Server](#configuration-of-http-statistics-server)
  - [Returned values](#returned-values)
  - [Usage example](#usage-example)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Embedded HTTP Statistics Server

> The embedded HTTP Statistics Server was introduced in GitLab Runner 1.4.0.

GitLab Runner is shipped with an embedded HTTP Statistics Server. The
server - if enabled - can be accessed with any HTTP client to gather
a statistic and debugging information.

HTTP Statistics Server is planned as an easy source of runtime debugging
and statistics information. This data is probably less useful for
a normal user, but it can be very important for administrators that are
hosting GitLab Runners.

For example, you may be interested if the _Load Average_ increase on your
runner's host is related to increase of processed builds or not. Or you
are running a cluster of machines to be used for the builds and you want
to track builds trend to plan changes in your infrastructure.

HTTP Statistics Server is giving you such possibility.

The HTTP Statistics Server can be configured to listen on a `tcp` socket,
so you can access it even with your web browser. If you don't want to
use TCP protocol (for performance, for security or for any other reason)
you can configure it to listen on a `unix` socket instead.

Data from the HTTP Statistics Server is returned as JSON encoded so you
can use it to script-up your monitoring with any popular scripting or
programing language.

### Configuration of HTTP Statistics Server

HTTP Statistics Server can be configured in two ways:

- with a `stats_server` global configuration option in `config.toml` file,
- with a `--stats-server` command line option for `run` command.

In both cases the option accepts a string with a format:

`[network_type]://[address]`

where:
- `network_type` can be one of `tcp` or `unix`,
- `address` is a valid address string for used `network_type`:
    - a path to a unix socket in case of `network_type = unix` (eg. `/var/run/gitlab-runner-stats.sock`)
    - an     `address:port` formated string in case of `network_type = tcp` (eg. `localhost:9999`)

Examples of addresses:

- `tcp://:9999` - will listen on all IPs of all interfaces on port `9999`
- `tcp://my-local-machine:1234` - will listen on an IP connected with `my-local-machine` on port `1234`
- `unix:///var/run/gitlab-runner-stats-server.sock`
- `unix://tmp/gitlab.sock`

Remember that for opening `tcp` sockets with port less than **1024** - at least
on Linux/Unix systems - you need to have root/administrator rights.

Also if you choose to use `unix` sockets, then you need to have write
access to the directory where you want to create the socket. The socket
will be created with ownership of user owning the Runner's process.

### Returned values

| Value                | Type    | Description |
|----------------------|---------|-------------|
| `builds_count`       | integer | Number of builds processed currently by the runner |
| `config_reloaded_at` | string  | Last config reload timestamp in RFC3339 format |
| `started_at`         | string  | Process start timestamp in RFC3339 format |
| `uptime`             | float   | Process uptime in hours |

### Usage example

In this example we will prepare a runners configuration file with
HTTP Statistics Server listening on a `unix` socket. Next we will
prepare a simple script that will use this socket to get builds count
and output this value in a format used by [Check_MK][checkmk_website] -
one of the popular monitoring solutions.

**`config.toml` file**

First we need to prepare a `config.toml` file. Let's assume that we are
running the GitLab Runner as system service on a Debian server.

The configuration file will be located at `/etc/gitlab-runner/config.toml`.
And we'll place the unix socket at `/var/run/gitlab-stats.sock`.

```toml
concurrent = 10
check_interval = 10
stats_server = "unix:///var/run/gitlab-stats.sock"

[[runners]]
  name = "runner-1"
  url = "https://gitlab.example.com/ci"
  token = "SECRET_TOKEN_HERE"
  executor = "shell"
```

If you want to use above example to do some testing then remember to
use a proper `url` of chosen GitLab instance and a valid `token` of
already registered runner. You can also [register the runner][registration-docs] -
see an [example][gitlab-registration-example] - and adjust `config.toml`
file after this.

When the configuration file is ready we can test if the configuration
is valid and the process can start:

```bash
root@server:# gitlab-runner run
Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
Running in system-mode.

Config loaded: statsserveraddress: unix:///var/run/gitlab-stats.sock
concurrent: 10
checkinterval: 10
user: ""
runners:
- name: runner-1
  limit: 0
  outputlimit: 0
  runnercredentials:
    url: https://gitlab.example.com/ci
    token: SECRET_TOKEN_HERE
    tlscafile: ""
  runnersettings:
    executor: shell
    buildsdir: ""
    cachedir: ""
    environment: []
    shell: ""
    ssh: null
    docker: null
    parallels: null
    virtualbox: null
    cache: null
    machine: null
modtime: {}
loaded: true
  builds=0
Starting StatsServer...                             socket=/var/run/gitlab-stats.sock
(...)
```

You should see a similar output in your terminal. If there is no error
we can stop the process by _CTRL + C_ and then start the system service:

```bash
root@server:# gitlab-runner start
root@server:#
```

We can check if the unix socket file was realy created:

```bash
root@server:# ls -lsa /var/run/gitlab-stats.sock
0 srwxr-xr-x 1 root root 0 Jul  1 14:49 /var/run/gitlab-stats.sock
```

**Connect to the HTTP Statistics Server**

First make sure that we have available software that we will use in
our monitoring script:

```bash
root@server:# apt-get update
root@server:# apt-get install -y netcat-openbsd jq
```

Let's try the command:

```bash
root@server:# echo -en "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n" | nc -U /var/run/gitlab-stats.sock
HTTP/1.1 200 OK
Content-Type: application/json
Date: Fri, 01 Jul 2016 15:15:56 GMT
Content-Length: 116

{"builds_count":0,"config_reloaded_at":"2016-07-01T14:49:25Z","duration":0.4419,"started_at":"2016-07-01T14:49:25Z"}
```

We should see a raw HTTP response with some headers (including `Content-Type: application/json`)
and JSON encoded body. If we've got the response then HTTP Statistics
Server is running and is ready to give us data.

**Prepare a monitoring script**

Now let's prepare a script which could be used by Check_MK. Check_MK
would execute the script in regular intervals (eg. each 5 minutes) and
then it would analyze the output of the script and send us warnings
and/or plot the data on some diagram.

Let's prepare a file `/root/gitlab_runner_check.sh`:

```bash
#!/bin/bash

SOCKET="/var/run/gitlab-stats.sock"
LIMIT=10
CRITICAL=9
WARNING=7

output=$(echo -en "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n" | nc -U ${SOCKET} | awk 'NR==1,/^\r/ {next} {printf "%s%s",$0,RT}')
started_at=$(echo $output | jq .started_at)
config_reloaded_at=$(echo $output | jq .config_reloaded_at)
builds_count=$(echo $output | jq .builds_count)
uptime=$(echo $output | jq .uptime)

state=0
if [ -n "$builds_count" ]; then
  if [ "$builds_count" -lt "$WARNING" ]; then
      state=0
  else
      if [ "$builds_count" -lt "$CRITICAL" ]; then
          state=1
      else
          state=2
      fi
  fi
else
  builds_count=0
fi

echo "$state GitLab_Runner_Statistics builds_count=$builds_count;$WARNING;$CRITICAL;0;$LIMIT|uptime=$uptime;;;;; OK - started_at=$started_at config_reloaded_at=$config_reloaded_at"
```

And let's check if it's working:

```bash
root@server:# chmod +x /root/gitlab_runner_check.sh
root@server:# /root/gitlab_runner_check.sh
0 GitLab_Runner_Statistics builds_count=0;7;9;0;10|uptime=0.1361;;;;; OK - started_at="2016-07-01T15:33:11Z" config_reloaded_at="2016-07-01T15:33:11Z"
```

**What does this script do?**

1. We are connecting to the HTTP Statistics Server with netcat and getting
   only the body of the response with `awk`.
1. Next - using `jq` - we are getting the value of some variables provided
   by the HTTP Statistics Server.
1. After this we are calculating the status of the check:
   - if `builds_count` is less than `WARNING` threshold, then we are
     setting the status to 0 - _success_,
   - if `builds_count` is higher than `WARNING` but less than `CRITICAL`
     we are setting the status to `1` - _warning_,
   - other way the status is set to `2` - _critical_.
1. We are also printing another performance metric - `uptime`. We don't
   calculate any status basing on this - we only want to store this
   in the monitoring system and have a graph of uptime changes.
1. We are also printing the dates of process start and last configuration
   file reload.

[checkmk_website]: http://mathias-kettner.com/check_mk.html "Check_MK - Official website"
[registration-docs]: docs/commands#registration-related-commands "Registration Commands - documentation"
[gitlab-registration-example]: docs/examples/gitlab.md "Example of non-interactive registration for GitLab.com"
