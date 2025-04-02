#!/bin/bash
set -e

DOMAIN=$1
CERT_DIR=$(dirname "$0")

openssl genrsa -out "$CERT_DIR/$DOMAIN.key" 2048
openssl req -new -key "$CERT_DIR/$DOMAIN.key" -out "$CERT_DIR/$DOMAIN.csr" \
  -subj "/CN=$DOMAIN" -addext "subjectAltName=DNS:$DOMAIN"

openssl x509 -req -in "$CERT_DIR/$DOMAIN.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/$DOMAIN.crt" -days 365 -extfile <(printf "subjectAltName=DNS:$DOMAIN")

rm -f "$CERT_DIR/$DOMAIN.csr"