param(
    [string]$PostgresBin = 'C:/Program Files/PostgreSQL/17/bin',
    [string]$AdminURL = 'postgres://bridge_test@127.0.0.1:55435/postgres?sslmode=disable',
    [string]$DatabaseName = ('stage2d1_roundtrip_' + [guid]::NewGuid().ToString('N'))
)
$ErrorActionPreference = 'Stop'
if ($DatabaseName -notmatch '^stage2d1_roundtrip_[a-z0-9_]+$') { throw 'Only a named Stage2D1 isolated acceptance database is permitted.' }
$stage2d1Psql = Join-Path $PostgresBin 'psql.exe'
$stage2d1MigrationRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../apps/api/migrations'))
$stage2d1Exists = & $stage2d1Psql $AdminURL -At -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_database WHERE datname='$DatabaseName'"
if ($LASTEXITCODE -ne 0 -or $stage2d1Exists.Trim() -ne '0') { throw 'Database exists or cannot be inspected; refusing to overwrite.' }
& $stage2d1Psql $AdminURL -v ON_ERROR_STOP=1 -c "CREATE DATABASE $DatabaseName"
if ($LASTEXITCODE -ne 0) { throw 'Create temporary database failed.' }
Write-Output ('Isolated synthetic database: ' + $DatabaseName)
$stage2d1URI = [System.UriBuilder]::new($AdminURL)
$stage2d1URI.Path = '/' + $DatabaseName
$stage2d1URL = $stage2d1URI.Uri.AbsoluteUri
$stage2d1Up = Get-ChildItem -LiteralPath $stage2d1MigrationRoot -Filter '*.up.sql' | Sort-Object Name
$stage2d1Down = Get-ChildItem -LiteralPath $stage2d1MigrationRoot -Filter '*.down.sql' | Where-Object { [int]$_.Name.Substring(0,6) -gt 16 } | Sort-Object Name -Descending
function Apply-Stage2D1Migration($file) {
    & $stage2d1Psql $stage2d1URL -v ON_ERROR_STOP=1 -1 -f $file.FullName | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ('Migration failed: ' + $file.Name) }
}
$stage2d1CatalogSQL = @'
SELECT 'relation:'||c.relkind::text||':'||c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
UNION ALL SELECT 'function:'||p.proname||':'||pg_get_function_identity_arguments(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'
UNION ALL SELECT 'trigger:'||c.relname||':'||t.tgname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal
UNION ALL SELECT 'constraint:'||c.relname||':'||k.conname||':'||pg_get_constraintdef(k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
UNION ALL SELECT 'column:'||c.relname||':'||a.attname||':'||format_type(a.atttypid,a.atttypmod)||':'||a.attnotnull::text FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND a.attnum>0 AND NOT a.attisdropped
UNION ALL SELECT 'type:'||t.typname||':'||t.typtype::text FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public'
ORDER BY 1
'@
foreach ($file in $stage2d1Up | Where-Object { [int]$_.Name.Substring(0,6) -le 16 }) { Apply-Stage2D1Migration $file }
$stage2d1Baseline = & $stage2d1Psql $stage2d1URL -At -v ON_ERROR_STOP=1 -c $stage2d1CatalogSQL
if ($LASTEXITCODE -ne 0) { throw 'Baseline catalog read failed.' }
foreach ($file in $stage2d1Up | Where-Object { [int]$_.Name.Substring(0,6) -gt 16 }) { Apply-Stage2D1Migration $file }
Write-Output ('UP 16 -> ' + $stage2d1Up[-1].Name.Substring(0,6) + ': PASS')
foreach ($file in $stage2d1Down) { Apply-Stage2D1Migration $file }
$stage2d1Restored = & $stage2d1Psql $stage2d1URL -At -v ON_ERROR_STOP=1 -c $stage2d1CatalogSQL
if ($LASTEXITCODE -ne 0) { throw 'Restored catalog read failed.' }
$stage2d1Residual = Compare-Object $stage2d1Baseline $stage2d1Restored
if ($stage2d1Residual) { $stage2d1Residual | Format-Table; throw 'Residual schema objects after DOWN to 16.' }
Write-Output 'DOWN latest -> 16: PASS; residual objects = 0'
foreach ($file in $stage2d1Up | Where-Object { [int]$_.Name.Substring(0,6) -gt 16 }) { Apply-Stage2D1Migration $file }
Write-Output ('RE-UP 16 -> latest: PASS; isolated database retained: ' + $DatabaseName)
