# CCU Read Only — Android validation client

Installable APK: [dist/ccu-readonly.apk](dist/ccu-readonly.apk). Package:
`local.powertuner.readonly`, version 0.2, Android 6.0/API 23 or newer. It has its
own package identity and does not replace Yamaha Power Tuner.

The APK is signed with a local validation certificate. It is a development build,
not a Play Store release. The generated key is in ignored `build/`; it is not a
production signing credential. Signature verification and offline transport tests
passed. The app has not been installed or exercised on an Android device/emulator
or connected to a motorcycle during development.

## Use

1. Transfer the APK to your Android device and install it using the device's APK
   installer (allow installation from that source if Android asks).
2. Join the CCU Wi-Fi in Android Settings. Keep that Wi-Fi connection when Android
   reports that it has no Internet access.
3. Open **CCU Read Only**, refresh the network list, and select the connected CCU
   Wi-Fi using the displayed local IP addresses. Default URL: `http://192.168.0.1:8080`.
4. Select a read operation and tap **Read snapshot**. Choose **JSON / UTF-8 text**,
   **Binary (bits)**, **Hex**, or **JSON / Binary / Hex**. Changing formats does not
   send another request. **Copy result** copies the current display. No HTTP
   requests are made on app launch or refresh.

The client now retains the original HTTP response-body bytes before decoding.
JSON/text displays the existing UTF-8 payload without reserializing numbers;
non-JSON text remains text. Invalid UTF-8 is flagged, with hex/binary views still
available. Binary shows eight bits per byte, hex shows two digits per byte, and
both include byte offsets. These are views of the same response, not different
ECU protocols or a request to make the ECU return a different encoding.

Display previews show at most the first 16 KiB; the full response (up to 4 MiB)
remains available through **Save original bytes (.bin)**. Android's file picker
saves the exact selected response to a local/user-selected document, without
text conversion or additional storage permission. It does not write to the ECU.

Maps performs GET `/info/`, requires a nonempty `SerialNumber`, then GET
`/ecu/settings/`. The EPS checkbox adds GET `/eps/settings/`; use it only for
supported vehicles. The response selector lets you inspect/save each actual
response separately (identity, ECU maps, EPS), with ECU maps selected initially.
Earlier successful responses are retained if a later read fails. Map/EPS bodies
are no longer rejected just because they are not JSON; the identity preflight
still requires JSON with a serial number. It preserves numeric text, including model-specific map encodings, without
guessing offsets, units, or diagnostic-code meanings. Monitor is a single snapshot.

Other choices read identity, model, lock status, laps, EPS settings, or activity
log pages 0–3. **Lock status only reads status.** There is no editing, upload,
login, key exchange, fuel reset, switch change, or lock-change function.
HTTP failures stop the read without authentication or application-level retry.
Firmware may reject reads when locked; this client does not work around that.

Requests bind to the chosen Wi-Fi `Network`; there is no cellular fallback or
automatic association/discovery. A configurable HTTP(S) base address also permits
manual testing against a mock on that Wi-Fi. Connect/read timeout is five seconds;
responses are limited to 4 MiB; redirects and URL credentials/paths are rejected.
The transport requests `Connection: close` and `Accept-Encoding: identity`.
Only INTERNET and ACCESS_NETWORK_STATE permissions are used; no location, storage,
camera or device-control permission is requested. Copying uses the clipboard.

## Build and offline test

No Gradle or third-party dependencies are needed. Scripts default to the installed
SDK build-tools 36.0.0, android-36 platform and Android JDK 21. Override `-Sdk` and
`-Jdk` if their locations differ. Build artifacts are confined to this directory.

```powershell
cd android-client
powershell -NoProfile -ExecutionPolicy Bypass -File .\test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

The process-scoped execution-policy option permits these local scripts; it does
not change machine policy. Build uses javac → D8 → AAPT2 → ZIP alignment → APK
signing, then verifies signatures and prints package metadata plus SHA-256.
Tests run only on the desktop JVM with fake HTTP connections. They neither launch
the APK nor open network connections. Compilation does not establish runtime or
motorcycle compatibility.

Protocol evidence: [map upload extension](../Yamaha-Power-Tuner-map-upload-analysis.md)
and [original report](../Yamaha-Power-Tuner-static-analysis.md). The initial Go
implementation remains in `../powertuner-client/` and remains read-only.
