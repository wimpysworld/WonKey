# Hardware operations and safety

[Wiki home](Home) · [Usage and configuration](usage)

`key COMBINATION`, `rgb MODE [RGB]`, and `restore [capture-directory]` change stored settings after confirmation.
Bare `key` and `rgb` only read the device.
Restore changes supported settings, not firmware or the complete historical configuration.

[Device access](#device-selection-and-access) · [Restore sources](#restore-sources) · [Hardware checks](#check-hardware-effects-and-persistence) · [Apply safety and records](#apply-safety-and-records)

## Device selection and access

Your user needs access to the selected vendor hidraw node.
WonKey does not grant permissions or install permission rules.
Ask an administrator to configure access for the verified device.
Do not guess a device from its hidraw number.

Selection requires VID/PID `af88:6688`, revision `0100`, and exact USB/configuration bytes.
All four report descriptors must match exactly.
The device must have interfaces 0 to 3 and one vendor hidraw mapping on interface 3.
Report descriptor lengths are 62, 114, 25 and 34 bytes.
The vendor interface has usage page `FF00`, usage `01`, endpoints `84`/`04`, 64-byte payloads, and no report IDs.
The shared serial `XFKEY` is not unique.
Moving or reconnecting the device can change its path and node.

## Restore sources

See [restore saved settings](usage#restore-saved-settings) for commands and source selection.
WonKey validates the source completion record, identity, raw replies, and exact reconstructed configuration.
It reads source files through bounded, descriptor-relative operations, without following directory links.
It keeps validated source bytes in memory through confirmation.

A complete backup remains useful if a later upload, commit, or comparison failed.
Retention ownership and a successful apply outcome are not restore requirements.
An explicit source can be an old or moved capture outside automatic storage.

Compatibility checks compare model, version, and protocol identifier.
These values do not prove physical-device identity.
The fresh physical path, descriptors, and node bind the transaction to the selected device.
Restore preserves current unknown bytes, even when they differ from the source.

## Check hardware effects and persistence

An exact readback comparison proves the returned configuration bytes, not emitted key events or visible lighting effects.
Check the key and lighting separately after a successful change.
If keyd holds an exclusive input grab, another event viewer can report no events.
Silence does not prove that configuration failed.
Stop or reconfigure keyd only with separate authority.

Persistence requires a separate check after a manual reconnect.
Run `wonkey key` and `wonkey rgb` again.
Check the physical effects.
WonKey never reconnects or resets the device automatically.

## Apply safety and records

The [selection and confirmation guide](usage#selection-and-confirmation) covers terminal requirements, previews, accepted answers, and cancellation.

### Revalidation

Selection and confirmation share one buffered input reader, so pasted answers remain available.
After confirmation, WonKey revalidates the selected descriptor, physical path, and node identity, then creates and validates a new durable backup.
The transaction uses the freshly acquired identifier and version as expected values. It also checks every configuration byte against the approved plan.
Changed identity, version, or settings stop before upload, even if the new settings already match the requested values.
A no-op or cancellation sends no settings upload or commit and creates no backup.
Bare `key` and `rgb` read hardware without saved captures. No files does not mean offline.

### Durable backup storage

New backups always use [XDG storage](usage#backup-storage).
An explicit restore source selects input data, not a backup destination.

The tool creates missing capture-root directories with mode `0700`, without changing existing permissions.
It resolves the root path and synchronises every directory from that root up to `/`, including existing ancestors.
Any synchronisation failure stops before the backup query or settings upload. Public changes already read the device to show their preview.
The synchronisation check also covers roots created by an earlier interrupted attempt.
Apply holds one capture-root lock, then creates a private backup.

### Capture names

New captures use `YYMMDD-HHMMSS_key-KEY_rgb-MODE-COLOUR`, with a UTC start time.
An example is `260912-083853_key-ctrl-alt-f13_rgb-steady-0000ff`.
Names describe the captured bytes, not the requested settings. Names are lowercase, with modifiers ordered `ctrl-shift-alt-gui` and six RGB hex digits without `#`.
Collisions add `-2`, `-3`, and so on, with an exclusive limit of 100 candidates. Existing directories are never overwritten.
Before settings arrive, the new directory uses `key-unknown_rgb-unknown-unknown`. Identity-only and failed queries retain this fallback.
After a complete settings query, WonKey labels only the new unfinished directory.
The rename is atomic, prevents replacement, and includes directory synchronisation.

Unsupported key layouts use `key-unknown-HEX`, where `HEX` contains the first five configuration bytes.
Unknown lighting modes use `rgb-unknown-XX-COLOUR`, where `XX` is the raw mode byte. Available RGB bytes remain six hex digits.
WonKey does not migrate or rename existing captures. Internal filenames and retention safety checks stay unchanged.

### Backup validation

WonKey synchronises the raw identity/replies, current 128-byte configuration, completion marker, and directories before uploading.
It reopens and checks the backup and expected identity before it writes a strict `backup.json` ownership record.
It then checks the supported layout and saves the intended configuration and plan durably.
Backup validation or storage failure sends no configuration writes.

### Upload and readback

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
Both paths check descriptors and the node before each exchange.
The transport uses `O_NONBLOCK` and a two-second deadline shared by each write/read pair.
An output syscall can remain in the kernel after timeout. A private duplicate retains that exact opened device until the call ends.
The session stops on timeout without another command. Do not assume that a timed-out submitted write did nothing.
Only the selected vendor node opens, never keyboard input nodes. The identity query also primes persistence according to upstream research.

### Transaction records

| Record | Meaning |
|---|---|
| `provenance.json`, `reply-01.bin`, `reply-06/07/08.bin`, `configuration.bin`, `result.json` | New backup of current query state. `result.json` completes the backup only. |
| `backup.json` | Strict ownership, device identity, directory name, and UTC creation record for retention |
| `plan.json`, `intended-configuration.bin` | Exact intended bytes before upload |
| `write-1/2/3/4-request.bin`, corresponding `*-reply.bin` | Upload/commit evidence, including malformed replies when available |
| `post-reply-06/07/08.bin`, `post-configuration.bin` | Available post-commit evidence |
| `apply-outcome.json` | No-op, verified readback, mismatch, or failure with state uncertainty |

Files use exclusive creation and file/directory synchronisation. Existing records are never overwritten.
A storage failure after a device command can prevent complete records. The command reports that failure, not a durable success.
Missing `apply-outcome.json` never proves success. No software can guarantee recording after power loss or a failed storage device.

### Retention

After a durable `no-op` or `readback-verified` outcome, WonKey keeps the newest 10 owned backups for the same model and identifier.
Retention ignores legacy, incomplete, malformed, foreign, linked, and uncertain directories. It never uses the version or physical path to group backups.
A cleanup failure adds a warning but keeps the successful apply result. Do not retry a successful apply because of this warning.

See [usage and configuration](usage) for setting values and [protocol evidence](protocol#evidence) for sources.
