
# Execute this script in the directory with test-cert.cnf
# Purpose: Create a self-signed certificate for testing

if [ -d consumer ]; then
  rm -rf consumer
fi

mkdir consumer

openssl genrsa -out consumer/key.pem

# To create a CSR for submitting to CA
# openssl req -new -key test-cert.key -out test-cert.csr -config test-cert.cnf

# Create a self-signed certificate
openssl req -x509 -new -key consumer/key.pem -out consumer/cert.pem -config mock-cert.cnf

# Show the certificate
openssl x509 -in consumer/cert.pem -text -noout