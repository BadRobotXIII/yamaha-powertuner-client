param([string]$Jdk = 'C:\Program Files\Android\openjdk\jdk-21.0.8')
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force build\tests | Out-Null
& (Join-Path $Jdk 'bin\javac.exe') -encoding UTF-8 -d build\tests src\local\powertuner\readonly\ReadOnlyClient.java src\local\powertuner\readonly\ResponseFormatter.java tests\ReadOnlyClientTest.java
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& (Join-Path $Jdk 'bin\java.exe') -cp build\tests local.powertuner.readonly.ReadOnlyClientTest
exit $LASTEXITCODE
