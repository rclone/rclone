#!/bin/bash
#
# Upload a release
#
# Requires reprepro from https://github.com/ionos-cloud/reprepro

set -e

FINGERPRINT=${1:-"FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA"}
BUCKET_PATH=${2:-"/mnt/apt.rclone.org"}

# path for persistant files in bucket
LOCAL_PATH="$BUCKET_PATH/.local"
REPREPRO_PATH="$LOCAL_PATH/reprepro"

# if the bucket path does not exist, give error
if [[ ! -d $BUCKET_PATH ]]
then
	echo "Bucket not mounted. Expected directory at ${BUCKET_PATH}."
	exit 1
elif [[ ! -d $REPREPRO_PATH ]]
then
	echo "Config dir not found. Performing first time setup."
	mkdir -p "$REPREPRO_PATH/conf"; mkdir -p "$REPREPRO_PATH/db"; mkdir -p "$REPREPRO_PATH/log"
	cat <<- EOF > "$REPREPRO_PATH/conf/options"
		basedir $BUCKET_PATH
		dbdir $REPREPRO_PATH/db
		logdir $REPREPRO_PATH/log
	EOF
	cat <<- EOF > "$REPREPRO_PATH/conf/distributions"
		Origin: apt.rclone.org
		Label: Rclone
		Codename: any
		Architectures: amd64 i386 arm64 armhf armel mips mipsel
		Components: main
		Description: Apt repository for Rclone
		SignWith: $FINGERPRINT
		Limit: 20
	EOF
fi

for build in build/*.deb; do
	if [[ ${build} == *-arm.deb ]]
	then
		# do nothing because both arm and arm-v7 are armhf?
		echo "Skipping ${build}."
	else
		reprepro --confdir "$REPREPRO_PATH/conf" includedeb any "${build}"
		echo "Added ${build} to APT repo."
	fi
done

echo "Done"
