package main

import (
	"fmt"
	"github.com/kardianos/osext"
	"path/filepath"
)

func genSysvInitScript(relcfgpath string) string {
	exepath, _ := osext.Executable()
	abscfgpath, _ := filepath.Abs(relcfgpath)
	return fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides: logias
# Required-Start:    $local_fs $remote_fs $syslog $network $time
# Required-Stop:     $local_fs $remote_fs $syslog $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: server monitoring agent
# Description:       logias is a small server monitoring agent.
### END INIT INFO

PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
DAEMON="%s"
DAEMONARGS="-c %s"
NAME=`+"`basename ${DAEMON}`"+`
PIDFILE="/var/run/${NAME}/${NAME}.pid"
LOCKFILE="/var/lock/${NAME}/lock"
STDOUTLOG=/dev/null
KILLSIG=3
USER=root
STARTWAITLIMIT=3
STOPWAITLIMIT=10
SCRIPTNAME=/etc/init.d/${NAME}

if [ `+"`whoami`"+` != "root" ]; then
   echo "You must be root user to run this script."
   exit 1
fi

log_end_msg () {
  [ $1 -eq 0 ] && echo "[ OK ]" || echo "[ FAILED ]"
}

[ -f "/lib/lsb/init-functions" ] && . /lib/lsb/init-functions
[ -f "/etc/init.d/functions" ] && . /etc/init.d/functions
  
RETVAL=0
mkdir -p `+"`dirname ${LOCKFILE}`"+`
mkdir -p `+"`dirname ${PIDFILE}`"+`

case "$1" in
  start)
  echo -n "Starting $NAME: "
  $0 status > /dev/null 2>&1
  if [ $? -ne 0 ]; then # program is not running
    sudo -u ${USER} nohup ${DAEMON} ${DAEMONARGS} 0<&- 1> ${STDOUTLOG} 2>&1 &
    echo $! > ${PIDFILE}
    RETVAL=1
    for i in `+"`seq ${STARTWAITLIMIT}`"+`; do
      sleep 1
      ps aux | grep `+"`cat ${PIDFILE}`"+` | grep ${NAME} | grep -v grep > /dev/null 2>&1
      if [ $? -eq 0 ]; then
        touch ${LOCKFILE}
        RETVAL=0
        break
      fi
    done
  fi
  log_end_msg ${RETVAL}
  ;;

  stop)
  echo -n "Stopping $NAME: "
  $0 status > /dev/null 2>&1
  if [ $? -eq 0 ]; then # program is running
    cat ${PIDFILE} | xargs kill ${KILLSIG}
    RETVAL=1
    for i in `+"`seq ${STOPWAITLIMIT}`"+`; do
      ps aux | grep `+"`cat ${PIDFILE}`"+` | grep ${NAME} | grep -v grep > /dev/null 2>&1
      if [ $? -ne 0 ]; then
        rm -f ${PIDFILE} ${LOCKFILE}
        RETVAL=0
        break
      fi
      sleep 1
    done
  fi
  log_end_msg ${RETVAL}
  ;;

  restart)
  $0 status > /dev/null 2>&1
  if [ $? -eq 0 ]; then # program is running
    $0 stop
    $0 start
    RETVAL=$?
  fi
  ;;

  status)
  if [ ! -f ${PIDFILE} ]; then
    RETVAL=3 # program is not running
    echo "$NAME is not running"
  else
    kill -s 0 `+"`cat ${PIDFILE}`"+` > /dev/null 2>&1
    if [ $? -eq 0 ]; then
      RETVAL=0 # program is running or service is OK
      echo "$NAME is running"
    else
      if [ -f ${LOCKFILE} ]; then
        RETVAL=2 # program is dead and /var/run lock file exists
      else
        RETVAL=1 # program is dead and /var/run pid file exists
      fi
      echo "$NAME is not running"
    fi
  fi
  ;;

  rotate)
  $0 status > /dev/null 2>&1
  if [ $? -eq 0 ]; then # program is running
    cat ${PIDFILE} | xargs kill -USR1
  fi
  ;;

  *)
  echo "Usage: $SCRIPTNAME {start|stop|restart|status|rotate}" >&2
  RETVAL=3
  ;;

esac

exit $RETVAL
`, exepath, abscfgpath)
}
