variants:

{{- range .images }}
  {{- $i := . }}
  {{ range .versions }}
  - name: "{{ $i.name }}-{{ .version }}"
    image:
      name: {{ $i.name }}
      tag: {{ .version }}
    from_image: {{ $.from_image }}
    additional_packages: {{ or .additional_packages "[]" }}
  {{- end }}
{{- end }}