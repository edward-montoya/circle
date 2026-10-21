#!/bin/sh
# Smoke: the web service serves the sample Digest out of the box.
set -eu
trap 'docker compose down >/dev/null 2>&1' EXIT
docker compose up --build -d web
body=""
for _ in $(seq 1 30); do
  body=$(curl -fs "http://localhost:8080/api/digest?date=2025-03-11" 2>/dev/null) && break
  sleep 1
done
echo "$body"
echo "$body" | python3 -c 'import json,sys; sys.exit(0 if json.load(sys.stdin)["total"] > 0 else 1)'
