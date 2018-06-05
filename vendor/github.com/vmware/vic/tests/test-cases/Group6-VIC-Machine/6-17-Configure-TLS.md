Test 6-17 - Verify vic-machine configure TLS function
=======

# Purpose:
Verify vic-machine configure certificates

# References:
* vic-machine-linux configure -x

# Environment:
This test requires that a vSphere server is running and available. One VCH is created for the suite and used throughout by each test so that we don't have to call create & configure in each test (which would duplicate the work of 6-13-TLS).

Configure VCH w/ own CA
===
Performs a similar test to the one in create.
1) Generates a CA, self-signed
2) Runs vic-machine configure against the VCH set up in pre-test
3) Makes sure the installed certificate is the correct one

Configure VCH w/ trusted CA
===
1) Generates a CA, adds it to trust pool, just like the test in Create
2) Runs vic-machine to replace the previous cert with this one
3) Uses openssl tool to verify correct trusted certificate is in place


Configure VCH - Run Configure Without Cert Options & Ensure Certs are Unchanged
===
1) Generates a CA, installs it, verifies it's installed, as in the last test
2) Calls configure again with *no* TLS options
3) Check to make sure the installed cert from 1) is still presented


Configure VCH - Replace certificates with self-signed certificate using --no-tlsverify
===
1) Calls configure against the existing VCH with --no-tlsverify and an empty --tls-cert-path
2) Checks that a self-signed certificate is generated
3) Checks that the installed certificate is the self-signed one that we just generated
