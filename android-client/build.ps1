param(
    [string]$Sdk = 'C:\Program Files (x86)\Android\android-sdk',
    [string]$Jdk = 'C:\Program Files\Android\openjdk\jdk-21.0.8'
)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$bt = Join-Path $Sdk 'build-tools\36.0.0'
$platformJar = Join-Path $Sdk 'platforms\android-36\android.jar'
$env:JAVA_HOME = $Jdk
function Run([string]$exe, [string[]]$arguments) {
    & $exe @arguments
    if ($LASTEXITCODE -ne 0) { throw "$exe failed: $LASTEXITCODE" }
}
New-Item -ItemType Directory -Force build, build\classes, build\dex, dist | Out-Null
$sources = @(Get-ChildItem -LiteralPath src -Recurse -Filter '*.java' | ForEach-Object FullName)
Run (Join-Path $Jdk 'bin\javac.exe') (@('--release', '8', '-encoding', 'UTF-8', '-classpath', $platformJar, '-d', 'build\classes') + $sources)
Run (Join-Path $Jdk 'bin\jar.exe') @('cf', 'build\classes.jar', '-C', 'build\classes', '.')
Run (Join-Path $bt 'd8.bat') @('--lib', $platformJar, '--min-api', '23', '--output', 'build\dex', 'build\classes.jar')
Run (Join-Path $bt 'aapt2.exe') @('link', '-I', $platformJar, '--manifest', 'AndroidManifest.xml', '-o', 'build\unsigned.apk')
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = $null
for ($attempt = 0; $attempt -lt 20; $attempt++) {
    try { $zip = [System.IO.Compression.ZipFile]::Open((Join-Path $PSScriptRoot 'build\unsigned.apk'), 'Update'); break }
    catch [System.IO.IOException] { if ($attempt -eq 19) { throw }; Start-Sleep -Milliseconds 250 }
}
try { [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, (Join-Path $PSScriptRoot 'build\dex\classes.dex'), 'classes.dex') | Out-Null } finally { $zip.Dispose() }
Run (Join-Path $bt 'zipalign.exe') @('-f', '4', 'build\unsigned.apk', 'build\aligned.apk')
if (!(Test-Path -LiteralPath 'build\validation.keystore')) {
    Run (Join-Path $Jdk 'bin\keytool.exe') @('-genkeypair', '-keystore', 'build\validation.keystore', '-storepass', 'android', '-keypass', 'android', '-alias', 'validation', '-keyalg', 'RSA', '-keysize', '2048', '-validity', '3650', '-dname', 'CN=Local CCU Read Only Validation', '-noprompt')
}
Run (Join-Path $bt 'apksigner.bat') @('sign', '--ks', 'build\validation.keystore', '--ks-key-alias', 'validation', '--ks-pass', 'pass:android', '--key-pass', 'pass:android', '--out', 'dist\ccu-readonly.apk', 'build\aligned.apk')
Run (Join-Path $bt 'apksigner.bat') @('verify', '--verbose', 'dist\ccu-readonly.apk')
Run (Join-Path $bt 'aapt.exe') @('dump', 'badging', 'dist\ccu-readonly.apk')
Get-FileHash -Algorithm SHA256 -LiteralPath dist\ccu-readonly.apk
