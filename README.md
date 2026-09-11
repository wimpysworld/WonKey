# WonKey 1️⃣

One key. Your rules.

This Linux, standard-library Go tool inspects descriptors, captures queries, plans partial settings changes offline, and applies explicitly requested settings.
**Only `apply --write` permits configuration upload and commit.** There is no arbitrary packet sender, firmware writer, reset, or bootloader command.

The authentic user capture in `internal/xfkey/testdata/hardware-20260911` confirms identify/readback framing on this unit:
model `0112`, version `1014`, identifier `be077ba2`, Enter on press without modifiers, and RGB API mode 0 with channels `255,255,255`.
The four report descriptors also match. Upload echoes, effects, and persistence still need a user-run hardware test.
The parser's conservative `host-derived, hardware-unverified` status remains in existing output and capture metadata. It does not describe a tested upload.
A settings capture is **not a firmware backup** or a proven restore image.

See [AGENTS.md](AGENTS.md) for development guidance and offline checks.

## Build and inspect

Requires Linux and Go 1.23 or later. Run commands from the project root.
The build disables VCS stamping so that it also works when VCS metadata is unavailable.

```sh
go build -buildvcs=false -o ./wonkey ./cmd/wonkey
./wonkey inspect --path 1-1.2
```

`inspect` reads sysfs and node metadata without opening hidraw. It reports mode and owner, not effective ACL access.
Selection requires VID/PID `af88:6688`, revision `0100`, exact USB/configuration bytes, all four exact report descriptors, interfaces 0–3, and one vendor hidraw mapping on interface 3.
Report descriptor lengths are 62, 114, 25 and 34 bytes. The vendor interface has usage page `FF00`, usage `01`, endpoints `84`/`04`, 64-byte payloads, and no report IDs.
The shared serial `XFKEY` is not unique. Reinspect after moving or reconnecting the device.

### Plan against the original capture

These commands are offline. They do not change settings or open a device.

```sh
original=/home/martin/.local/state/agent-reviews/key/worktree-key/run-20260911T150408Z-dMkT7Y/captures/20260911T162302.726391430Z-3220202532
./wonkey plan --capture "$original" --rgb-mode 1 --red 0 --green 0 --blue 255
./wonkey plan --capture "$original" --key f13
```

The labelled fixture `internal/xfkey/testdata/hardware-20260911` is also a valid `--capture` directory.
Planning checks the completion marker, identity, raw replies, and reconstructed configuration for consistency.
Output includes current/intended configurations, changed offsets, and upload/commit payloads. A no-op has no upload payloads.
A saved plan never authorises a later write. Apply always reads and backs up current device state again.

Only explicitly supplied fields change. All other bytes remain equal to the current configuration.
Supported current layouts are single-key Enter or F13, with a known trigger, modifier mask, and RGB mode. Unknown layouts or modes fail closed, even for RGB-only changes.
There is no conversion of macros, mouse/media commands, multi-key layouts, or unknown keys.

| Flag | Values | Configuration byte |
|---|---|---:|
| `--key` | `enter` (usage `28`), `f13` (usage `68`) | 4 |
| `--trigger` | 1 press, 2 release, 3 both | 1 |
| `--modifiers` | 0–15, left Ctrl=1, Shift=2, Alt=4, GUI=8 | 2 |
| `--rgb-mode` | API index 0–7, stored as index+1 | 124 |
| `--red`, `--green`, `--blue` | 0–255 each | 125–127 |

Bytes 0 and 3 must be `00` and `01`. The tool preserves those bytes and bytes 5–123.
There are no default key or RGB changes. At least one explicit field is required.

## First hardware test: use the on-disk wrapper

Run the wrapper as your normal user, **not through sudo**. It builds the current source into a new private temporary directory.
It builds `wonkey` in a private `wonkey-build-*` directory and does not use an existing `/tmp/wonkey`. Dependencies are Go, sudo, Python 3, `getfacl`, `setfacl`, `stat`, and `cmp`.

The wrapper is fixed to physical path `1-1.2` and `/dev/hidraw4`. It checks the descriptor-verified mapping before granting access.
If either value changes, inspect first and update the two constants. Do not guess a node from its number.

The privileged shell saves the existing ACL, grants access only to the invoking UID on this node, and runs the tool as that UID.
It checks node identity before access and restoration, restores with `setfacl -P`, and compares the saved/restored ACLs.
ACL records remain in the printed private `/tmp/wonkey-acl-*` directory. If identity changes, restoration stops and reports the record path.
Signals trigger cleanup, except uncatchable termination or power loss. If cleanup fails, stop and ask an administrator to restore the ACL after checking node identity.
The wrapper does not create persistent permission rules.
Set `WONKEY_CAPTURE_ROOT` to an absolute directory to configure capture storage.
A non-empty `WONKEY_CAPTURE_ROOT` takes precedence over `XFKEY_CAPTURE_ROOT`, which remains a supported fallback.
If both are unset or empty, captures still default to `$HOME/.local/state/xfkey-captures`.
Existing captures stay in place. Capture formats and explicit `--capture` and `--capture-root` paths are unchanged.

Optional query only, with no settings upload or commit:

```sh
./capture-settings.sh readback
```

### 1. Static blue, preserving Enter

**This command changes RGB settings when executed.** It preserves the current key, modifiers, trigger, and every unrelated byte.
Run it only after inspection confirms the fixed path and node. The expected version and identifier come from the original capture.

```sh
./capture-settings.sh apply --write --expect-identifier be077ba2 --expect-version 1014 \
  --rgb-mode 1 --red 0 --green 0 --blue 255
```

Stop if the command fails. Do not retry automatically. A failure after upload starts leaves device state uncertain.
If all 128 readback bytes match, check that the LED is static blue and the key still sends Enter.
The mode name comes from the vendor utility. Its visual effect needs this hardware check.

### 2. F13 separately, preserving RGB

**This command changes the key to F13 when executed.** It preserves the current RGB, modifiers, trigger, and unrelated bytes.
Run this separate test only after the first test succeeds.

```sh
./capture-settings.sh apply --write --expect-identifier be077ba2 --expect-version 1014 --key f13
```

Check F13 with an appropriate input viewer. Matching configuration bytes alone do not prove the emitted key event.

### 3. Restore only the known original fields

**This command changes the key/modifier/trigger and RGB fields when executed.** The values come from the original capture.
It preserves all other current bytes. It does not restore firmware or copy a complete saved configuration over the device.

```sh
./capture-settings.sh apply --write --expect-identifier be077ba2 --expect-version 1014 \
  --key enter --modifiers 0 --trigger 1 --rgb-mode 0 --red 255 --green 255 --blue 255
```

### Persistence is a separate check

A successful apply reports `all_128_bytes_readback_verified: true` and `persistence_after_reconnect_verified: false`.
After a successful test, manually reconnect the device, inspect the mapping again, and run the query-only wrapper.
Compare `configuration.bin` from that new capture against `intended-configuration.bin` from the relevant apply capture.
Only a matching reconnect capture establishes persistence for that test. The tool never reconnects or resets the device automatically.

## Apply safety and records

Direct use requires `apply --write --path ... --capture-root ... --expect-identifier ... --expect-version ...` plus explicit settings.
The model must be `0112`. Missing write permission, target flags, or settings fail before device access.
The tool creates missing capture-root directories with mode `0700`, without changing existing permissions.
It resolves the root path and synchronises every directory from that root up to `/`, including existing ancestors.
Any synchronisation failure stops before device access. This also covers roots created by an earlier interrupted attempt.
Apply then creates a new private, exclusive capture directory and synchronises its parent before querying identity and all three readback replies.
It synchronises the raw identity/replies, current 128-byte configuration, completion marker, and directories before uploading.
It reopens and checks that backup, checks the expected identity/version and supported current layout, and saves the intended configuration and plan durably.
Backup validation or storage failure sends no configuration writes. A no-op still takes a backup but sends no upload or commit.

For configuration `C`, upload uses exactly:

```text
AF 02 00 3C C[0:60]
AF 02 3C 3C C[60:120]
AF 02 78 08 C[120:128] Z(52)
AF 04 Z(62)
```

Slice upper bounds are excluded. `Z(n)` means n zero bytes. Each vendor payload is 64 bytes.
Linux output adds one leading zero report-ID placeholder, making 65 bytes. Every AF02 and AF04 reply must echo all 64 bytes exactly.
No retries occur. Any chunk/echo/storage error stops before commit. Commit failures stop without follow-up queries.
After a valid commit echo, AF06/07/08 reconstruct the current configuration for an exact 128-byte comparison.
A mismatch or timeout fails honestly, records available evidence, and never triggers rollback.

The query-only allowlist remains AF01/06/07/08 with zero padding. Configuration uses a separate guarded transaction, not that allowlist.
Both paths reuse the strict transport: fresh descriptor/node checks before each exchange, `O_NONBLOCK`, and a two-second deadline shared by each write/read pair.
An output syscall can remain in the kernel after timeout. A private duplicate retains that exact opened device until the call ends.
The session stops on timeout without another command. Do not assume that a timed-out submitted write did nothing.
Only the selected vendor node opens, never keyboard input nodes. Identify also primes persistence according to upstream research.

| Record | Meaning |
|---|---|
| `provenance.json`, `reply-01.bin`, `reply-06/07/08.bin`, `configuration.bin`, `result.json` | New backup of current query state. `result.json` completes the backup only. |
| `plan.json`, `intended-configuration.bin` | Exact intended bytes before upload |
| `write-1/2/3/4-request.bin`, corresponding `*-reply.bin` | Upload/commit evidence, including malformed replies when available |
| `post-reply-06/07/08.bin`, `post-configuration.bin` | Available post-commit evidence |
| `apply-outcome.json` | No-op, verified readback, mismatch, or failure with state uncertainty |

Files use exclusive creation and file/directory synchronisation. Existing records are never overwritten.
A storage failure after a device command can prevent complete records. The command reports that failure, not a durable success.
Missing `apply-outcome.json` never proves success. No software can guarantee recording after power loss or a failed storage device.

## Existing offline operations

`preview` remains offline and deliberately constructs a complete replacement. It is not an apply input.
Every field is required, and `--replace-all` acknowledges resetting all other bytes to zero.

```sh
./wonkey preview --replace-all --key enter --modifiers 0 --trigger 1 \
  --rgb-mode 1 --red 0 --green 0 --blue 255
./wonkey parse-identify --hex "$IDENTIFY_HEX"
./wonkey parse-readback --identify "$IDENTIFY_HEX" \
  --read6 "$READ6_HEX" --read7 "$READ7_HEX" --read8 "$READ8_HEX"
```

Each supplied reply must be exactly 128 hex characters, with no spaces or report-ID placeholder.
Identify has model at offsets 2–3, version at 4–5, both big-endian, and identifier at 6–9.
Readback copies `RX6[2:64]` to `C[0:62]`, `RX7[2:64]` to `C[62:124]`, and `RX8[2:6]` to `C[124:128]`.
All raw replies remain available, including unused RX8 bytes. Offline parsing retains unknown RGB values, labelled `unknown`.

RGB vendor labels, not verified effects:

| API index | Stored byte | Vendor interpretation |
|---:|---:|---|
| 0 | 01 | Full-colour gradient |
| 1 | 02 | Single-colour steady |
| 2 | 03 | Single-colour flowing |
| 3 | 04 | Flash on click |
| 4 | 05 | Neon flowing |
| 5 | 06 | Lights off |
| 6 | 07 | On while pressed, off on release |
| 7 | 08 | Toggle on click |

No brightness or speed field is established.

## Evidence

Sources are pinned to `cuylerstuwe/xfkey-cross-platform` commit `d86f50af54c5ce6395c41c958f8598d42e1d8a15`:

- [TECH.md descriptors and protocol](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/TECH.md). Some descriptor lengths and its identify layout are incorrect.
- [Python encoder, configuration and framing](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/flash_xfkey.py#L326-L428).
- [Browser encoder](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/examples/listener-web/index.html#L402-L454).
- [Vendor executable, static analysis only](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/xfp-win/xfp-win.exe), SHA-256 `0fdd932071b00064cf63b91c34d70b2be13ded06f14ce03f46c38dbd8a2b510c`.
- [Vendor database](https://github.com/cuylerstuwe/xfkey-cross-platform/blob/d86f50af54c5ce6395c41c958f8598d42e1d8a15/xfp-win/userdata.db), five “One Key Max” rows with type 274 (`0112`).

Vendor executable evidence: big-endian model at VA `0046A989–0046A9A1`, version at `0046AB17–0046AB27`, identifier at `0046A9BD–0046AA15`, readback copies at `0046ACEF–0046AE71`, and full configuration copy at `0042E8A0–0042E8D6`.
RGB strings are at file offsets `000A6D46–000A6DA9`, indexed insertion at VA `00435234–00435618`, and assignment at `00436110–00436121`.
The authentic fixture confirms queries only.
