{{ define "install" }}
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y \
        curl \
        nginx
{{ end }}