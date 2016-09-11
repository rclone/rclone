Rclone Package Building
============

An FPM recipe for RClone.

Note: FPM must be run on the output system to create the package, eg. must be run on RHEL/Fedora/CentOS to create and RPM.

```
$ git clone https://github.com/ncw/rclone/
$ cd package/
$ bundle install
$ sudo bundle exec fpm-cook
===> Verifying build_depends and depends with Puppet
===> Verifying package: rpm-build
===> Verifying package: unzip
===> Missing/wrong version packages: rpm-build, unzip
===> Running as root; installing missing/wrong version build_depends and depends with Puppet
===> Installing package: rpm-build
===> created
===> Installing package: unzip
===> All dependencies installed!
===> Starting package creation for rclone-1.33 (centos, rpm)
===>
===> Verifying build_depends and depends with Puppet
===> Verifying package: rpm-build
===> Verifying package: unzip
===> All build_depends and depends packages installed
===> Fetching source:
######################################################################## 100.0%
Archive:  /pipeline/source/fpm-recipes/rclone/cache/rclone-v1.33-linux-amd64.zip
   creating: rclone-v1.33-linux-amd64/rclone-v1.33-linux-amd64/
  inflating: rclone-v1.33-linux-amd64/rclone-v1.33-linux-amd64/README.html
  inflating: rclone-v1.33-linux-amd64/rclone-v1.33-linux-amd64/rclone
  inflating: rclone-v1.33-linux-amd64/rclone-v1.33-linux-amd64/rclone.1
  inflating: rclone-v1.33-linux-amd64/rclone-v1.33-linux-amd64/README.txt
===> Using source directory: rclone-v1.33-linux-amd64
===> Building in /pipeline/source/fpm-recipes/rclone/tmp-build/rclone-v1.33-linux-amd64
===> Installing into /pipeline/source/fpm-recipes/rclone/tmp-dest
===> [FPM] Converting dir to rpm {}
===> [FPM] Reading template {"path":"/usr/local/share/gems/gems/fpm-1.6.2/templates/rpm.erb"}
===> [FPM] Running rpmbuild {"args":["rpmbuild","-bb","--define","buildroot /tmp/package-rpm-build20160902-2618-1nu0u0d/BUILD","--define","_topdir /tmp/package-rpm-build20160902-2618-1nu0u0d","--define","_sourcedir /tmp/package-rpm-build20160902-2618-1nu0u0d","--define","_rpmdir /tmp/package-rpm-build20160902-2618-1nu0u0d/RPMS","--define","_tmppath ","/tmp/package-rpm-build20160902-2618-1nu0u0d/SPECS/rclone.spec"]}
===> [FPM] error: Macro %_tmppath has empty body {}
===> [FPM] error: Macro %_tmppath has empty body {}
===> [FPM] Executing(%prep): /bin/sh -e /var/tmp/rpm-tmp.suuroD {}
===> [FPM] Executing(%build): /bin/sh -e /var/tmp/rpm-tmp.6dGY9F {}
===> [FPM] Executing(%install): /bin/sh -e /var/tmp/rpm-tmp.iGRJVI {}
===> [FPM] Processing files: rclone-1.33-2.x86_64 {}
===> [FPM] Provides: rclone = 1.33-2 rclone(x86-64) = 1.33-2 {}
===> [FPM] Requires(rpmlib): rpmlib(PayloadFilesHavePrefix) <= 4.0-1 rpmlib(CompressedFileNames) <= 3.0.4-1 {}
===> [FPM] Conflicts: rclone {}
===> [FPM] Obsoletes: rclone {}
===> [FPM] Wrote: /tmp/package-rpm-build20160902-2618-1nu0u0d/RPMS/x86_64/rclone-1.33-2.x86_64.rpm {}
===> [FPM] Executing(%clean): /bin/sh -e /var/tmp/rpm-tmp.prcLsW {}
===> Created package: /pipeline/source/fpm-recipes/rclone/pkg/rclone-1.33-2.x86_64.rpm
```
