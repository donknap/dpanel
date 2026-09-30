package buildx

const buildShellTmpl = `
set -e

echo "Starting ..."

{{- if .builder }}
if docker buildx inspect {{ quote .builder }} >/dev/null 2>&1; then
    docker buildx inspect --bootstrap {{ quote .builder }} >/dev/null
fi
{{- else }}
docker buildx inspect --bootstrap >/dev/null
{{- end }}

{{- range .target }}
TARGET_NAME={{ if .target }}{{ quote .target }}{{ else }}"default"{{ end }}
echo "Building target: $TARGET_NAME ..."

META_TEMP="$(mktemp "${TMPDIR:-/tmp}/dpanel_build_XXXXXX")"

if docker buildx build {{- if $.builder }} --builder {{ quote $.builder }} {{- end }} --progress plain --metadata-file "$META_TEMP" {{- if $.pull }} --pull {{ end -}}
    {{- if $.push }} {{- if $.outputs }} {{- range $.outputs }} --output {{ quote . }} {{ end -}} {{- else }} --push {{- end }} {{- else }} --load {{- end }}
    {{- if $.noCache }} --no-cache {{ end -}}
    {{- if $.file }} -f {{ quote $.file }} {{ end -}}
    {{- if .target }} --target {{ quote .target }} {{ end -}}
    {{- range .tags }} -t {{ quote . }} {{ end -}}
    {{- range $.buildArg }} --build-arg {{ quote . }} {{ end -}}
    {{- range $.cacheFrom }} --cache-from {{ quote . }} {{ end -}}
    {{- range $.cacheTo }} --cache-to {{ quote . }} {{ end -}}
    {{- range $.labels }} --label {{ quote . }} {{ end -}}
    {{- range $.annotation }} --annotation {{ quote . }} {{ end -}}
    {{- range $.platforms }} --platform {{ quote . }} {{ end -}}
    {{- range $.secrets }} --secret {{ quote . }} {{ end -}}
    {{ quote $.workDir }}; then

    echo "DPANEL_BUILD_RESULT|${TARGET_NAME}|$(cat "$META_TEMP")"
    rm -f "$META_TEMP"
else
    echo "Error: Build failed for target $TARGET_NAME" >&2
    rm -f "$META_TEMP"
    exit 1
fi

{{- end }}
`
