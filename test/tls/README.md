# Test Certificates

Self-signed test certificates are used for testing, but they expire periodically, which causes TestServerTLS in server_test.go to fail.

```bash
% certigo dump ./test_server.crt

** CERTIFICATE 1 **
Valid: 2020-01-18 19:00 UTC to 2022-10-14 19:00 UTC
Subject:
	CN=test_server
Issuer:
	CN=test root ca
IP Addresses:
	127.0.0.1

% certigo dump ./test_client.crt

** CERTIFICATE 1 **
Valid: 2020-01-18 18:58 UTC to 2022-10-14 18:58 UTC
Subject:
	CN=test_client
Issuer:
	CN=test root ca
IP Addresses:
	127.0.0.1
```

In a terminal, `certigo` colors "2022-10-14 19:00 UTC" red because it's expired as of this writing (March 2023).
Note, however, that the root cert is still valid (until 2027):

```bash
% certigo dump ./test_root_ca.crt

** CERTIFICATE 1 **
Valid: 2017-03-23 20:45 UTC to 2027-03-23 20:45 UTC
Subject:
	CN=test root ca
Issuer:
	CN=test root ca
```

## Generate New Test Certificates

Run `generate-test-certs.sh`.

A general read on the topic is https://learn.microsoft.com/en-us/azure/application-gateway/self-signed-certificates but notice that the steps in `generate-test-certs.sh` use a slightly more complex configuration in order to set SAN IP=127.0.0.1 and generate a v3 x509 cert.
