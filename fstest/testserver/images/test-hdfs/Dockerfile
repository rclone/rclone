# A very minimal hdfs server for integration testing rclone
FROM debian:stretch

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openjdk-8-jdk \
                                      net-tools curl python krb5-user krb5-kdc krb5-admin-server \
    && rm -rf /var/lib/apt/lists/*

ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64/

ENV HADOOP_VERSION 3.2.1
ENV HADOOP_URL https://www.apache.org/dist/hadoop/common/hadoop-$HADOOP_VERSION/hadoop-$HADOOP_VERSION.tar.gz
RUN set -x \
    && curl -fSL "$HADOOP_URL" -o /tmp/hadoop.tar.gz \
    && tar -xvf /tmp/hadoop.tar.gz -C /opt/ \
    && rm /tmp/hadoop.tar.gz*

RUN ln -s /opt/hadoop-$HADOOP_VERSION/etc/hadoop /etc/hadoop
RUN mkdir /opt/hadoop-$HADOOP_VERSION/logs

RUN mkdir /hadoop-data
RUN mkdir -p /hadoop/dfs/name
RUN mkdir -p /hadoop/dfs/data

ENV HADOOP_HOME=/opt/hadoop-$HADOOP_VERSION
ENV HADOOP_CONF_DIR=/etc/hadoop
ENV MULTIHOMED_NETWORK=1

ENV USER=root
ENV PATH $HADOOP_HOME/bin/:$PATH

ADD core-site.xml    /etc/hadoop/core-site.xml
ADD hdfs-site.xml    /etc/hadoop/hdfs-site.xml
ADD httpfs-site.xml  /etc/hadoop/httpfs-site.xml
ADD kms-site.xml     /etc/hadoop/kms-site.xml
ADD mapred-site.xml  /etc/hadoop/mapred-site.xml
ADD yarn-site.xml    /etc/hadoop/yarn-site.xml

ADD krb5.conf        /etc/
ADD kdc.conf         /etc/krb5kdc/
RUN echo '*/admin@KERBEROS.RCLONE *' > /etc/krb5kdc/kadm5.acl

ADD run.sh /run.sh
RUN chmod a+x /run.sh
CMD ["/run.sh"]
