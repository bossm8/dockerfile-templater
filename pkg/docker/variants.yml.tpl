variants:

  - name: debug
    image:
      name: {{ .image_name }}
      tag: debug
    from_image: busybox:musl
    go_version: "{{ .go_version }}"

  - name: scratch
    image:
      name: {{ .image_name }}
      tag: latest
    from_image: scratch
    go_version: "{{ .go_version }}"