# Protocol and evidence

[Wiki home](Home) · [Development](development)

WonKey reads and writes settings through the device's vendor HID interface.
This reference records reply layouts and the evidence behind the implementation.

[Hardware verification](#hardware-verification) · [Reply layout](#reply-layout) · [Source evidence](#evidence)

See [device selection](hardware#device-selection-and-access) for descriptor requirements and [apply safety](hardware#apply-safety-and-records) for upload framing.

## Hardware verification

The authentic user capture in `internal/xfkey/testdata/hardware-20260911` confirms identify/readback framing on this unit:
model `0112`, version `1014`, identifier `be077ba2`, <kbd>Enter</kbd> on press without modifiers, and RGB API mode 0 with channels `255,255,255`.
The four report descriptors also match. Separate user-run tests confirmed upload/commit echoes and full 128-byte readback after <kbd>F13</kbd> and steady-blue changes.
The user confirmed <kbd>F13</kbd> press/release events, steady-blue RGB, and persistence of both settings after reconnect.
Other settings and devices remain unverified.
Letters, digits, <kbd>F1</kbd> to <kbd>F12</kbd>, and <kbd>F14</kbd> to <kbd>F24</kbd> are software-supported but remain unverified on hardware.
Existing parser output and capture metadata retain the status `host-derived, hardware-unverified`.
The included query fixture does not record the later upload tests.

## Reply layout

Each reply contains exactly 64 vendor bytes, without a Linux report-ID placeholder.
The public executable has no offline parser or protocol command interface.
The internal parser and authentic fixtures retain the raw protocol evidence.

Identify has model at offsets 2 to 3, version at 4 to 5, both big-endian, and identifier at 6 to 9.
Readback copies `RX6[2:64]` to `C[0:62]`, `RX7[2:64]` to `C[62:124]`, and `RX8[2:6]` to `C[124:128]`.
Saved captures retain all raw replies, including unused RX8 bytes.
Internal parsing retains unknown RGB values, labelled `unknown`.

See [lighting](usage#lighting) for public modes and [configuration fields](usage#supported-configuration-fields) for writable offsets.

## Evidence

Sources are pinned to `cuylerstuwe/xfkey-cross-platform` commit `d86f50af54c5ce6395c41c958f8598d42e1d8a15`:

- [TECH.md descriptors and protocol](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/TECH.md). Some descriptor lengths and its identify layout are incorrect.
- [Python encoder, configuration and framing](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L326-L428).
- [Python key table](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L165-L181) maps letters, digits, Enter, and F1-F24 to the [supported HID usages](usage#supported-configuration-fields).
- [Browser encoder](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/examples/listener-web/index.html#L402-L454).
- [Vendor executable, static analysis only](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/xfp-win/xfp-win.exe), SHA-256 `0fdd932071b00064cf63b91c34d70b2be13ded06f14ce03f46c38dbd8a2b510c`.
- [Vendor database](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/xfp-win/userdata.db), five “One Key Max” rows with type 274 (`0112`).

Vendor executable evidence: big-endian model at VA `0046A989–0046A9A1`, version at `0046AB17–0046AB27`, identifier at `0046A9BD–0046AA15`, readback copies at `0046ACEF–0046AE71`, and full configuration copy at `0042E8A0–0042E8D6`.
RGB strings are at file offsets `000A6D46–000A6DA9`, indexed insertion at VA `00435234–00435618`, and assignment at `00436110–00436121`.
The authentic fixture confirms queries only.

Public names differ from the historical vendor interpretations retained by the internal parser and capture metadata:

| Public mode | Stored byte | Historical vendor interpretation | Former public name |
| --- | --- | --- | --- |
| `static` | `02` | Single-colour steady | `steady` |
| `breathe` | `03` | Single-colour flowing | `flowing` |
| `cycle-slow` | `01` | Full-colour gradient | `gradient` |
| `cycle-fast` | `05` | Neon flowing | `neon` |
| `flash` | `04` | Flash on click | unchanged |
| `held` | `07` | On while pressed, off on release | unchanged |
| `toggle` | `08` | Toggle on click | unchanged |
| `off` | `06` | Lights off | unchanged |

The public descriptions reflect user observations. The rename does not add automated hardware verification or change protocol values.
Former names are not aliases. Existing captures retain their names and remain valid restore sources.

The [USB HID Usage Tables](https://www.usb.org/sites/default/files/hut1_7.pdf) define F13-F24 on keyboard page `0x07`.
These usages fit the captured keyboard descriptor and the single-key configuration field. This confirms representation, not firmware execution.
