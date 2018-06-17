#/usr/bin/env bash

# Deletes all but the most current model version.
for v in $(ls ./models/apis | grep -v '.go' ); do
	for vm in $(ls -r models/apis/$v/ | sed -n '1!p' ); do
		echo "rm -r models/apis/$v/$vm"
		rm -r models/apis/$v/$vm
	done
done
