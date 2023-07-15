{{- define "dev" }}{{- end -}}
{{- template "dev" . -}}
FROM {{ .from_image }}
{{ if .dev }}
COPY --from=0 dockerfile-templater /usr/local/bin/templater
{{ else }}
COPY dockerfile-templater /usr/local/bin/templater
{{ end }}
ENTRYPOINT ["templater"]
CMD ["-h"]