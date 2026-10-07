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

const buildWindowsTmpl = `
Write-Output 'Starting ...'

{{- if .builder }}
& docker-buildx inspect {{ quote .builder }} *> $null
if ($LASTEXITCODE -eq 0) {
    & docker-buildx inspect --bootstrap {{ quote .builder }} *> $null
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
{{- else }}
& docker-buildx inspect --bootstrap *> $null
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
{{- end }}

{{- range .target }}
$targetName = {{ if .target }}{{ quote .target }}{{ else }}'default'{{ end }}
Write-Output "Building target: $targetName ..."
$metaTemp = [System.IO.Path]::GetTempFileName()
try {
    $buildArgs = @('build')
    {{- if $.builder }}
    $buildArgs += @('--builder', {{ quote $.builder }})
    {{- end }}
    $buildArgs += @('--progress', 'plain', '--metadata-file', $metaTemp)
    {{- if $.pull }}
    $buildArgs += '--pull'
    {{- end }}
    {{- if $.push }}
        {{- if $.outputs }}
            {{- range $.outputs }}
    $buildArgs += @('--output', {{ quote . }})
            {{- end }}
        {{- else }}
    $buildArgs += '--push'
        {{- end }}
    {{- else }}
    $buildArgs += '--load'
    {{- end }}
    {{- if $.noCache }}
    $buildArgs += '--no-cache'
    {{- end }}
    {{- if $.file }}
    $buildArgs += @('-f', {{ quote $.file }})
    {{- end }}
    {{- if .target }}
    $buildArgs += @('--target', {{ quote .target }})
    {{- end }}
    {{- range .tags }}
    $buildArgs += @('-t', {{ quote . }})
    {{- end }}
    {{- range $.buildArg }}
    $buildArgs += @('--build-arg', {{ quote . }})
    {{- end }}
    {{- range $.cacheFrom }}
    $buildArgs += @('--cache-from', {{ quote . }})
    {{- end }}
    {{- range $.cacheTo }}
    $buildArgs += @('--cache-to', {{ quote . }})
    {{- end }}
    {{- range $.labels }}
    $buildArgs += @('--label', {{ quote . }})
    {{- end }}
    {{- range $.annotation }}
    $buildArgs += @('--annotation', {{ quote . }})
    {{- end }}
    {{- range $.platforms }}
    $buildArgs += @('--platform', {{ quote . }})
    {{- end }}
    {{- range $.secrets }}
    $buildArgs += @('--secret', {{ quote . }})
    {{- end }}
    $buildArgs += {{ quote $.workDir }}

    & docker-buildx @buildArgs
    if ($LASTEXITCODE -ne 0) {
        [Console]::Error.WriteLine("Error: Build failed for target $targetName")
        exit $LASTEXITCODE
    }
    Write-Output "DPANEL_BUILD_RESULT|$targetName|$(Get-Content -LiteralPath $metaTemp -Raw)"
} finally {
    Remove-Item -LiteralPath $metaTemp -Force -ErrorAction SilentlyContinue
}
{{- end }}
`
