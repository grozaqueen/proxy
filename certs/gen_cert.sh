#!/bin/bash
DOMAIN=$1
openssl genrsa -out $DOMAIN.key 2048
openssl req -new -key $DOMAIN.key -out $DOMAIN.csr -subj "/CN=$DOMAIN"
openssl x509 -req -in $DOMAIN.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out $DOMAIN.crt -days 365 -sha256
rm $DOMAIN.csr