# Hardware operations and safety

[Wiki home](Home) · [Usage and configuration](usage)

`key [CONTROL] <COMBINATION>`, `trigger <press|release|both>`, `macro [CONTROL] <STROKE[,STROKE...]>`, `mouse [CONTROL] <BUTTONS>`, `media [CONTROL] <ACTION|0xHHHH>`, and `rgb <MODE> [RGB]` change settings on an XFKEY One Key Max (`0112`) after confirmation (`[CONTROL]`, `--layer <1..15>`, and `led <MODE>` are for CH57 `1189:8890` only; `[CONTROL]` and `--layer` are rejected on `0112`).
Bare `wonkey` reads all compatible XFKEY One Key Max (`0112`) and CH57 (`1189:8890`) devices without modifying them.

[Device access](#device-selection-and-access) · [Hardware checks](#check-hardware-effects-and-persistence) · [Apply safety](#apply-safety)

## Device selection and access

Your user needs access to the selected XFKEY vendor hidraw node or the CH57 USB device node under `/dev/bus/usb`.
WonKey does not grant permissions or install permission rules.
Ask an administrator to configure access for the verified device.
Do not guess a device from its hidraw or USB bus number.

WonKey enumerates both supported device families automatically:

- **XFKEY One Key Max (`0112`)**: Requires VID/PID `af88:6688`, revision `0100`, and exact USB/configuration bytes. All four report descriptors must match (62, 114, 25, and 34 bytes). The device must have interfaces 0 to 3 and one vendor hidraw mapping on interface 3 (usage page `FF00`, usage `01`, endpoints `84`/`04`, 64-byte payloads, and no report IDs). The shared serial `XFKEY` is not unique.
- **CH57 (`1189:8890`)**: Requires VID/PID `1189:8890` and the exact published single-configuration USB descriptor structure across interfaces 0 to 3. Bare `wonkey` verifies the 36-byte report ID 3 descriptor on interface 1, issues one Input report ID 3 `GET_REPORT` control IN transfer, and reports `Controls   key1..key12, knob-left/press/right, knob1..knob3, layers 1..15`. WonKey accepts a 65-byte report ID 3 reply or the 1-byte `0xaa` status response returned by stock write-only firmware. On `1189:8890`, `key`, `macro`, `mouse`, and `media` validate optional `[CONTROL]` (`key1`..`key12`, `knob-left`, `knob-press`, `knob-right`, `knob1-left`..`knob3-right`, defaulting to `key1`) and `--layer <1..15>` (defaulting to `1`) before live writes are refused for `1189:8890`.

Bare `wonkey` enumerates and inspects all compatible devices automatically in deterministic USB-path order (`[1]`, `[2]`, and so on). For changes, when one compatible device is attached, WonKey selects it automatically; when multiple devices are attached, pass `--device <N>` using the USB-path-sorted device number shown by `wonkey`. Moving or reconnecting a device can change its path, node, and device number.

## Check hardware effects and persistence

An exact readback comparison on `0112` proves the returned configuration bytes, not emitted key events or visible lighting effects.
Check the configured keyboard, trigger, mouse, media, or macro action and lighting separately after a successful change.
The new action types and expanded keyboard usages have no hardware validation yet.
If keyd holds an exclusive input grab, another event viewer can report no events.
Silence does not prove that configuration failed.
Stop or reconfigure keyd only with separate authority.

Persistence requires a separate check after a manual reconnect.
Run `wonkey` without arguments again.
Check the physical effects.
WonKey never reconnects or resets the device automatically.

## Apply safety

The [selection and confirmation guide](usage#selection-and-confirmation) covers terminal requirements, previews, accepted answers, and cancellation.

### Revalidation

Confirmation reads from one buffered input reader.
After confirmation, WonKey revalidates the selected descriptor, physical path, and node identity, then performs a fresh readback in memory.
The transaction uses the freshly acquired identifier and version as expected values. It also checks every configuration byte against the approved plan.
Changed identity, version, or settings stop before upload, even if the new settings already match the requested values.
A no-op or cancellation sends no settings upload or commit.
Bare `wonkey` reads hardware in memory without writing files.

### Upload and readback

For configuration `C` on an XFKEY One Key Max (`0112`), upload uses:

```text
AF 02 00 3C C[0:60]
AF 02 3C 3C C[60:120]
AF 02 78 08 C[120:128] Z(52)
AF 04 Z(62)
```

Slice upper bounds are excluded. `Z(n)` means n zero bytes. Each vendor payload is 64 bytes.
Linux output adds one leading zero report-ID placeholder, making 65 bytes. Every AF02 and AF04 reply must echo all 64 bytes exactly.
No retries occur. Any chunk or echo error stops before commit. Commit failures stop without follow-up queries.
After a valid commit echo, AF06/07/08 reconstruct the current configuration in memory for an exact 128-byte comparison.
A mismatch or timeout fails honestly and never triggers rollback.

The query-only allowlist remains AF01/06/07/08 with zero padding. Configuration uses a separate guarded transaction, not that allowlist.
Both paths check descriptors and the node before each exchange.
The transport uses `O_NONBLOCK` and a two-second deadline shared by each write/read pair.
An output syscall can remain in the kernel after timeout. A private duplicate retains that exact opened device until the call ends.
The session stops on timeout without another command. Do not assume that a timed-out submitted write did nothing.
Only the selected vendor node opens on `0112`, never keyboard input nodes. The identity query also primes persistence according to upstream research.

See [usage and configuration](usage) for setting values and [protocol evidence](protocol#evidence) for sources.
