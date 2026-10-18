param(
 [ValidateSet('windows','android')][string]$Target = 'windows',
 [string]$Sdk = 'C:\Program Files (x86)\Android\android-sdk',
 [string]$Ndk = (Join-Path $PSScriptRoot '..\..\tools\android-ndk-r27c'),
 [string]$Jdk = 'C:\Program Files\Android\openjdk\jdk-21.0.8'
)
$ErrorActionPreference='Stop'
Set-Location $PSScriptRoot
if($Target -eq 'windows') {
 $env:PATH='C:\msys64\mingw64\bin;'+$env:PATH
 $env:CC='gcc'
 go build -o ..\powertuner-gui.exe .
 if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
} else {
 # cgo splits an unquoted NDK compiler path; NTFS short paths avoid the spaces.
 $fso=New-Object -ComObject Scripting.FileSystemObject
 $env:ANDROID_HOME=$fso.GetFolder($Sdk).ShortPath
 $env:ANDROID_NDK_HOME=$fso.GetFolder((Resolve-Path -LiteralPath $Ndk).Path).ShortPath
 $env:JAVA_HOME=$fso.GetFolder($Jdk).ShortPath
 $env:PATH=$env:JAVA_HOME+'\bin;'+$env:PATH
 $packager=Join-Path $PSScriptRoot '..\..\tools\fyne.exe'
 if(!(Test-Path -LiteralPath $packager)) {
  $env:GOBIN=Split-Path -Parent $packager
  go install fyne.io/tools/cmd/fyne@v1.7.3
  if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
 }
 & $packager package -os android/arm64 -appID local.powertuner.fyne -name 'CCU Read Only Fyne' -appVersion 0.3.0 -appBuild 3
 if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
 # Compile our final manifest with SDK AAPT2. Fyne's bundled resource table
 # cannot encode usesCleartextTraffic, which this local HTTP client requires.
 $bt=Join-Path $Sdk 'build-tools\36.0.0'
 function Run([string]$exe,[string[]]$arguments){ & $exe @arguments; if($LASTEXITCODE -ne 0){throw "$exe failed: $LASTEXITCODE"} }
 New-Item -ItemType Directory -Force build\res\drawable,build\compiled,dist | Out-Null
 Copy-Item -LiteralPath Icon.png -Destination build\res\drawable\icon.png -Force
 Run (Join-Path $bt 'aapt2.exe') @('compile','build\res\drawable\icon.png','-o','build\compiled')
 Run (Join-Path $bt 'aapt2.exe') @('link','-I',(Join-Path $Sdk 'platforms\android-36\android.jar'),'--manifest','AndroidManifest.final.xml','-o','build\repacked.apk','build\compiled\drawable_icon.png.flat')
 Run 'python' @('repack.py')
 Run (Join-Path $bt 'zipalign.exe') @('-f','4','build\repacked.apk','build\aligned.apk')
 $key=Join-Path $PSScriptRoot 'build\validation.keystore'
 if(!(Test-Path -LiteralPath $key)) {
  Run (Join-Path $Jdk 'bin\keytool.exe') @('-genkeypair','-keystore',$key,'-storepass','android','-keypass','android','-alias','validation','-keyalg','RSA','-keysize','2048','-validity','3650','-dname','CN=Local Fyne CCU Validation','-noprompt')
 }
 Run (Join-Path $bt 'apksigner.bat') @('sign','--ks',$key,'--ks-key-alias','validation','--ks-pass','pass:android','--key-pass','pass:android','--out','dist\ccu-readonly-fyne.apk','build\aligned.apk')
 Run (Join-Path $bt 'apksigner.bat') @('verify','--verbose','dist\ccu-readonly-fyne.apk')
 Run (Join-Path $bt 'aapt.exe') @('dump','badging','dist\ccu-readonly-fyne.apk')
 Get-FileHash -Algorithm SHA256 -LiteralPath dist\ccu-readonly-fyne.apk
}
