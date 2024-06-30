#!/usr/bin/env bash

case "$1" in 
    start)
	start
	;;
    stop)
	stop
	;;
    status)
	status
	;;
    *)
	echo "usage: $0 start|stop|status" >&2
	exit 1
	;;
esac
