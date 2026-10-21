# Development environment

Facts about the machine this repository is developed on, which no file in the
code would tell you.

- **Docker Hub is rate-limited here.** Pull official images through the mirror:
  `mirror.gcr.io/library/<image>:<tag>` (e.g. `mirror.gcr.io/library/python:3.12-slim`).
  The mirror serves the same images and works on any machine.
- **Outbound network goes through a TLS-intercepting proxy** on 127.0.0.1, so a
  `docker compose build` cannot reach it (host networking is refused for bake
  builds). Image builds here install from prebuilt wheels instead: the build
  context `ca` (compose: `CA_BUNDLE_DIR`, default `./scripts/no-ca`) holds the
  proxy CA and a `wheels/` directory. On this machine it is
  `/home/user/.mail-digest-build`, set once with
  `circle local set CA_BUNDLE_DIR /home/user/.mail-digest-build` — every gate in
  every worktree picks it up, and nothing machine-specific is committed. To
  refresh the wheels: `pip download -r requirements.txt -d <dir>/wheels` on the
  host (pip there trusts the proxy).
- **The Docker daemon is running** and `docker compose` v2 is available.
- There is no real mailbox. Work against sample `.eml` files in the repository.
