# Read-only Yamaha Power Tuner client

Go 1.23+ CLI and reusable package, with no external dependencies in the core module.
A separate [Fyne GUI module](gui/README.md) provides Windows and Android builds,
mobile-friendly controls, and automatic comparison of successive snapshots. Based on
the adjacent static-analysis report for Android Power Tuner 2.5.0. Hardware
compatibility has not been tested. Tests use a local HTTP mock only.

From this directory:

```powershell
go test ./...
go build -o powertuner.exe ./cmd/powertuner
.\powertuner.exe -h
```

When you choose to use it with the CCU, join its Wi-Fi through your OS first.
The default base URL is `http://192.168.0.1:8080`. The client uses normal OS routing;
it does not discover, join, or select Wi-Fi interfaces like the Android app.

```powershell
.\powertuner.exe info
.\powertuner.exe maps > maps.json
.\powertuner.exe -eps maps > maps-with-eps.json
.\powertuner.exe monitor
.\powertuner.exe lock
.\powertuner.exe laps
.\powertuner.exe -page 0 log
.\powertuner.exe -addr http://127.0.0.1:8080 -timeout 2s info
.\powertuner.exe -format all monitor
.\powertuner.exe -format hex -part ecu maps
.\powertuner.exe -raw-out monitor.bin -save first.json monitor
.\powertuner.exe -compare first.json -save second.json monitor
.\powertuner.exe compare first.json second.json
```

The last command compares two local snapshot files **without network access**.
An offline example with synthetic values is included:

```powershell
.\powertuner.exe compare testdata\monitor-before.json testdata\monitor-after.json
```

Formats: `json` (default, pretty-prints valid JSON), `text` (original UTF-8),
`binary` (eight bits per byte), `hex`, `all` (JSON/text + binary + hex), and `raw`
(unmodified bytes without an appended newline). These are representations of the
same HTTP body, not requests for alternate ECU encodings. Non-UTF-8 text is flagged;
binary/hex and raw export remain available. No model-specific values are converted.

Use `-raw-out file.bin` for reliable byte-exact local files, especially in Windows
PowerShell where native-command output redirection may transcode bytes. For a map
capture, choose `-part ecu`, `-part info`, or `-part eps` for raw export. `-save`
writes a structured snapshot with source, UTC timestamp and base64 response bodies,
so binary and invalid UTF-8 round-trip. It is not an ECU payload. Snapshot files
contain the captured data/identity and should be shared accordingly.

`-compare previous.json` reads a new capture and writes its comparison report to
stderr, leaving the selected data representation on stdout. JSON changes use
JSON-pointer paths (array indices included), with added/removed/changed values.
Object-key order and whitespace affect the byte comparison but not the value
comparison; numeric spelling is preserved (`1.0` and `1` are reported as different).
Large JSON integers retain precision. Binary data is compared by byte offset, with
length changes and missing bytes reported. Reports show the first 32 byte offsets
and first 200 JSON changes, while counting all differences. Offsets are positional,
not an insertion/deletion alignment algorithm.

Comparison requires the same source address, command, EPS option and log page.
This prevents accidental cross-endpoint comparisons, but cannot identify a different
motorcycle later using the same IP address. The Fyne GUI provides Reset comparisons.
Neither timestamps nor file metadata are treated as telemetry changes.

`maps` performs GET `/info/`, requires `SerialNumber`, then GET `/ecu/settings/`.
`-eps` adds GET `/eps/settings/` for vehicles supporting it. Default JSON output bundles
the original JSON under `info`, `ecu`, and optionally `eps`. It downloads all
returned maps, without changing numeric values. Each original response is retained
separately in a saved snapshot. Non-JSON map/EPS bodies can be inspected as bytes;
the identity preflight still requires JSON with a serial number. No year-specific offsets,
sentinel substitutions, axis conversions, or assumed units are applied.

Other commands issue a single GET: `info`, `eps`, `monitor`, `model`, `lock`,
`laps`, or `log` (page 0–3). JSON is pretty-printed without float conversion;
non-JSON responses are printed as received, with a final newline if needed.
Monitor is one snapshot, with no polling loop. Diagnostics remain the raw
`co_diag_data` value; no code meanings are guessed.

The client exposes only fixed GET endpoints. It does not implement map uploads,
fuel clearing, switch changes, lock changes, authentication, password changes,
key exchange, or binary recorded-data decoding. HTTP failures stop the command;
authentication requirements are reported without attempting a login. In
particular, `lock` reads status only. GET-only describes client requests; static
analysis cannot establish all firmware-side effects of those requests.

Requests have a five-second timeout by default, no redirects, no environment
proxy, no retries, and a 4 MiB response limit. Ctrl+C cancels pending requests.
No network request occurs for help, building, or creating a package client.
Flags must appear before the command. Shell redirection saves output locally.
