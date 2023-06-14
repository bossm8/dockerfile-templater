variants:

{{- range .images }}
  {{- $i := . }}
  {{ range .versions }}
  - image:
      name: {{ $i.name }}
      tag: {{ .version }}
    from_image: {{ $.from_image }}
    additional_packages: {{ or .additional_packages "[]" }}
  {{- end }}
{{- end }}