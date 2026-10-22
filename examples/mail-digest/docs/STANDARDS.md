# Standards

- Gates run against the built image: the `test` service carries no bind mounts,
  so what passes is what ships.
- Configuration reaches containers through compose, with a default
  (`OWNER_EMAIL`, default `owner@example.com`).
- Shell scripts carry no unused variables and fail loudly (`set -eu`).
- Machine-specific values are never committed; see `docs/DEV-ENVIRONMENT.md`.
