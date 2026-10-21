FROM mirror.gcr.io/library/python:3.12-slim AS base
ARG HTTPS_PROXY=""
COPY --from=ca . /etc/build-ca/
WORKDIR /srv
COPY requirements.txt ./
# Behind a proxy that a build cannot reach, wheels fetched on the host sit next
# to the CA bundle in the "ca" context; elsewhere pip uses the index.
RUN if [ -f /etc/build-ca/ca-bundle.crt ]; then export PIP_CERT=/etc/build-ca/ca-bundle.crt; fi \
 && if [ -d /etc/build-ca/wheels ]; then \
      pip install --no-cache-dir --no-index --find-links /etc/build-ca/wheels -r requirements.txt; \
    else \
      pip install --no-cache-dir -r requirements.txt; \
    fi
FROM base AS app
COPY app ./app
COPY tests ./tests
COPY samples ./samples
ENV MAIL_DIR=/srv/samples/mail
EXPOSE 8080
CMD ["python", "-m", "app.server"]

FROM app AS test
COPY requirements-test.txt ./
RUN if [ -f /etc/build-ca/ca-bundle.crt ]; then export PIP_CERT=/etc/build-ca/ca-bundle.crt; fi \
 && if [ -d /etc/build-ca/wheels ]; then \
      pip install --no-cache-dir --no-index --find-links /etc/build-ca/wheels -r requirements-test.txt; \
    else \
      pip install --no-cache-dir -r requirements-test.txt; \
    fi

FROM app AS web
