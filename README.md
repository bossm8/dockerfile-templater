# Dockerfile Templater

***This software is not currently considered stable and may be subject to change
until this notice is removed***

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

## Usage

### Template Functions

The templater includes [sprig](https://github.com/Masterminds/sprig) (which is also
included in [helm](https://helm.sh/docs/chart_template_guide/functions_and_pipelines/))
to extend the limited set of [go template functions](https://pkg.go.dev/text/template#hdr-Functions).

### Variants YML

Flag: `--variants.def`

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

If you do not provide an additional variants configuration file (`--variants.cfg`)
the variants file (`--variants.def`) is expected to be a valid yml file. There will
be no processing and the data read from the file is directly passed to the
Dockerfile template.

#### Templated

As mentioned, the variants can itself be templated, this enables a high
flexibility such as different combinations of package versions and reduces
configuration duplications.  When the flag `--variants.cfg` is supplied, this
file will be interpreted as a template itself and be processed with the contents
of the variants configuration before it will be fed into the Dockerfile
template.

### Variants YML Config

Flag: `--variants.cfg`

Supplying this file is optional but may help reducing configuration and duplication
in the variants itself. This file has no constraints on it's content except that
it must be a valid yml file.

### Dockerfile

Flag: `--dockerfile.tpl`

The templated Dockerfile which accepts the configuration of the variants yml. It
must be a valid go template.

#### Template Directory

Flag: `--dockerfile.tpldir`

An optional directory which contains
[includable templates](https://pkg.go.dev/text/template#hdr-Associated_templates) for your
templated Dockerfile. Files in this directory which end in `.tpl` can then be
included in your main Dockerfile template (or in the includes itself).
This flag can be used multiple times to include multiple directories.

#### Additional Variables / Variable Overrides

Flag: `--dockerfile.var`

Additional variables or variable overrides to apply before rendering a Dockerfile.
There are three different cases which may occur:

1. Override a variable on a single variant only. In this case the variable needs to be prefixed with the variants.name.
   `--dockerfile.var <VARIANT_NAME>:<KEY_PATH>=value`
2. Override a variable on all variants. Here the variant name must be omitted.
   `--dockerfile.var <KEY_PATH>=value`
3. Add a new variable. The same rules as mentioned above apply.

Notes:
 - You may add new hierarchy elements, they will be created on the fly
 - Existing key: value elements cannot be converted into a hierarchy

### Output

Flag: `--out.dir`

The generated Dockerfiles are written to the specified directory when rendered.

### Output Name Format

Flag: `--out.fmt`

Dockerfiles are written with the specified naming scheme to the output directory.
The format takes a go template string that can contain variables defined in the variants.

The default format (`Dockerfile.{{ .image.name }}.{{ .image.tag }}`) allows you to 
build the images like this for example (assuming no dots in the name/tag):

```bash
for DF in $(find dockerfiles -type f); do
    NAME=$(echo ${DF} | awk -F '.' '{print $2}')
    TAG=$(echo ${DF} | awk -F '.' '{print $3}')
    docker build . -f ${DF} -t ${NAME}:${TAG}
done
```

### Verbosity

There are two additional flags which control the verbosity of the templater:

- `--verbose`: Print debug messages
- `--debug`: Debug the handled yml structures, this may help debugging the
            yml syntax, especially if the variants file is a template itself.
            This flag must be used in conjunction with `--verbose`.

### Configuration File / Environment

As an alternative to commandline flags you may also provide the relevant flags
with either a configuration file (`--config`) or environment variables prefixed
with `DTPL` and dots replaced with underscores.

Example for `--dockerfile.tpldir`:

CLI usage:
```bash
(..) --dockerfile.tpldir some/dir --dockerfile.tpldir other/dir
```

Configuration file:
```yml
dockerfile:
    tpldir:
        - some/dir
        - other/dir
```

Environment variable:
```bash
export DTPL_DOCKERFILE_TPLDIR="some/dir other/dir"
```

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
           -variants.def variants.yml \
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
            --dockerfile.tpl Dockerfile.tpl
            --variants.def variants.yml
            --variants.cfg variants.cfg.yml
            --out.dir ${CI_PROJECT_DIR}/dockerfiles
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
                NAME=$(echo ${DF} | awk -F '.' '{print $2}')
                TAG=$(echo ${DF} | awk -F '.' '{print $3}')
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
the templater image with github workflows) contain some basic example files which
can be used with the templater.