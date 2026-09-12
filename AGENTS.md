# WonKey

## Architecture notes

- Keep `cmd/wonkey` and `cmd/wonkey-dev` limited to process entry and exit handling. Keep CLI, protocol, and Linux device logic in `internal/xfkey`.
- Expose only `key` and `rgb` in the consumer executable, with help and `key --on` flags. Keep legacy and protocol tooling in the separately built `wonkey-dev`, outside the default build/install.
- Use WonKey/`wonkey` for project branding and the executable, not for genuine hardware or compatibility identifiers.
- Preserve `XFKEY`, One Key Max, `internal/xfkey`, upstream URLs, and captured metadata.
- Preserve the `XFKEY_CAPTURE_ROOT` fallback and default `$HOME/.local/state/xfkey-captures` when changing capture configuration.
- Keep protocol changes grounded in the [protocol and source evidence](wiki/protocol.md#evidence). The upstream TECH.md has known descriptor/layout errors.

## Build and test

Run these offline checks from the project root on Linux:

```sh
go test ./...
go vet ./...
(
  set -eu
  build=$(mktemp -d)
  trap 'rm -rf -- "$build"' EXIT
  go build -buildvcs=false -o "$build/wonkey" ./cmd/wonkey
)
```

Use `-buildvcs=false` when VCS metadata is unavailable or stamping fails. It disables stamping, not compilation checks.
Build into a fresh temporary directory to avoid overwriting an existing `wonkey` binary.

Check formatting without changing source:

```sh
unformatted=$(gofmt -l cmd internal) && printf '%s' "$unformatted" && test -z "$unformatted"
```

Check helper syntax without executing either script. Run ShellCheck when available:

```sh
sh -n capture-settings.sh && sh -n check-key.sh
shellcheck capture-settings.sh check-key.sh
```

## Testing

- Use synthetic transports and temporary captures for failure tests. Do not make tests depend on connected hardware.
- Keep authentic fixtures in `internal/xfkey/testdata/hardware-20260911` byte-for-byte unchanged, including their warnings and paths.
- Copy fixtures to temporary directories before corruption tests. Never regenerate authentic evidence to match changed output.
- For protocol or apply changes, test framing, unspecified-byte preservation, guards, durable backup failures, upload/commit failures, deadlines, and no-op behaviour.
- Test exact post-write comparison separately from hardware effects and persistence. Query fixtures prove neither upload nor restore.

## Security and secrets

- Obtain explicit user authority before device inspection, queries, writes, event streams, helper execution, or permission changes.
- Do not treat README hardware examples as permission to execute them. `inspect` reads live sysfs despite not opening hidraw.
- Preserve the explicit `apply --write` gate in developer tooling. For public changes, acquire expected identity/version freshly and retain the shared guarded transaction and automatic backup root.
- Auto-select one compatible device. For multiple matches, require terminal selection tied to a displayed physical path, never persistent list numbering. Revalidate selected descriptors, node, path, identity, and version before the transaction.
- Refuse public changes without a terminal before device access. Read and show current-to-proposed values, confirm exactly `write` with no bypass, then save, reopen, and validate a fresh durable backup before upload.
- Share one buffered reader between selection and confirmation. Blank, EOF, or invalid input cancels safely. Reject settings drift after preview, including drift that makes the request a no-op.
- Keep bare `key` and `rgb` query-only with no saved captures. Keep no-args/help and rejected syntax free of hardware access. No-op and cancellation must send no settings upload or commit.
- Require a new durable backup and revalidate it before upload. A saved offline plan never authorises a live write.
- Preserve exclusive capture creation and file/directory synchronisation, including capture-root ancestors. Never overwrite existing captures or ACL records.
- Treat a public key expression as the complete combination, clearing unspecified modifiers. Preserve omitted `--on` and RGB colour. Change no unrelated bytes. Reject unsupported current layouts even for RGB-only changes.
- Keep developer `preview --replace-all` offline and separate from apply. Its zero-filled replacement must not replace preservation logic.

## Gotchas

- Keep query commands restricted to zero-padded AF01/06/07/08. Keep AF02 upload and AF04 commit behind the separate guarded transaction.
- Keep vendor payloads and replies at 64 bytes. Only Linux hidraw output gets a leading zero placeholder, making 65 bytes.
- Require exact 64-byte upload/commit echoes. Preserve unused reply bytes as evidence.
- Preserve fresh descriptor/node checks before each exchange and the shared write/read deadline. Use only the selected vendor hidraw node.
- Do not infer identity from hidraw numbering or the shared `XFKEY` serial. Reinspect after reconnecting or moving the device.
- Stop on transaction errors without automatic retries or rollback. A timed-out submitted write can still complete in the kernel.
- Retain the private duplicate descriptor until its submitted output syscall finishes. `O_NONBLOCK` does not bound that syscall.
- Read [apply safety and records](wiki/hardware.md#apply-safety-and-records) before changing transaction order or capture completion semantics.
- If keyd holds an exclusive input grab, another event viewer can see no events. Do not treat silence as failed configuration.
- Obtain separate authority before stopping keyd or changing its configuration to test input events.
