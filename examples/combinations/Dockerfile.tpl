FROM ubuntu:latest

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y \
            curl \
            tar \
            package_a={{ .package_a_version }} \
    && curl -o package_b.tgz {{ printf "https://github.com/package_b/download/%s/package_b_%s.tar.gz" .package_b_version .package_b_version }} \
    && tar -C /usr/local/bin -xvf package_b.tgz package-b