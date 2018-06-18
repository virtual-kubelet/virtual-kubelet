# scripts


Integration image MUST be rebuilt after updates to scripts in this directory to be included in CI 


### ca-test.sh

Generate a root CA and a server certificate

### intermediate-ca-test.sh

Generate a root CA, intermediate CA, and a server certificate

### ubuntu-install-ca.sh

Installs the generated root CA into the system root store

### ubuntu-install-intermediate-ca.sh

Installs the generated intermediate CA into the system root store

### ubuntu-remove-ca.sh

Removes the generated root CA from the system root store

### ubuntu-remove-intermediate-ca.sh

Removes the generated intermediate CA from the system root store
