#!/bin/sh
# Build check: image builds take HTTPS_PROXY and the CA bundle from compose,
# empty by default, and the images build with whatever the machine provides.
# The test service runs the built image, forwarding OWNER_EMAIL, with no bind
# mounts; docs/STANDARDS.md is registered as standards knowledge.
set -eu

cfg=$(env -u HTTPS_PROXY -u CA_BUNDLE_DIR -u OWNER_EMAIL docker compose config)
echo "$cfg" | grep -q 'HTTPS_PROXY' || { echo "FAIL: HTTPS_PROXY not wired"; exit 1; }
echo "$cfg" | grep -q 'additional_contexts' || { echo "FAIL: CA context not wired"; exit 1; }
grep -q '^ARG HTTPS_PROXY' Dockerfile || { echo "FAIL: Dockerfile lacks ARG HTTPS_PROXY"; exit 1; }

test_svc=$(echo "$cfg" | awk '/^  test:/{f=1;next} /^  [a-z]/{f=0} f')
echo "$test_svc" | grep -q 'OWNER_EMAIL: owner@example.com' || { echo "FAIL: test lacks OWNER_EMAIL default"; exit 1; }
echo "$test_svc" | grep -q 'volumes:' && { echo "FAIL: test service has bind mounts"; exit 1; }
OWNER_EMAIL=me@x.org docker compose config | grep -q 'OWNER_EMAIL: me@x.org' || { echo "FAIL: OWNER_EMAIL not forwarded"; exit 1; }
web_svc=$(echo "$cfg" | awk '/^  web:/{f=1;next} /^  [a-z]/{f=0} f')
echo "$web_svc" | grep -q 'OWNER_EMAIL: owner@example.com' || { echo "FAIL: web lacks OWNER_EMAIL default"; exit 1; }
OWNER_EMAIL=me@x.org docker compose config web | grep -q 'OWNER_EMAIL: me@x.org' || { echo "FAIL: web does not forward OWNER_EMAIL"; exit 1; }
[ -f docs/STANDARDS.md ] || { echo "FAIL: docs/STANDARDS.md missing"; exit 1; }
grep -q 'STANDARDS.md' .circle/project.toml || { echo "FAIL: STANDARDS.md not registered"; exit 1; }

[ -z "$(git ls-files | grep -E '\.py[co]$|(^|/)__pycache__/')" ] || { echo "FAIL: bytecode is tracked"; exit 1; }
for p in __pycache__/ '*.pyc' .env; do
  grep -qxF "$p" .gitignore 2>/dev/null || { echo "FAIL: .gitignore lacks $p"; exit 1; }
done

echo "build with the machine's proxy and CA_BUNDLE_DIR, as provided"
docker compose build
echo "build_check ok"
