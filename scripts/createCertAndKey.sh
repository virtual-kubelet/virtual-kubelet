#!/bin/bash
# Generate cert and key for chart
openssl req -newkey rsa:4096 -new -nodes -x509 -days 3650 -keyout key.pem -out cert.pem -subj "/C=US/ST=CA/L=virtualkubelet/O=virtualkubelet/OU=virtualkubelet/CN=virtualkubelet"
if [[ "$OSTYPE" == "darwin"* ]]; then
    cert=$(base64 cert.pem)
    key=$(base64 key.pem)
else
    cert=$(base64 cert.pem -w0)
    key=$(base64 key.pem -w0)
fi
