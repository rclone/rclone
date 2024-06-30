#!/usr/bin/env bash

# start an "rclone serve" server

PIDFILE=/tmp/${NAME}.pid
DATADIR=/tmp/${NAME}-data

stop() {
    if status ; then
        pid=$(cat "$PIDFILE")
        kill "$pid"
        rm "$PIDFILE"
        echo "$NAME stopped"
    fi
}

status() {
    if [ -e "$PIDFILE" ]; then
        pid=$(cat "$PIDFILE")
        if kill -0 "$pid" >/dev/null 2>&1; then
            # echo "$NAME running"
            return 0
        else
            rm "$PIDFILE"
        fi
    fi
    # echo "$NAME not running"
    return 1
}

run() {
    if ! status ; then
        mkdir -p "$DATADIR"
        nohup "$@" >> "/tmp/${NAME}.log" 2>&1 </dev/null &
        pid=$!
        echo $pid > "$PIDFILE"
        disown "$pid"
    fi
}

# shellcheck disable=SC1090
. "$(dirname "$0")/run.bash"
