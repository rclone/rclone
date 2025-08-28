#!/usr/bin/env bash
set -euo pipefail

# Create test TLS certificates for use with rclone.

OUT_DIR="${OUT_DIR:-./tls-test}"
CA_SUBJ="${CA_SUBJ:-/C=US/ST=Test/L=Test/O=Test Org/OU=Test Unit/CN=Test Root CA}"
SERVER_CN="${SERVER_CN:-localhost}"
CLIENT_CN="${CLIENT_CN:-Test Client}"
CLIENT_KEY_PASS="${CLIENT_KEY_PASS:-testpassword}"

CA_DAYS=${CA_DAYS:-3650}
SERVER_DAYS=${SERVER_DAYS:-825}
CLIENT_DAYS=${CLIENT_DAYS:-825}

mkdir -p "$OUT_DIR"
cd "$OUT_DIR"

# Create OpenSSL config

# CA extensions
cat > ca_openssl.cnf <<'EOF'
[ ca_ext ]
basicConstraints = critical, CA:true, pathlen:1
keyUsage = critical, keyCertSign, cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
EOF

# Server extensions (SAN includes localhost + loopback IP)
cat > server_openssl.cnf <<EOF
[ server_ext ]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = ${SERVER_CN}
IP.1 = 127.0.0.1
EOF

# Client extensions (for mTLS client auth)
cat > client_openssl.cnf <<'EOF'
[ client_ext ]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

echo "Create CA key, CSR, and self-signed CA cert"
if [ ! -f ca.key.pem ]; then
  openssl genrsa -out ca.key.pem 4096
  chmod 600 ca.key.pem
fi

openssl req -new -key ca.key.pem -subj "$CA_SUBJ" -out ca.csr.pem

openssl x509 -req -in ca.csr.pem -signkey ca.key.pem \
  -sha256 -days "$CA_DAYS" \
  -extfile ca_openssl.cnf -extensions ca_ext \
  -out ca.cert.pem

echo "Create server key (NO PASSWORD) and cert signed by CA"
openssl genrsa -out server.key.pem 2048
chmod 600 server.key.pem

openssl req -new -key server.key.pem -subj "/CN=${SERVER_CN}" -out server.csr.pem

openssl x509 -req -in server.csr.pem \
  -CA ca.cert.pem -CAkey ca.key.pem -CAcreateserial \
  -out server.cert.pem -days "$SERVER_DAYS" -sha256 \
  -extfile server_openssl.cnf -extensions server_ext

echo "Create client key (PASSWORD-PROTECTED), CSR, and cert"
openssl genrsa -aes256 -passout pass:"$CLIENT_KEY_PASS" -out client.key.pem 2048
chmod 600 client.key.pem

openssl req -new -key client.key.pem -passin pass:"$CLIENT_KEY_PASS" \
  -subj "/CN=${CLIENT_CN}" -out client.csr.pem

openssl x509 -req -in client.csr.pem \
  -CA ca.cert.pem -CAkey ca.key.pem -CAcreateserial \
  -out client.cert.pem -days "$CLIENT_DAYS" -sha256 \
  -extfile client_openssl.cnf -extensions client_ext

echo "Verify chain"
openssl verify -CAfile ca.cert.pem server.cert.pem client.cert.pem

echo "Done"

echo
echo "Summary"
echo "-------"
printf "%-22s %s\n" \
  "CA key:" "ca.key.pem" \
  "CA cert:" "ca.cert.pem" \
  "Server key:" "server.key.pem (no password)" \
  "Server CSR:" "server.csr.pem" \
  "Server cert:" "server.cert.pem (SAN: ${SERVER_CN}, 127.0.0.1)" \
  "Client key:" "client.key.pem (encrypted)" \
  "Client CSR:" "client.csr.pem" \
  "Client cert:" "client.cert.pem" \
  "Client key password:" "$CLIENT_KEY_PASS"

echo
echo "Test rclone server"
echo
echo "rclone serve http -vv --addr :8080 --cert ${OUT_DIR}/server.cert.pem --key ${OUT_DIR}/server.key.pem --client-ca ${OUT_DIR}/ca.cert.pem ."

echo
echo "Test rclone client"
echo
echo "rclone lsf :http: --http-url 'https://localhost:8080' --ca-cert ${OUT_DIR}/ca.cert.pem --client-cert ${OUT_DIR}/client.cert.pem --client-key ${OUT_DIR}/client.key.pem --client-pass \$(rclone obscure $CLIENT_KEY_PASS)"
echo
