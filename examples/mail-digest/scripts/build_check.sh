#!/bin/sh
# Build check: image builds take HTTPS_PROXY and the CA bundle from compose,
# empty by default, and the images build with whatever the machine provides.
set -eu

for svc in web test; do
  cfg=$(env -u HTTPS_PROXY -u CA_BUNDLE_DIR docker compose config)
  echo "$cfg" | grep -q 'HTTPS_PROXY' || { echo "FAIL: HTTPS_PROXY not wired"; exit 1; }
  echo "$cfg" | grep -q 'additional_contexts' || { echo "FAIL: CA context not wired"; exit 1; }
done
grep -q '^ARG HTTPS_PROXY' Dockerfile || { echo "FAIL: Dockerfile lacks ARG HTTPS_PROXY"; exit 1; }

echo "build with the machine's proxy and CA_BUNDLE_DIR, as provided"
docker compose build
echo "build_check ok"
