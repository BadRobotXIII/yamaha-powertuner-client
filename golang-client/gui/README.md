# Fyne read-only client (Windows + Android)

Builds:

- Windows: `../powertuner-gui.exe`
- Android arm64: [dist/ccu-readonly-fyne.apk](dist/ccu-readonly-fyne.apk), package
  `local.powertuner.fyne`, version 0.3.0, min API 23, target 35. This is a separate
  app from the original native Java validation client. The APK is a locally signed
  debug/validation build, not a Play Store release.

Join the CCU Wi-Fi in OS settings, open the client, choose the address/endpoint and
tap **Read snapshot**. Unlike the Java Android client, this Go/Fyne client uses OS
routing and does not select/bind an Android Wi-Fi interface. No requests occur on
launch, format changes, comparison, copy, or local save. There is no automatic polling.

The Response tab shows JSON/text, binary bits, hex, or a combined view of the same
response bytes. Map reads retain separate identity/ECU/optional-EPS responses; choose
one in the response selector. Preview is limited to 16 KiB per body; **Save raw bytes**
exports the full selected body. **Save snapshot** exports all response bodies with
source and capture-start timestamp using the shared Go snapshot format.

The Compare tab automatically compares the last two **successful** captures of the
same address/endpoint/options. Failed reads keep the previous successful capture and
do not advance the baseline. It reports exact byte differences and JSON field/array
changes without float rounding. **Compare saved snapshot** loads an exported baseline
to compare against the current capture. **Reset comparisons** clears the in-memory
history, useful when changing motorcycles at the same address. All exports are local
file-picker operations, not ECU writes. History is in memory only unless exported.

Only fixed GET endpoints are available. No uploads, authentication/key exchange,
fuel clearing, lock changes, or switch changes are implemented. Defaults: 5-second
HTTP request timeout, 4 MiB body limit, no proxy or redirects. Like the original Go
client, this is raw protocol validation, without vehicle-specific value transforms.

## Rebuild

This directory is a separate Go module so the CLI/core remain dependency-free.
`go.mod`/`go.sum` pin Fyne v2.8.2 and dependencies. Build from this directory:

```powershell
go test -tags ci ./...
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Target windows
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Target android
```

Windows needs the installed MinGW64 compiler. Android uses SDK build-tools 36.0.0,
platform android-36, JDK 21 and the workspace-local NDK r27c under `../../tools`.
The script accepts `-Sdk`, `-Ndk`, and `-Jdk` overrides and uses NTFS short paths to
avoid cgo compiler-path splitting on Windows. If short filenames are disabled, use
an SDK/NDK checkout without spaces. The process-scoped execution-policy flag permits
the script without changing machine policy.

The maintained Fyne CLI v1.7.3 is installed at `../../tools/fyne.exe` and builds
the native Go code. If missing, the build script installs that pinned CLI version
into the workspace. Its
legacy binary-XML encoder cannot encode some modern Android attributes, so a
minimal bootstrap `AndroidManifest.xml` is used, followed by SDK AAPT2 packaging
with `AndroidManifest.final.xml`, then alignment and signing. `repack.py` combines
only our Fyne-built dex/native libraries/assets with our own compiled resources.
It does not modify Yamaha's APK. Final manifest allows the CCU's local cleartext
HTTP and requests only INTERNET permission. Signing keys are local validation keys
in ignored `build/`. Rebuilds reuse that key so updates can be installed normally.

Tests use a local mock HTTP server for protocol behavior and Fyne's software driver
for a portrait-layout preview (`build/portrait-preview.png`). No visible GUI,
device installation or CCU connection is needed for these checks. Desktop/Android
builds and APK signature verification do not establish runtime compatibility with
a particular phone or motorcycle. iOS packaging was not built or signed here;
Fyne's supported mobile workflow requires the corresponding platform toolchain.

Fyne references: [mobile packaging](https://docs.fyne.io/started/mobile/),
[main-goroutine updates](https://docs.fyne.io/explore/migrations/). Network work runs
in a worker goroutine and updates widgets through `fyne.Do`.
