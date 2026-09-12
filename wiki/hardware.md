# Hardware operations and safety

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

`set` and legacy `apply --write` change stored settings. `set --dry-run` only reads the device. Configuration is not firmware flashing.
A settings capture is not a firmware backup or a proven restore image.
Read the conditions below before device access. Run all commands from the project root.

## Find the device path

Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
A device path identifies one physical USB connection. Find the current device path before using the fixed-path wrapper.

```sh
./wonkey advanced devices
```

`advanced devices` reads sysfs and device-node metadata without opening a HID device. It reports mode and owner, not effective ACL access.
Selection requires VID/PID `af88:6688`, revision `0100`, exact USB/configuration bytes, all four exact report descriptors, interfaces 0–3, and one vendor hidraw mapping on interface 3.
Report descriptor lengths are 62, 114, 25 and 34 bytes. The vendor interface has usage page `FF00`, usage `01`, endpoints `84`/`04`, 64-byte payloads, and no report IDs.
The shared serial `XFKEY` is not unique. Reinspect after moving or reconnecting the device.

## First hardware test: use the on-disk wrapper

Run the wrapper as your normal user, **not through sudo**. It builds the current source into a new private temporary directory.
It builds `wonkey` in a private `wonkey-build-*` directory and does not use an existing `/tmp/wonkey`. Dependencies are Go, sudo, Python 3, `getfacl`, `setfacl`, `stat`, and `cmp`.

The wrapper is fixed to physical path `1-1.2` and `/dev/hidraw4`. It checks the descriptor-verified mapping before granting access.
If either value changes, find the device path first and update the two constants. Do not guess a node from its number.

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

**This command changes lighting settings when executed.** It preserves the current key, modifiers, trigger, and every unrelated byte.
Run it only after inspection confirms the fixed path and node. The expected version and identifier come from the original capture.

```sh
./capture-settings.sh apply --write --expect-identifier be077ba2 --expect-version 1014 \
  --lighting steady --colour 0000ff
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
  --key enter --modifiers none --trigger press --lighting gradient --colour ffffff
```

### Persistence is a separate check

A successful JSON apply reports `readback_verified: true` and `persistence_after_reconnect_verified: false`.
After a successful test, manually reconnect the device, find the device path again, and run the query-only wrapper.
Compare `configuration.bin` from that new capture against `intended-configuration.bin` from the relevant apply capture.
Only a matching reconnect capture establishes persistence for that test. The tool never reconnects or resets the device automatically.

## Apply safety and records

Everyday use is `show` and `set key=f13`, `set light=steady:0000ff`, or combined assignments.
Both commands select exactly one compatible device. Multiple matches require `--device PHYSICAL_PATH`.
`set` freshly reads identity, version, and settings in memory, shows the changes, and asks for confirmation.
Only the exact answer `write` confirms. Blank input cancels. `--yes` skips confirmation only.
After confirmation, `set` creates and validates a new durable backup through the guarded transaction.
The transaction uses the freshly acquired identifier and version as expected values. It also checks every configuration byte against the approved plan.
Changed identity, version, or settings stop before upload, even if the new settings already match the requested values.
A no-op or cancellation sends no settings upload or commit and creates no backup.
`set --dry-run` reads the device and previews changes without saved captures or settings writes. No files does not mean offline.

Legacy `apply TARGET key=f13 --write` keeps its target, write gate, and backup-before-confirmation sequence.
Use `show --target` for the copyable form `0112:path:identifier:version`, for example `0112:1-1.2:be077ba2:1014`.
Copy the whole target. Do not guess its parts. It binds the physical path, model, expected identifier, and expected version.
The old `--target` flag and `wonkey-target-v1:` encoded tokens remain accepted. JSON targets keep the old format.
Legacy apply rejects missing write permission, target, or settings before device access.
Use `advanced plan CAPTURE` only for optional offline work with a completed backup. A saved plan never authorises a write.
Old top-level commands remain hidden aliases. Protocol tools are under `advanced protocol`.

The fixed-path wrapper is a compatibility path. It still passes the hidden `--path`, `--expect-identifier`, and `--expect-version` flags after descriptor checks. Hidden numeric setting flags remain accepted for existing scripts. Do not mix numeric values with friendly `--lighting` and `--colour` values.
The tool creates missing capture-root directories with mode `0700`, without changing existing permissions.
It resolves the root path and synchronises every directory from that root up to `/`, including existing ancestors.
Any synchronisation failure stops before the backup query or settings upload. `set` already read the device to show its plan.
This also covers roots created by an earlier interrupted attempt.
Apply holds one capture-root lock, then creates a private backup named `YYYYMMDD-HHMMSS-XXXX` with a random hexadecimal suffix.
It synchronises the raw identity/replies, current 128-byte configuration, completion marker, and directories before uploading.
It reopens and checks the backup and expected identity before it writes a strict `backup.json` ownership record.
It then checks the supported layout and saves the intended configuration and plan durably.
Backup validation or storage failure sends no configuration writes. Legacy apply takes a backup for a no-op but sends no upload or commit.

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
| `backup.json` | Strict ownership, device identity, directory name, and UTC creation record for retention |
| `plan.json`, `intended-configuration.bin` | Exact intended bytes before upload |
| `write-1/2/3/4-request.bin`, corresponding `*-reply.bin` | Upload/commit evidence, including malformed replies when available |
| `post-reply-06/07/08.bin`, `post-configuration.bin` | Available post-commit evidence |
| `apply-outcome.json` | No-op, verified readback, mismatch, or failure with state uncertainty |

Files use exclusive creation and file/directory synchronisation. Existing records are never overwritten.
A storage failure after a device command can prevent complete records. The command reports that failure, not a durable success.
Missing `apply-outcome.json` never proves success. No software can guarantee recording after power loss or a failed storage device.

After a durable `no-op` or `readback-verified` outcome, WonKey keeps the newest 10 owned backups for the same model and identifier.
Retention ignores legacy, incomplete, malformed, foreign, linked, and uncertain directories. It never uses the version or physical path to group backups.
A cleanup failure adds a warning but keeps the successful apply result. Do not retry a successful apply because of this warning.

See [usage and configuration](usage.md) for setting values and [protocol evidence](protocol.md#evidence) for sources.
