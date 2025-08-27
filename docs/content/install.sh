#!/usr/bin/env bash

# error codes
# 0 - exited without problems
# 1 - parameters not supported were used or some unexpected error occurred
# 2 - OS not supported by this script
# 3 - installed version of rclone is up to date
# 4 - supported unzip tools are not available

set -e

#when adding a tool to the list make sure to also add its corresponding command further in the script
unzip_tools_list=('unzip' '7z' 'busybox')

usage() { echo "Usage: sudo -v ; curl https://rclone.org/install.sh | sudo bash [-s beta]" 1>&2; exit 1; }

#check for beta flag
if [ -n "$1" ] && [ "$1" != "beta" ]; then
    usage
fi

if [ -n "$1" ]; then
    install_beta="beta "
fi


#create tmp directory and move to it with macOS compatibility fallback
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t 'rclone-install.XXXXXXXXXX')
cd "$tmp_dir"


#make sure unzip tool is available and choose one to work with
set +e
for tool in ${unzip_tools_list[*]}; do
    trash=$(hash "$tool" 2>>errors)
    if [ "$?" -eq 0 ]; then
        unzip_tool="$tool"
        break
    fi
done
set -e

# exit if no unzip tools available
if [ -z "$unzip_tool" ]; then
    printf "\nNone of the supported tools for extracting zip archives (${unzip_tools_list[*]}) were found. "
    printf "Please install one of them and try again.\n\n"
    exit 4
fi

# Make sure we don't create a root owned .config/rclone directory #2127
export XDG_CONFIG_HOME=config

#check installed version of rclone to determine if update is necessary
version=$(rclone --version 2>>errors | head -n 1)
if [ -z "$install_beta" ]; then
    current_version=$(curl -fsS https://downloads.rclone.org/version.txt)
else
    current_version=$(curl -fsS https://beta.rclone.org/version.txt)
fi

if [ "$version" = "$current_version" ]; then
    printf "\nThe latest ${install_beta}version of rclone ${version} is already installed.\n\n"
    exit 3
fi


#detect the platform
OS="$(uname)"
case $OS in
  Linux)
    OS='linux'
    # Define values specifically for this platform/OS.
    binTgtDir=/usr/bin
    man1TgtDir=/usr/share/man/man1
    ;;
  FreeBSD)
    OS='freebsd'
    ;;
  NetBSD)
    OS='netbsd'
    ;;
  OpenBSD)
    OS='openbsd'
    ;;
  Darwin)
    OS='osx'
    binTgtDir=/usr/local/bin
    man1TgtDir=/usr/local/share/man/man1
    ;;
  SunOS)
    OS='solaris'
    echo 'OS not supported'
    exit 2
    ;;
  *)
    echo 'OS not supported'
    exit 2
    ;;
esac

OS_type="$(uname -m)"
case "$OS_type" in
  x86_64|amd64)
    OS_type='amd64'
    ;;
  i?86|x86)
    OS_type='386'
    ;;
  aarch64|arm64)
    OS_type='arm64'
    ;;
  armv7*)
    OS_type='arm-v7'
    ;;
  armv6*)
    OS_type='arm-v6'
    ;;
  arm*)
    OS_type='arm'
    ;;
  *)
    echo 'OS type not supported'
    exit 2
    ;;
esac


#download and unzip
if [ -z "$install_beta" ]; then
    download_link="https://downloads.rclone.org/rclone-current-${OS}-${OS_type}.zip"
    rclone_zip="rclone-current-${OS}-${OS_type}.zip"
else
    download_link="https://beta.rclone.org/rclone-beta-latest-${OS}-${OS_type}.zip"
    rclone_zip="rclone-beta-latest-${OS}-${OS_type}.zip"
fi

curl -OfsS "$download_link"
unzip_dir="tmp_unzip_dir_for_rclone"
# there should be an entry in this switch for each element of unzip_tools_list
case "$unzip_tool" in
  'unzip')
    unzip -a "$rclone_zip" -d "$unzip_dir"
    ;;
  '7z')
    7z x "$rclone_zip" "-o$unzip_dir"
    ;;
  'busybox')
    mkdir -p "$unzip_dir"
    busybox unzip "$rclone_zip" -d "$unzip_dir"
    ;;
esac

cd $unzip_dir/*

#mounting rclone to environment

case "$OS" in
  'linux')
    # - - - - - - - - - - - - - - - - - - - -
    #binary
    # First verify our sudo ability. If we pass that then execute the steps
    # requested but make sure we're doing it as root and not as any other user.
    # (If sudo was run before this, the user may be set to something *other*
    # than 'root', this makes sure.
    sudo -v
    sudo -u root -- bash -c "\
        mkdir -m 0755 ${binTgtDir} && \
        cp rclone ${binTgtDir}/rclone.new && \
        chmod 0755 ${binTgtDir}/rclone.new && \
        chown root:root ${binTgtDir}/rclone.new && \
        mv ${binTgtDir}/rclone.new ${binTgtDir}/rclone"
    # - - - - - - - - - - - - - - - - - - - -
    #manual
    if ! [ -x "$(command -v mandb)" ]; then
        echo 'mandb not found. The rclone man docs will not be installed.'
    else
        # See sudo note above.
        sudo -v
        sudo -u root -- bash -c "\
            mkdir -m 0755 -p ${man1TgtDir} && \
            cp rclone.1 ${man1TgtDir} && \
            chmod 0644 ${man1TgtDir}/rclone.1" && \
            mandb --quiet --filename=${man1TgtDir}/rclone.1"
    # - - - - - - - - - - - - - - - - - - - -
    fi
    ;;
  'freebsd'|'openbsd'|'netbsd')
    #binary
    cp rclone /usr/bin/rclone.new
    chown root:wheel /usr/bin/rclone.new
    mv /usr/bin/rclone.new /usr/bin/rclone
    #manual
    mkdir -p /usr/local/man/man1
    cp rclone.1 /usr/local/man/man1/
    makewhatis
    ;;
  'osx')
    #binary
    mkdir -m 0555 -p ${binTgtDir}
    cp rclone ${binTgtDir}/rclone.new
    mv ${binTgtDir}/rclone.new ${binTgtDir}/rclone
    chmod a=x ${binTgtDir}/rclone
    #manual
    mkdir -m 0555 -p ${man1TgtDir}
    cp rclone.1 ${man1TgtDir}
    chmod a=r ${man1TgtDir}/rclone.1
    ;;
  *)
    echo 'OS not supported'
    exit 2
esac

#update version variable post install
version=$(rclone --version 2>>errors | head -n 1)

#cleanup
rm -rf "$tmp_dir"

printf "\n${version} has successfully installed."
printf '\nNow run "rclone config" for setup. Check https://rclone.org/docs/ for more details.\n\n'
exit 0
