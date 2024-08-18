#!/usr/bin/env bash

# defaults
buildflags=""
binary="test.binary"
flags=""
iterations="100"
logprefix="test.out"

help="
This runs go tests repeatedly logging all the failures to separate
files. It is very useful for debugging with printf for tests which
don't fail very often.

Syntax: $0 [flags]

Note that flags for 'go test' need to be expanded, e.g. '-test.v' instead
of just '-v'. '-race' does not need to be expanded.

Flags this script understands

-h, --help
    show this help
-i=N, --iterations=N
    do N iterations (default ${iterations})
-b=NAME,--binary=NAME
    call the output binary NAME (default ${binary})
-l=NAME,--logprefix=NAME
    the log files generated will start with NAME (default ${logprefix})
-race
    build the binary with race testing enabled
-tags=TAGS
    build the binary with the tags supplied

Any other flags will be past to go test.

Example

    $0 flags -race -test.run 'TestRWFileHandleOpenTests'

"

if [[ "$@" == "" ]]; then
    echo "${help}"
    exit 1
fi

for i in "$@"
do
    case $i in
        -h|--help)
            echo "${help}"
            exit 1
            ;;
        -b=*|--binary=*)
            binary="${i#*=}"
            shift # past argument=value
            ;;
        -l=*|--log-prefix=*)
            logprefix="${i#*=}"
            shift # past argument=value
            ;;
        -i=*|--iterations=*)
            iterations="${i#*=}"
            shift # past argument=value
            ;;
        -race|--race|-tags=*|--tags=*)
            buildflags="${buildflags} $i"
            shift # past argument with no value
            ;;
        *)
            # unknown option
            flags="${flags} ${i#*=}"
            shift
            ;;
    esac
done

echo -n "Compiling ${buildflags} ${binary} ... "
go test ${buildflags} -c -o "${binary}" || {
    echo "build failed"
    exit 1
}
echo "OK"

for i in $(seq -w ${iterations}); do
    echo -n "Test ${buildflags} ${flags} ${i} "
    log="${logprefix}${i}.log"
    ./${binary} ${flags} > ${log} 2>&1
    ok=$?
    if [[ ${ok} == 0 ]]; then
        echo "OK"
        rm ${log}
    else
        echo "FAIL - log in ${log}"
    fi
done
