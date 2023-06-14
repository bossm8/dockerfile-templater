FROM golang:latest

RUN apt-get update && \
    apt-get install -y \
        git \
        vim \
        sudo \
        bash-completion && \
    useradd -ms /bin/bash -d /home/tpl tpl