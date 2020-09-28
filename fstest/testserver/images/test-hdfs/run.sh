#!/bin/bash

echo format namenode
hdfs namenode -format test

hdfs namenode &
hdfs datanode &
exec sleep infinity
