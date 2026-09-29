package context

const shellCommandTmpl = `
{{define "create"}}
set -e
docker context create {{quote .Name}} --description {{quote .Description}}
{{end}}
{{define "remove"}}
set -e
CONTEXTS="$(docker context ls --format '{{"{{.Name}}"}}')"
if printf '%s\n' "$CONTEXTS" | grep -Fxq {{quote .Name}}; then
    docker context rm {{quote .Name}}{{if .Force}} --force{{end}}
fi
CONTEXTS="$(docker context ls --format '{{"{{.Name}}"}}')"
if printf '%s\n' "$CONTEXTS" | grep -Fxq {{quote .Name}}; then
    echo 'Docker context still exists after removal' >&2
    exit 1
fi
{{end}}
`

const windowsCommandTmpl = `
{{define "create"}}
docker context create {{quote .Name}} --description {{quote .Description}}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
{{end}}
{{define "remove"}}
$contexts = @(docker context ls --format '{{"{{.Name}}"}}')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if ($contexts -contains {{quote .Name}}) {
    docker context rm {{quote .Name}}{{if .Force}} --force{{end}}
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
$contexts = @(docker context ls --format '{{"{{.Name}}"}}')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if ($contexts -contains {{quote .Name}}) {
    Write-Error 'Docker context still exists after removal'
    exit 1
}
{{end}}
`
