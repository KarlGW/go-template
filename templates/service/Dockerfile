# golang:1.25.5-alpine3.23 SHA1 digest.
FROM golang@sha256:ac09a5f469f307e5da71e766b0bd59c9c49ea460a528cc3e6686513d64a6f1fb as builder

ARG BIN

RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates

ENV USER=${BIN}
ENV UID=10001

RUN adduser \
  --disabled-password \
  --gecos "" \
  --home "/nohome" \
  --no-create-home \
  --shell "/sbin/nologin" \
  --uid "${UID}" \
  "${USER}"


FROM scratch

ARG BIN
ARG PORT

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY ./build/${BIN} /${BIN}

EXPOSE ${PORT}

USER ${BIN}:${BIN}

ENTRYPOINT [ "/" ]
