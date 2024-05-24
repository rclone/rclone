#!/usr/bin/env bash

KERBEROS=${KERBEROS-"false"}

if [ $KERBEROS = "true" ]; then
    echo prepare kerberos
    ADMIN_PASSWORD="kerberos"
    USER_PASSWORD="user"

    echo -e "$ADMIN_PASSWORD\n$ADMIN_PASSWORD" | kdb5_util -r "KERBEROS.RCLONE" create -s
    echo -e "$ADMIN_PASSWORD\n$ADMIN_PASSWORD" | kadmin.local -q "addprinc hadoop/admin"
    echo -e "$USER_PASSWORD\n$USER_PASSWORD"   | kadmin.local -q "addprinc user"
    kadmin.local -q 'addprinc -randkey hdfs/localhost'
    kadmin.local -q 'addprinc -randkey hdfs/rclone-hdfs'
    kadmin.local -q 'addprinc -randkey HTTP/localhost'
    kadmin.local -p hadoop/admin -q "ktadd -k /etc/hadoop/kerberos.key hdfs/localhost hdfs/rclone-hdfs HTTP/localhost"
    service krb5-kdc restart
    echo -e "$USER_PASSWORD\n" | kinit user
    klist
    echo kerberos ready
else
    echo drop kerberos from configuration files
    sed -i '/KERBEROS BEGIN/,/KERBEROS END/d' /etc/hadoop/core-site.xml
    sed -i '/KERBEROS BEGIN/,/KERBEROS END/d' /etc/hadoop/hdfs-site.xml
fi


echo format namenode
hdfs namenode -format test

hdfs namenode &
hdfs datanode &
exec sleep infinity
