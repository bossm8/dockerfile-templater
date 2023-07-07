# Dockerfile Templater

Simple GO utility to generate Dockerfiles from a template accepting yml data.

## Introduction

This utility helps deduplicating code by generating multiple Dockerfiles 
from a single template. Making multi-variant image builds easier and faster.

The templater is able to generate Dockerfiles in different variants from a 
[go templated](https://pkg.go.dev/text/template) Dockerfile (the templater includes
[sprig](https://github.com/Masterminds/sprig) to provide more template
functionality). The data passed to the template is read from a yml configuration
file ([variants](#variants-yml)). To make things more dynamic, the variants file
can itself be a template and accept data from an additional yml file ([variants
configuration](#variants-yml-config)).  Read below to get more information about
each file and their required format.

### Template Functions

The templater includes [sprig](https://github.com/Masterminds/sprig) (which is also
included in [helm](https://helm.sh/docs/chart_template_guide/functions_and_pipelines/))
to extend the limited set of [go template functions](https://pkg.go.dev/text/template#hdr-Functions).

### Variants YML

Flag: `-variants`

The main template configuration which will ultimately be passed to the Dockerfile
template.

It **must** contain the following structure:

```yaml
variants:
    - image:
        name: <name which you will use for the image>
        tag: <tag which you will use for the image>
      <custom content free of constraints>
```

#### Plain

If you do not provide an additional variants configuration file (`-variants.cfg`)
the variants file (`-variants`) is expected to be a valid yml file. There will
be no processing and the data read from the file is directly passed to the
Dockerfile template.

#### Templated

As mentioned, the variants can itself be templated, this enables a high
flexibility such as different combinations of package versions and reduces
configuration duplications.  When the flag `-variants.cfg` is supplied, this
file will be interpreted as a template itself and be processed with the contents
of the variants configuration before it will be fed into the Dockerfile
template.

### Variants YML Config

Flag: `-variants.cfg`

Supplying this file is optional but may help reducing configuration and duplication
in the variants itself. This file has no constraints on it's content except that
it must be a valid yml file.

### Dockerfile

Flag: `-dockerfile.tpl`

The templated Dockerfile which accepts the configuration of the variants yml. It
must be a valid go template.

## Usage

### Output Directory

Flag: `-out.dir`

The generated Dockerfiles are written to an output directory with the following
name scheme:

`Dockerfile[sep]<image.name>[sep]<image.tag>`

This scheme allows you to build the images like this for example:

```bash
for DF in $(find dockerfiles -type f); do
    NAME=$(echo ${DF} | awk -F '_-_' '{print $2}')
    TAG=$(echo ${DF} | awk -F '_-_' '{print $3}')
    docker build . -f ${DF} -t ${NAME}:${TAG}
done
```

Notes:
- `[sep]` can be specified with `-dockerfile.sep` and defaults to `_-_`
- `image.name` and `image.tag` are read from the currently processed variant 
  specified in the [variants](#variants-yml) file):


### Verbosity

There are two additional flags which control the verbosity of the templater:

- `-verbose`: Print debug messages
- `-yml.debug`: Debug the handled yml structures, this may help debugging the
                yml syntax, especially if the variants file is a template itself.
                This flag must be used in conjunction with `-verbose`.

### Docker

There are currently two flavours of the dockerized templater, `latest` and
`debug`.  Similar to the containers from
[kaniko](https://github.com/GoogleContainerTools/kaniko), the `debug` variant is
based on a busybox image and can thus be used with an interactive shell.

#### Latest

This image is built from scratch and contains the templater only. 
Example usage:

```bash
docker run -it --rm \
           --user $(id -u):$(id -g) \
           -v ${PWD}:${PWD} -w ${PWD} \
       ghcr.io/bossm8/dockerfile-templater:latest \
           -dockerfile.tpl Dockerfile.tpl \
           -variants variants.yml \
           -variants.cfg variants.cfg.yml
```

#### Debug

The debug image can be used to generate Dockerfiles in pipelines for example.
Below is an example GitLab CI/CD pipeline configuration:

```yaml
stages:
    - pre-build
    - build

generate-dockerfiles:
    stage: pre-build
    image: 
        name: ghcr.io/bossm8/dockerfile-templater:debug
        entrypoint: [""]
    script:
        - templater
            -dockerfile.tpl Dockerfile.tpl
            -variants variants.yml
            -variants.cfg variants.cfg.yml
            -out.dir ${CI_PROJECT_DIR}/dockerfiles
    artifacts:
        paths:
            - dockerfiles

build-images:
    stage: build
    image: 
        name: gcr.io/kaniko-project/executor:debug
        entrypoint: [""]
    script:
        - |
            for DF in $(find dockerfiles -type f); do
                NAME=$(echo ${DF} | awk -F '_-_' '{print $2}')
                TAG=$(echo ${DF} | awk -F '_-_' '{print $3}')
                /kaniko/executor \
                    --cleanup \
                    --dockerfile ${DF} \
                    --destination ${NAME}:${TAG}
            done
    needs:
        job: generate-dockerfiles
        artifacts: true
```

### Binary

TBD

## Examples

The `examples` directory, as well as the `pkg/docker` (which is used to build
the templater image with github workflows) contain some simple examples of files
which can be used with the templater.