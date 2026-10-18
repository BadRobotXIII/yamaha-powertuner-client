# Yamaha Power Tuner Read-Only Clients

Independent tools for inspecting motorcycle CCU responses, based on static analysis
of Yamaha Power Tuner 2.5.0. This repository contains a native Android client and a
Go client with CLI and Fyne interfaces.

Both implementations use fixed **HTTP GET-only endpoints** and preserve original
response bytes. They do not implement map uploads, authentication/key exchange,
fuel resets, switch changes, or ECU lock changes.

## Clients

| Client | Source | Platforms | Features |
|---|---|---|---|
| Native Android | [`android-client/`](android-client/) | Android 6.0+ | Select connected Wi-Fi, inspect responses, copy output, export original bytes |
| Go CLI and library | [`golang-client/`](golang-client/) | Go-supported desktop platforms | Scriptable reads, raw export, saved snapshots, offline comparison |
| Go/Fyne GUI | [`golang-client/gui/`](golang-client/gui/) | Windows; Android arm64 build | Touch-friendly controls, response export, automatic snapshot comparison |

The Go core has no external dependencies. Fyne v2.8.2 is isolated in a separate
GUI module, so the CLI does not require Fyne or a C compiler.

## Repository layout

```text
.
├── android-client/           # Native Java Android app
│   ├── src/
│   ├── tests/                # Offline JVM tests
│   ├── build.ps1
│   └── test.ps1
└── golang-client/            # Go library and CLI module
    ├── cmd/powertuner/       # CLI entry point
    ├── testdata/             # Synthetic comparison examples
    └── gui/                  # Separate Fyne application module
        ├── main.go
        ├── build.ps1
        └── FyneApp.toml
```

## Connect and read

1. Join the CCU's Wi-Fi through your operating system's settings.
2. Open your chosen client. Default address: `http://192.168.0.1:8080`.
3. Select an operation and read a snapshot.

The native Android client binds requests to the connected Wi-Fi network you select
in the app. Go/Fyne uses normal OS routing and does not select an Android Wi-Fi
interface. No client automatically joins or discovers CCU networks.

| Operation | Endpoint | Purpose |
|---|---|---|
| `info` | `/info/` | CCU identity and version |
| `maps` | `/info/`, then `/ecu/settings/` | ECU maps, after a serial-number preflight |
| `eps` | `/eps/settings/` | EPS settings on supported vehicles |
| `monitor` | `/ecu/monitor/` | One telemetry/diagnostics snapshot |
| `model` | `/ecu/model/` | ECU model information |
| `lock` | `/ecu/lock/` | Lock/password-registration status |
| `laps` | `/ecu/lap/` | Lap history |
| `log` | `/ecu/log/?request=0` through `3` | ECU activity-log pages |

The optional EPS setting adds an EPS read to a map download. Use it only on
supported vehicles. `lock` reads status only. HTTP failures stop the operation
without attempting authentication; firmware may restrict reads while locked.

## Go CLI

Requires Go 1.23 or newer. From the repository root:

```powershell
cd golang-client
go test ./...
go build -o powertuner.exe ./cmd/powertuner
.\powertuner.exe -h
```

Read and export:

```powershell
.\powertuner.exe info
.\powertuner.exe maps
.\powertuner.exe monitor
.\powertuner.exe -page 0 log
.\powertuner.exe -format all monitor
.\powertuner.exe -format hex -part ecu maps
.\powertuner.exe -raw-out monitor.bin monitor
```

Flags precede the command. `-addr` overrides the base URL; `-timeout` overrides the
default five-second request timeout. On other platforms, build with
`go build -o powertuner ./cmd/powertuner` and run `./powertuner`.

### Response formats

| Format | Output |
|---|---|
| `json` | Pretty-printed valid JSON, preserving numeric precision; default |
| `text` | Original UTF-8 text; invalid UTF-8 is flagged |
| `binary` | Eight bits per byte, with byte offsets |
| `hex` | Hexadecimal bytes with offsets |
| `all` | JSON/text, binary, and hex together |
| `raw` | Original bytes without an added newline |

These are views of the same response, not different ECU protocols. Use `-raw-out`
for byte-exact files, especially in Windows PowerShell where redirection can
transcode binary data. Select `-part info`, `-part ecu`, or `-part eps` when
exporting raw bytes from a multi-response map capture.

### Snapshot comparison

```powershell
.\powertuner.exe -save first.json monitor
.\powertuner.exe -compare first.json -save second.json monitor
.\powertuner.exe compare first.json second.json
```

`-compare` reports differences to stderr while the new response remains on stdout.
The standalone `compare` command reads local files only, without network requests.
Try it with the included synthetic data:

```powershell
.\powertuner.exe compare testdata\monitor-before.json testdata\monitor-after.json
```

Snapshot files store source information, a UTC timestamp, and base64-encoded
response bodies so binary bytes round-trip unchanged. Comparisons report added,
removed, or changed JSON fields/array entries, plus exact byte offsets and lengths.
JSON whitespace and key order do not count as value changes. Numeric spelling is
preserved, so `1` and `1.0` are reported as different.

Comparisons require the same address, operation, EPS option, and log page. An IP
address does not identify a particular motorcycle; start a new baseline when
switching bikes. See the [Go README](golang-client/README.md) for comparison limits
and library details.

## Fyne GUI

The **Response** tab provides format selection and export. **Compare** automatically
compares each successful capture with the preceding successful capture of the same
source. Failed reads do not advance the baseline. You can also compare a saved
snapshot, reset comparison history, and save snapshots or original response bytes.

Test and build the Windows GUI from the repository root:

```powershell
cd golang-client/gui
go test -tags ci ./...
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Target windows
```

The Windows script expects MinGW64 at `C:\msys64\mingw64\bin`. Output, relative to
the repository root: `golang-client/powertuner-gui.exe`.

Build the Android arm64 APK from the GUI directory:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Target android -Sdk 'C:\Android\Sdk' -Ndk 'C:\Android\android-ndk-r27c' -Jdk 'C:\Java\jdk-21'
```

Replace these example paths with your installations. Requirements: Go, Python,
Android SDK build-tools 36.0.0, platform android-36, NDK r27c, and JDK 21. The script
uses or installs Fyne CLI v1.7.3 at `tools/fyne.exe` relative to the repository root.
It packages the Go app, compiles the final manifest with AAPT2, then aligns, signs,
and verifies the APK.

Output: `golang-client/gui/dist/ccu-readonly-fyne.apk`; package
`local.powertuner.fyne`, minimum Android API 23, target API 35. This is a separate
app from the native Java client. See the [Fyne README](golang-client/gui/README.md)
for implementation details. Tool paths in scripts and component documentation may
need adjustment after moving the repository.

## Native Android client

The Java implementation uses Android platform widgets and networking APIs, without
Gradle or third-party dependencies. From the repository root:

```powershell
cd android-client
powershell -NoProfile -ExecutionPolicy Bypass -File .\test.ps1 -Jdk 'C:\Java\jdk-21'
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Sdk 'C:\Android\Sdk' -Jdk 'C:\Java\jdk-21'
```

Replace the example paths with your installations. The build requires SDK
build-tools 36.0.0 and platform android-36. It compiles Java, creates DEX with D8,
packages resources with AAPT2, and aligns/signs the APK.

Output: `android-client/dist/ccu-readonly.apk`; package `local.powertuner.readonly`.
Transfer the APK to your device and install it using Android's APK installer.
In **CCU Read Only**, refresh/select connected Wi-Fi, choose an operation, and tap
**Read snapshot**. It supports JSON/text, binary, hex, combined views, clipboard
copy, and original-byte export. Snapshot comparison is available in Go/Fyne.

## Validation and limitations

- Automated protocol tests use local mock HTTP servers or fake connections.
  Fyne layout tests use a software driver; native Android tests run on the desktop JVM.
- Both graphical clients limit previews to 16 KiB while retaining complete responses
  for export, up to the 4 MiB body limit.
- Map values and telemetry remain raw. Vehicle-specific offsets, sensor scaling,
  and diagnostic meanings are not automatically applied.
- APKs are locally signed development builds. Compilation/signature verification
  does not establish compatibility with every phone, CCU firmware, or motorcycle.
- Build scripts contain workstation-specific defaults. Pass toolchain overrides
  where supported. `-ExecutionPolicy Bypass` applies only to that PowerShell process.
- Build artifacts, captures, toolchains, and signing keys are local files. Build
  locally unless binaries are provided separately.
