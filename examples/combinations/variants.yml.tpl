variants:

  {{ range $a := $.package_a.versions }}
    {{ range $b := $.package_b.versions }}
      - name: "{{ $a }}-{{ $b }}"
        image:
          name: combinations
          tag: {{ printf "a-%s-b-%s" $a $b }}
        package_a_version: "{{ $a }}"
        package_b_version: "{{ $b }}"
    {{ end }}
  {{ end }}