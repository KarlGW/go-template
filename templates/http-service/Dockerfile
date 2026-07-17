# golang:1.26.5-alpine3.24 SHA digest.
FROM golang@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 as builder

ARG BIN="{{bin}}"
ARG OS=linux
ARG ARCH=amd64

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

WORKDIR /src/${BIN}
COPY . .

RUN CGO_ENABLED=0 GOOS=${OS} GOARCH=${ARCH} go build \
  -o build/${BIN} \
  -ldflags="-s -w" \
  -trimpath .


FROM scratch

ARG BIN="{{bin}}"
ARG PORT={{port}}

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /src/${BIN}/build/${BIN} /${BIN}

EXPOSE ${PORT}

USER ${BIN}:${BIN}

ENTRYPOINT [ "/{{bin}}" ]
