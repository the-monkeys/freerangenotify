#!/bin/bash

echo "Testing FreeRangeNotify setup..."

echo "1. Testing configuration loading..."
go run cmd/server/main.go &
SERVER_PID=$!

echo "2. Waiting for server to start..."
sleep 3

echo "3. Testing health endpoint..."
curl -s http://localhost:8080/health || echo "Health check failed"

echo "4. Testing version endpoint..."  
curl -s http://localhost:8080/version || echo "Version check failed"

echo "5. Testing API status..."
curl -s http://localhost:8080/api/v1/status || echo "API status check failed"

echo "6. Stopping server..."
kill $SERVER_PID

echo "✅ Week 1 setup testing completed!"