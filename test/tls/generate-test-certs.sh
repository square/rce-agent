#!/bin/bash

set -eux

generate() {
  side=$1

  rm test_${side}_* || true

  # Client|server key
  openssl ecparam -out test_${side}.key -name prime256v1 -genkey

  # Client|server CSR
  openssl req -new -sha256 -key test_${side}.key -out test_${side}.csr -config config.ini

  # Client|server certificate signed with test root CA
  openssl x509 -req -in test_${side}.csr -CA  test_root_ca.crt -CAkey test_root_ca.key -CAcreateserial -out test_${side}.crt -days 730 -sha256 -extensions v3_ca -extfile config.ini

  # Verify new test cert
  certigo dump test_${side}.crt

  rm test_${side}.csr
}

generate "client"
generate "server"

rm test_root_ca.srl || true
