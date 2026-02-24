#!/bin/bash

set -eo pipefail

src=go.mod
tgt=COPYING_NOTES.md

STARTAUTOGEN="<!-- START AUTOGEN -->"
ENDAUTOGEN="<!-- END AUTOGEN -->"
RE_STARTAUTOGEN="^${STARTAUTOGEN}$"
RE_ENDAUTOGEN="^${ENDAUTOGEN}$"
tmpDepLicenses=""

error(){
    echo "Error: $*"
    exit 1
}

generate_dep_licenses(){
    [ -r $src ] || error "Cannot read file '$src'"


    tmpDepLicenses="$(mktemp)"

    # Collect all go.mod lines beginig with tab:
    # * which no replace
    # * which have replace
    grep -E $'^\t[^=>]*$'    $src  | sed -r 's/\t([^ ]*) v.*/\1/g'         > "$tmpDepLicenses"
    # Replace each line with formated link
    sed  -i -r '/^github.com\/therecipe\/qt\/internal\/binding\/files\/docs\//d;' "$tmpDepLicenses"
    sed -i -r 's|^(.*)/([[:alnum:]-]+)/(v[[:digit:]]+)$|* [\2](https://\1/\2/\3)|g' "$tmpDepLicenses"
    sed -i -r 's|^(.*)/([[:alnum:]-]+)$|* [\2](https://\1/\2)|g' "$tmpDepLicenses"
    sed -i -r 's|^(.*)/([[:alnum:]-]+).(v[[:digit:]]+)$|* [\2](https://\1/\2.\3)|g' "$tmpDepLicenses"

    ## add license file to github links, and others
    sed -i -r '/github.com/s|^(.*(https://[^)]+).*)$|\1 available under [license](\2/blob/master/LICENSE) |g' "$tmpDepLicenses"
    sed -i -r '/golang.org\/x/s|^(.*golang.org/x/([^)]+).*)$|\1 available under [license](https://cs.opensource.google/go/x/\2/+/master:LICENSE) |g' "$tmpDepLicenses"
    sed -i -r '/google.golang.org\/grpc/s|^(.*)$|\1 available under [license](https://github.com/grpc/grpc-go/blob/master/LICENSE) |g' "$tmpDepLicenses"
    sed -i -r '/google.golang.org\/protobuf/s|^(.*)$|\1 available under [license](https://github.com/protocolbuffers/protobuf/blob/main/LICENSE) |g' "$tmpDepLicenses"
    sed -i -r '/google.golang.org\/genproto/s|^(.*)$|\1 available under [license](https://pkg.go.dev/google.golang.org/genproto?tab=licenses) |g' "$tmpDepLicenses"
    sed -i -r '/go.uber.org\/goleak/s|^(.*)$|\1 available under [license](https://pkg.go.dev/go.uber.org/goleak?tab=licenses) |g' "$tmpDepLicenses"
    sed -i -r '/gopkg.in\/yaml\.v3/s|^(.*)$|\1 available under [license](https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE) |g' "$tmpDepLicenses"

}


check_dependecies(){
    generate_dep_licenses

    tmpHaveLicenses=$(mktemp)
    sed "/${RE_STARTAUTOGEN}/,/${RE_ENDAUTOGEN}/!d;//d" $tgt > "$tmpHaveLicenses"

    diffOK=0
    if ! diff "$tmpHaveLicenses" "$tmpDepLicenses"; then diffOK=1; fi

    rm "$tmpDepLicenses" || echo "Failed to clean tmp file"
    rm "$tmpHaveLicenses" || echo "Failed to clean tmp file"

    [ $diffOK -eq 0 ] || error "Dependency licenses are not up-to-date"
    exit 0
}

update_dependecies(){
    generate_dep_licenses

    sed -i -e "/${RE_STARTAUTOGEN}/,/${RE_ENDAUTOGEN}/!b" \
        -e "/${RE_ENDAUTOGEN}/i ${STARTAUTOGEN}" \
        -e "/${RE_ENDAUTOGEN}/r $tmpDepLicenses" \
        -e "/${RE_ENDAUTOGEN}/a ${ENDAUTOGEN}" \
        -e "d" \
        $tgt


    rm "$tmpDepLicenses" || echo "Failed to clean tmp file"

    exit 0
}

case $1 in
    "check") check_dependecies;;
    "update") update_dependecies;;
    *) error "One of actions needed: check update" ;;
esac

