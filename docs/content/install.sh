#!/bin/sh

# error codes
# 0 - exited without problems
# 1 - parameters not supported were used or some unexpected error occured
# 2 - OS not supported by this script
# 3 - installed version of rclone is up to date
# 4 - supported unzip tools are not available

set -e

#when adding a tool to the list make sure to also add it's corresponding command further in the script
unzip_tools_list=('unzip' '7z')

usage() { echo "Usage: curl https://rclone.org/install.sh | sudo bash [-s beta]" 1>&2; exit 1; }

#check for beta flag
if [ -n "$1" ] && [ "$1" != "beta" ]; then
    usage
fi

if [ -n "$1" ]; then
    install_beta="beta "
fi


#create tmp directory and move to it
tmp_dir=`mktemp -d`; cd $tmp_dir


#make sure unzip tool is available and choose one to work with
set +e
for tool in ${unzip_tools_list[*]}; do
    trash=`hash $tool 2>>errors`
    if [ "$?" -eq 0 ]; then
        unzip_tool="$tool"
        break
    fi
done  
set -e

# exit if no unzip tools available
if [ -z "${unzip_tool}" ]; then
    printf "\nNone of the supported tools for extracting zip archives (${unzip_tools_list[*]}) were found. "
    printf "Please install one of them and try again.\n\n"
    exit 4
fi

#check installed version of rclone to determine if update is necessary
version=`rclone --version 2>>errors | head -n 1`
if [ -z "${install_beta}" ]; then
    current_version=`curl https://downloads.rclone.org/version.txt`
else
    current_version=`curl https://beta.rclone.org/version.txt`
fi

if [ "$version" = "$current_version" ]; then
    printf "\nThe latest ${install_beta}version of rclone ${version} is already installed.\n\n"
    exit 3
fi



#detect the platform
OS="`uname`"
case $OS in
  Linux)
    OS='linux'
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

OS_type="`uname -m`"
case $OS_type in
  x86_64|amd64)
    OS_type='amd64'
    ;;
  i?86|x86)
    OS_type='386'
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
if [ -z "${install_beta}" ]; then
    download_link="https://downloads.rclone.org/rclone-current-$OS-$OS_type.zip"
    rclone_zip="rclone-current-$OS-$OS_type.zip"
else
    download_link="https://beta.rclone.org/rclone-beta-latest-$OS-$OS_type.zip"
    rclone_zip="rclone-beta-latest-$OS-$OS_type.zip"
fi

curl -O $download_link
unzip_dir="tmp_unzip_dir_for_rclone"
# there should be an entry in this switch for each element of unzip_tools_list
case $unzip_tool in
  'unzip')
    unzip -a $rclone_zip -d $unzip_dir
    ;;
  '7z')
    7z x $rclone_zip -o$unzip_dir
    ;;
esac
    
cd $unzip_dir/*


#mounting rclone to enviroment

case $OS in
  'linux')
    #binary
    cp rclone /usr/bin/
    chmod 755 /usr/bin/rclone
    chown root:root /usr/bin/rclone
    #manuals
    mkdir -p /usr/local/share/man/man1
    cp rclone.1 /usr/local/share/man/man1/
    mandb
    ;;
  'freebsd'|'openbsd'|'netbsd')
    #bin
    cp rclone /usr/bin/
    chmod 755 /usr/bin/rclone
    chown root:wheel /usr/bin/rclone
    #man
    mkdir -p /usr/local/man/man1
    cp rclone.1 /usr/local/man/man1/
    makewhatis
    ;;
  'osx')
    #binary
    mkdir -p /usr/local/bin
    cp rclone /usr/local/bin/
    #manual
    mkdir -p /usr/local/share/man/man1
    cp rclone.1 /usr/local/share/man/man1/    
    ;;
  *)
    echo 'OS not supported'
    exit 2
esac


printf '\nNow run "rclone config" for setup. Check https://rclone.org/docs/ for more details.\n\n'
exit 0
