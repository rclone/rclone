#!/bin/bash


WORK_DIR=../../../bin
PID_FILE=dmn.pid
LOG_FILE=dmn.log
DMN="./sample"
DMN_STATUS="$DMN --status --silent"

cd $WORK_DIR
#export _GO_DAEMON=1

PID=
getpid() {
	if $DMN_STATUS; then
		echo "daemon is not running"
		exit
	else
		PID=`cat $PID_FILE`
	fi
}

case "$1" in
	start)
		if $DMN; then
			echo "starting daemon: OK"
		else
			echo "daemon return error code: $?"
		fi
		;;

	stop)
		getpid
		kill -TERM $PID
		echo "stopping daemon: OK"
		;;

	status)
		getpid
		echo "daemon pid: $PID"
		;;

	reload)
		getpid
		kill -HUP $PID
		echo "reloading daemon config: OK"
		;;

	clean)
		if $DMN_STATUS; then
			echo "" > $LOG_FILE
			echo "log cleaned"
		else
			echo "unable clean"
		fi
		;;

	log)
		cat $LOG_FILE
		;;
	*)
		echo "Usage: dmn.sh {start|stop|status|reload|clean|log}"
esac
