{{ define "dev" -}}
FROM golang:{{ .go_version }}

ENV VERSION=dev

RUN apt-get update && \
    apt-get upgrade -y

ENV CGO_ENABLED=0

WORKDIR ${GOPATH}/src/
COPY . . 

RUN go get -d -v
RUN go build -o /dockerfile-templater -ldflags="-X github.com/bossm8/dockerfile-templater/cmd.version=${VERSION}"

# ---
{{ end }}