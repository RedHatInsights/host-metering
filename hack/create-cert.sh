
# Execute this script in the directory with test-cert.cnf
# Purpose: Create a self-signed certificate for testing

openssl genrsa -out test-cert.key

# To create a CSR for submitting to CA
# openssl req -new -key test-cert.key -out test-cert.csr -config test-cert.cnf

# Create a self-signed certificate
openssl req -x509 -new -key test-cert.key -out test-cert.crt -config test-cert.cnf

# Show the certificate
openssl x509 -in test-cert.crt -text -noout