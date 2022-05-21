#!/usr/bin/env bash
#
# Upload a release
#
# Requires createrepo-c and rpm

set -e

FINGERPRINT=${1:-"FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA"}
BUCKET_PATH=${2:-"/mnt/yum.rclone.org"}

if [[ ! -d $BUCKET_PATH ]]
then
	echo "Bucket not mounted. Expected directory at ${BUCKET_PATH}."
	exit 1
fi

update_rpm_repo() {
	local RPM_FILE="$1"
	local RPM_REPO_DIR="$2"

	# query rpm version and package name
	local RPM_FULLNAME=$(rpm -qp "${RPM_FILE}")

	# query rpm arch separately
	local RPM_ARCH=$(rpm -qp --qf "%{arch}" "${RPM_FILE}")

	mkdir -p "${RPM_REPO_DIR}/${RPM_ARCH}/" &&
	cp "${RPM_FILE}" "${RPM_REPO_DIR}/${RPM_ARCH}/${RPM_FULLNAME}.rpm"
	echo "Copied ${RPM_FILE} to ${RPM_REPO_DIR}/${RPM_ARCH}/${RPM_FULLNAME}.rpm"

	# remove and replace repodata
	createrepo_c  --update "${RPM_REPO_DIR}/${RPM_ARCH}"

	rm -f "${RPM_REPO_DIR}/${RPM_ARCH}/repodata/repomd.xml.asc" &&
	gpg --default-key "$3" -absq -o "${RPM_REPO_DIR}/${RPM_ARCH}/repodata/repomd.xml.asc" "${RPM_REPO_DIR}/${RPM_ARCH}/repodata/repomd.xml"
}

for build in build/*.rpm; do
	update_rpm_repo "$build" "$BUCKET_PATH" "$FINGERPRINT"
	echo "Added ${build} to YUM repo."
done

echo "Done"
