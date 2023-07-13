FROM golang:latest

RUN apt-get update && \
    apt-get install -y \
        git \
        vim \
        bash-completion && \
    useradd -ms /bin/bash -d /home/tpl tpl && \
    cp /home/tpl/.bash* /home/tpl/.profile /root/ 

RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' \
        | tee /etc/apt/sources.list.d/goreleaser.list && \
    apt-get update && \
    apt-get install -y goreleaser && \
    goreleaser completion bash > /etc/bash_completion.d/goreleaser

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b $(go env GOPATH)/bin v1.53.3

COPY --from=docker:dind /usr/local/bin/docker /usr/local/bin/
COPY --from=docker/buildx-bin /buildx /usr/libexec/docker/cli-plugins/docker-buildx