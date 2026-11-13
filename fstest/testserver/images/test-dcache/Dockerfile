FROM maven:3.9.9-eclipse-temurin-17@sha256:f58d59b6273e785ac0a4477f6e9b5ba1d7731c75b906c0f7b34076f1851318cc

RUN apt-get update && apt-get install -y --no-install-recommends git openssl patch procps hostname \
    && rm -rf /var/lib/apt/lists/* \
    && (getent group 1000 >/dev/null || groupadd -g 1000 fixture) \
    && useradd -m -u 10001 -g 1000 rclone \
    && install -d -o rclone -g 1000 /opt/dcache-source

COPY entrypoint.sh /usr/local/bin/rclone-dcache-test
ENV HOME=/home/rclone MAVEN_CONFIG=/home/rclone/.m2 \
    MAVEN_OPTS="-Xmx2g -XX:ActiveProcessorCount=2" LANG=C.UTF-8
USER rclone
WORKDIR /opt/dcache-source

RUN git clone --depth 1 --branch 11.2.11 https://github.com/dCache/dcache.git . \
    && test "$(git rev-parse HEAD)" = b68980db4e674f2e65b40970cbe32586bc32d197 \
    && mvn -B package -am -pl packages/system-test -DskipTests \
    && sed -i 's/^dcache.net.listen=127.0.0.1$/dcache.net.listen=0.0.0.0\nwebdav.redirect.on-read=true\nwebdav.redirect.on-write=true/' \
       packages/system-test/target/dcache/etc/layouts/system-test.conf

ENTRYPOINT ["bash", "/usr/local/bin/rclone-dcache-test"]
