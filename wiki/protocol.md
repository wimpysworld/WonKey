# Protocol and evidence

[Wiki home](Home) · [Development](development)

WonKey reads and writes settings through the device's vendor HID interface.
This reference records reply layouts and the evidence behind the implementation.

[Hardware verification](#hardware-verification) · [Reply layout](#reply-layout) · [Action layouts](#action-layouts) · [Source evidence](#evidence)

See [device selection](hardware#device-selection-and-access) for descriptor requirements and [apply safety](hardware#apply-safety-and-records) for upload framing.

## Hardware verification

The authentic user capture in `internal/xfkey/testdata/hardware-20260911` confirms identify/readback framing on this unit:
model `0112`, version `1014`, identifier `be077ba2`, <kbd>Enter</kbd> on press without modifiers, and RGB API mode 0 with channels `255,255,255`.
The four report descriptors also match. Separate user-run tests confirmed upload/commit echoes and full 128-byte readback after <kbd>F13</kbd> and steady-blue changes.
The user confirmed <kbd>F13</kbd> press/release events, steady-blue RGB, and persistence of both settings after reconnect.
Other settings and devices remain unverified.
All other selectable keys, including the added keyboard usages through `0x91`, and all mouse, consumer media, and multi-key actions remain hardware-unverified.
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

## Action layouts

The 128-byte configuration uses byte 0 as an action type. These layouts apply only to model `0112` with the exact checked descriptors.

| Type | Active bytes | Validation |
|---|---|---|
| `00` keyboard | 1 trigger (`01` press, `02` release, `03` both), 2 left-modifier mask (`0x00` to `0x0f`), 3 count (`01`), 4 keyboard-page usage | Defined keyboard usages `0x04` to `0x91`; right modifiers are unsupported. |
| `01` relative mouse | 1 button bitmap (left `01`, right `02`, middle `04`), 2 signed X, 3 signed Y, 4 signed wheel | Button mask `0..7`; X, Y, and wheel each `-127..127` (signed byte). |
| `02` consumer media | 1 low usage byte, 2 high usage byte | Consumer-page usage `0x0001..0x023c`; little-endian bytes, for example volume up `0x00e9` is `e9 00`. |
| `03` multi-key | 1 to 2 interval in milliseconds (big-endian), 3 repeat count, 4 key count, 5 onward keyboard-page usage bytes | Interval `1..65535`, repeat `1..255`, count `1..115`; each key uses the supported keyboard name table. |

The multi-key count stops at 115 because keys occupy bytes 5 to 119. WonKey preserves bytes 120 to 123 and RGB bytes 124 to 127.
Only active fields change. On a type change, WonKey clears the previous active span. On a shorter same-type action, it clears retired active bytes.
It preserves unknown bytes outside those spans. Keyboard trigger `both` can involve separate release fields, so conversion to or from another type is rejected.
Unsupported special (`04`), macro (`06`), touch (`07`), unknown types, and malformed known actions fail closed, including RGB-only changes and restore.
The public command is not a raw packet writer. See [action syntax](usage#mouse-media-and-multi-key-actions).

## Evidence

Sources are pinned to `cuylerstuwe/xfkey-cross-platform` commit `d86f50af54c5ce6395c41c958f8598d42e1d8a15`:

- [TECH.md descriptors and protocol](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/TECH.md). Some descriptor lengths and its identify layout are incorrect.
- [Python encoder, configuration and framing](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L284-L323) documents mouse, media, and multi-key bytes. The [upload code](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L326-L428) documents framing.
- [Python key table](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L165-L181) supplies a subset of the [supported HID usages](usage#supported-key-names), including letters, digits, Enter, and F1-F24.
- [Browser encoder](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/examples/listener-web/index.html#L402-L454) also stores media low byte first.
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

The [USB HID Usage Tables](https://www.usb.org/sites/default/files/hut1_7.pdf), section 10, define keyboard/keypad usages on page `0x07`.
WonKey accepts the defined key usages from `0x04` through `0x91`, excluding the preceding no-event and error values.
The captured keyboard descriptor has a usage maximum of `0x91` and a logical maximum of `0xff`.
The accepted usages fit the existing single-key configuration field.
This confirms representation, not firmware execution. WonKey now accepts consumer-page media, but not right-hand modifiers.
The vendor's pre-swapped integer constants represent bytes in the opposite display order. The pinned Python and browser encoders write the low usage byte first, so `0x00e9` stores `e9 00`, not `00 e9`.
The [HID Usage Tables](https://www.usb.org/sites/default/files/hut1_7.pdf), consumer page `0x0c`, define media usages. The upper bound `0x023c` follows the upstream supported range, not a claim that every value has a device effect.
