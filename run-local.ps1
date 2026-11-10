$ErrorActionPreference = 'Stop'
$config = Get-Content -LiteralPath (Join-Path $PSScriptRoot '.local.json') -Raw | ConvertFrom-Json
$previous = @{}
try {
    foreach ($property in $config.PSObject.Properties) {
        $previous[$property.Name] = [Environment]::GetEnvironmentVariable($property.Name, 'Process')
        [Environment]::SetEnvironmentVariable($property.Name, [string]$property.Value, 'Process')
    }
    Push-Location (Join-Path $PSScriptRoot 'backend')
    try { go run ./cmd/api } finally { Pop-Location }
} finally {
    foreach ($name in $previous.Keys) { [Environment]::SetEnvironmentVariable($name, $previous[$name], 'Process') }
}
