#!/usr/bin/env bash
# This adds the version each backend was released to its docs page
set -e
for backend in $( find backend -maxdepth 1 -type d ); do
    backend=$(basename $backend)
    if [[ "$backend" == "backend" || "$backend" == "vfs" || "$backend" == "all" || "$backend" == "azurefile" ]]; then
        continue
    fi
    
    commit=$(git log --oneline -- $backend | tail -1 | cut -d' ' -f1)
    if [ "$commit" == "" ]; then
        commit=$(git log --oneline -- backend/$backend | tail -1 | cut -d' ' -f1)
    fi
    version=$(git tag --contains $commit | grep ^v | sort -n | head -1)
    echo $backend $version
    sed -i~ "4i versionIntroduced: \"$version\"" docs/content/${backend}.md
done
