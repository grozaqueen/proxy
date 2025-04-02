1. sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain certs/ca.crt

2. docker build -t proxy-scanner .
   docker run -p 8080:8080 -p 8000:8000 -v $(pwd)/certs:/app/certs proxy-scanner
