package buildx

import (
	"strconv"
	"text/template"
)

const buildShellTmpl = `
set -e
echo "Starting ..."

{{- if .Push }}
{{- range .RegistryAuth }}
echo {{ quote .Password }} | docker login {{ quote .ServerAddress }} -u {{ quote .Username }} --password-stdin
{{- end }}
{{- end }}

CONTEXT_NAME="{{.Name}}"
BUILDER_NAME="$CONTEXT_NAME-builder"

if docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
    docker buildx inspect --bootstrap >/dev/null 2>&1
fi

docker buildx build --builder "$BUILDER_NAME" --progress plain --metadata-file {{ quote (print .WorkDir "/meta.json") }} {{- if .Pull }} --pull {{ end -}}
    {{- if .Push }} {{- if .Outputs }} {{- range .Outputs }} --output {{ quote . }} {{ end -}} {{- else }} --push {{- end }} {{- else }} --load {{- end }}
    {{- if .NoCache }} --no-cache {{ end -}}
    {{- if .File }} -f {{ quote .File }} {{ end -}}
    {{- if .Target }} --target {{ quote .Target }} {{ end -}}
    {{- range .Tags }} -t {{ quote . }} {{ end -}}
    {{- range .BuildArg }} --build-arg {{ quote . }} {{ end -}}
    {{- range .CacheFrom }} --cache-from {{ quote . }} {{ end -}}
    {{- range .CacheTo }} --cache-to {{ quote . }} {{ end -}}
    {{- range .Labels }} --label {{ quote . }} {{ end -}}
    {{- range .Annotation }} --annotation {{ quote . }} {{ end -}}
    {{- range .Platforms }} --platform {{ quote . }} {{ end -}}
    {{- range .Secrets }} --secret {{ quote . }} {{ end -}}
    {{ quote .WorkDir }}
`

var buildShellFunc = template.FuncMap{
	"quote": func(s string) string {
		return strconv.Quote(s)
	},
}
