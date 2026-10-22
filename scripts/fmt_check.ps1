# Fail if any Go source file under cmd, internal or clients is not gofmt-formatted.
$unformatted = @(gofmt -l cmd internal clients)
if ($unformatted.Count -gt 0) {
    Write-Output "Unformatted Go files:"
    $unformatted
    exit 1
}
