#!/bin/bash
set -e

openssl genrsa -out ca.key 2048
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt -subj "/CN=Proxy CA"

mkdir -p certs_cache

chmod 700 .
chmod 600 ca.key
chmod 644 ca.crt