#!/bin/bash

echo "Starting basicdns test..."
echo "Starting the DNS server..."
sudo go run main.go &

# Wait a bit for the server to start
sleep 2

# Save the PID of the process using port 53
DNS_SERVER_PID=$(sudo lsof -i :53 -t | head -n 1)
echo "DNS server PID: $DNS_SERVER_PID"

# Local test
echo "Testing local domain resolution:"
dig @localhost example.local A +short

# External forwarding test
echo "Testing external domain forwarding:"
dig @localhost google.com A +short

# Kill the DNS server
echo "Stopping the DNS server..."
sudo kill $DNS_SERVER_PID

echo "DNS server stopped and tests completed."
