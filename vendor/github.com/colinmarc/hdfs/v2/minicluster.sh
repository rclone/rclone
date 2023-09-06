#!/bin/sh

HADOOP_HOME=${HADOOP_HOME-"/tmp/hadoop"}
NN_PORT=${NN_PORT-"9000"}
HADOOP_NAMENODE="localhost:$NN_PORT"

if [ ! -d "$HADOOP_HOME" ]; then
  mkdir -p $HADOOP_HOME

  echo "Downloading latest CDH to ${HADOOP_HOME}/hadoop.tar.gz"
  curl -o ${HADOOP_HOME}/hadoop.tar.gz -L http://archive.cloudera.com/cdh5/cdh/5/hadoop-latest.tar.gz

  echo "Extracting ${HADOOP_HOME}/hadoop.tar.gz into $HADOOP_HOME"
  tar zxf ${HADOOP_HOME}/hadoop.tar.gz --strip-components 1 -C $HADOOP_HOME
fi

MINICLUSTER_JAR=$(find $HADOOP_HOME -name "hadoop-mapreduce-client-jobclient*.jar" | grep -v tests | grep -v sources | head -1)
if [ ! -f "$MINICLUSTER_JAR" ]; then
  echo "Couldn't find minicluster jar!"
  exit 1
fi

echo "Starting minicluster..."
$HADOOP_HOME/bin/hadoop jar $MINICLUSTER_JAR minicluster -nnport $NN_PORT -datanodes 3 -nomr -format "$@" > minicluster.log 2>&1 &

export HADOOP_CONF_DIR=$(mktemp -d)
cat > $HADOOP_CONF_DIR/core-site.xml <<EOF
<configuration>
  <property>
    <name>fs.defaultFS</name>
    <value>hdfs://$HADOOP_NAMENODE</value>
  </property>
</configuration>
EOF

echo "Waiting for namenode to start up..."
$HADOOP_HOME/bin/hdfs dfsadmin -safemode wait

export HADOOP_CONF_DIR=$(mktemp -d)
cat > $HADOOP_CONF_DIR/core-site.xml <<EOF
<configuration>
  <property>
    <name>fs.defaultFS</name>
    <value>hdfs://$HADOOP_NAMENODE</value>
  </property>
</configuration>
EOF

export HADOOP_FS="$HADOOP_HOME/bin/hadoop fs"
./fixtures.sh

echo "Please run the following commands:"
echo "export HADOOP_CONF_DIR='$HADOOP_CONF_DIR'"
echo "export HADOOP_FS='$HADOOP_HOME/bin/hadoop fs'"
