FROM {{ .from_image }}
LABEL version {{ .image.tag }}

RUN apt-get update \
    && apt-get install -y \
        curl \
    {{- range .additional_packages }}
        {{ . }} \
    {{- end }} 
    && apt-get autoclean