#!/bin/bash

echo "Starting basicdns test..."
echo "Starting the DNS server..."
sudo go run main.go &
DNS_SERVER_PID=$!

# Wait a bit for the server to start
sleep 2

# Local test
echo "Testing local domain resolution:"
dig @127.0.0.1 example.local A +short

# External forwarding test
echo "Testing external domain forwarding:"
dig @127.0.0.1 google.com A +short

# Kill the DNS server
echo "Stopping the DNS server..."
sudo kill $DNS_SERVER_PID

echo "DNS server stopped and tests completed."
