package service_helpers

import "os"

func SysvScript() string {
	switch {
	case isDebianSysv():
		return sysvDebianScript
	case isRedhatSysv():
		return sysvRedhatScript
	}

	return ""
}

func isDebianSysv() bool {
	if _, err := os.Stat("/lib/lsb/init-functions"); err != nil {
		return false
	}
	if _, err := os.Stat("/sbin/start-stop-daemon"); err != nil {
		return false
	}
	return true
}

func isRedhatSysv() bool {
	if _, err := os.Stat("/etc/rc.d/init.d/functions"); err != nil {
		return false
	}
	return true
}

const sysvDebianScript = `#! /bin/bash

### BEGIN INIT INFO
# Provides:          {{.Path}}
# Required-Start:    $local_fs $remote_fs $network $syslog
# Required-Stop:     $local_fs $remote_fs $network $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: {{.DisplayName}}
# Description:       {{.Description}}
### END INIT INFO

DESC="{{.Description}}"
USER="{{.UserName}}"
NAME="{{.Name}}"
PIDFILE="/var/run/$NAME.pid"

# Read configuration variable file if it is present
[ -r /etc/default/$NAME ] && . /etc/default/$NAME

# Define LSB log_* functions.
. /lib/lsb/init-functions

## Check to see if we are running as root first.
if [ "$(id -u)" != "0" ]; then
    echo "This script must be run as root"
    exit 1
fi

do_start() {
  start-stop-daemon --start \
    {{if .ChRoot}}--chroot {{.ChRoot|cmd}}{{end}} \
    {{if .WorkingDirectory}}--chdir {{.WorkingDirectory|cmd}}{{end}} \
    {{if .UserName}} --chuid {{.UserName|cmd}}{{end}} \
    --pidfile "$PIDFILE" \
    --background \
    --make-pidfile \
    --exec {{.Path}} -- {{range .Arguments}} {{.|cmd}}{{end}}
}

do_stop() {
  start-stop-daemon --stop \
    {{if .UserName}} --chuid {{.UserName|cmd}}{{end}} \
    --pidfile "$PIDFILE" \
    --quiet
}

case "$1" in
  start)
    log_daemon_msg "Starting $DESC"
    do_start
    log_end_msg $?
    ;;
  stop)
    log_daemon_msg "Stopping $DESC"
    do_stop
    log_end_msg $?
    ;;
  restart)
    $0 stop
    $0 start
    ;;
  status)
    status_of_proc -p "$PIDFILE" "$DAEMON" "$DESC"
    ;;
  *)
    echo "Usage: sudo service $0 {start|stop|restart|status}" >&2
    exit 1
    ;;
esac

exit 0
`

const sysvRedhatScript = `#!/bin/sh
# For RedHat and cousins:
# chkconfig: - 99 01
# description: {{.Description}}
# processname: {{.Path}}

# Source function library.
. /etc/rc.d/init.d/functions

name="{{.Name}}"
desc="{{.Description}}"
user="{{.UserName}}"
cmd={{.Path}}
args="{{range .Arguments}} {{.|cmd}}{{end}}"
lockfile=/var/lock/subsys/$name
pidfile=/var/run/$name.pid

# Source networking configuration.
[ -r /etc/sysconfig/$name ] && . /etc/sysconfig/$name

start() {
    echo -n $"Starting $desc: "
    daemon \
        {{if .UserName}}--user=$user{{end}} \
        {{if .WorkingDirectory}}--chdir={{.WorkingDirectory|cmd}}{{end}} \
        "$cmd $args </dev/null >/dev/null 2>/dev/null & echo \$! > $pidfile"
    retval=$?
    [ $retval -eq 0 ] && touch $lockfile
    echo
    return $retval
}

stop() {
    echo -n $"Stopping $desc: "
    killproc -p $pidfile $cmd -TERM
    retval=$?
    [ $retval -eq 0 ] && rm -f $lockfile
    rm -f $pidfile
    echo
    return $retval
}

restart() {
    stop
    start
}

reload() {
    echo -n $"Reloading $desc: "
    killproc -p $pidfile $cmd -HUP
    RETVAL=$?
    echo
}

force_reload() {
    restart
}

rh_status() {
    status -p $pidfile $cmd
}

rh_status_q() {
    rh_status >/dev/null 2>&1
}

case "$1" in
    start)
        rh_status_q && exit 0
        $1
        ;;
    stop)
        rh_status_q || exit 0
        $1
        ;;
    restart)
        $1
        ;;
    reload)
        rh_status_q || exit 7
        $1
        ;;
    force-reload)
        force_reload
        ;;
    status)
        rh_status
        ;;
    condrestart|try-restart)
        rh_status_q || exit 0
        ;;
    *)
        echo $"Usage: $0 {start|stop|status|restart|condrestart|try-restart|reload|force-reload}"
        exit 2
esac
`
