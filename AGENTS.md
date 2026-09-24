# WonKey

## Architecture notes

- Keep `cmd/wonkey` limited to process entry and exit handling. Keep CLI, protocol, and Linux device logic in `internal/xfkey`.
- Expose only `key`, `rgb`, and `restore`, with help and `key --on` flags. Do not add developer commands, legacy aliases, or a second transaction engine.
- Use WonKey/`wonkey` for project branding and the executable, not for genuine hardware or compatibility identifiers.
- Preserve `XFKEY`, One Key Max, `internal/xfkey`, upstream URLs, and captured metadata.
- Store automatic backups in `$XDG_STATE_HOME/wonkey/captures` when `XDG_STATE_HOME` is absolute and non-empty. Otherwise, use `$HOME/.local/state/wonkey/captures`. Reject an unset, empty, or relative HOME when fallback is necessary.
- Ignore `WONKEY_CAPTURE_ROOT` and `XFKEY_CAPTURE_ROOT`. Add no backup destination flags, literal `~` expansion, or temporary fallback. Leave old captures untouched. An explicit restore source never changes the backup destination.
- Keep protocol changes grounded in the [protocol and source evidence](wiki/protocol.md#evidence). The upstream TECH.md has known descriptor/layout errors.

## Build and test

After code or test changes, run these mandatory checks from the project root on Linux before a commit or PR:

```sh
just test
just lint
```

Use `just test` for all enabled tests and `just test-go` for Go tests only.
Do not replace the mandatory recipes with `just test-go`, direct `go test`, or `go vet` checks.
Fix all failures, including failures from optional CI checks.
After fixes, rerun both recipes.
If tools or network access are unavailable, report the blocked checks, not a pass.
When local and CI results differ, compare tool versions with the current CI configuration.

Check formatting without changing source:

```sh
unformatted=$(gofmt -l cmd internal) && printf '%s' "$unformatted" && test -z "$unformatted"
```

Validate compilation in a fresh temporary directory:

```sh
(
  set -eu
  build=$(mktemp -d)
  trap 'rm -rf -- "$build"' EXIT
  go build -buildvcs=false -o "$build/wonkey" ./cmd/wonkey
)
```

Use `just build` or `just build-go` only in a temporary project copy for build validation.
Both managed recipes write `bin/wonkey`; repeated builds replace that generated file.
Do not use `just build-wonkey` for validation because it writes `wonkey` at the project root and overwrites an existing root binary.
`-buildvcs=false` disables VCS stamping, not compilation checks.
Keep tests independent of connected hardware.
Obtain explicit user authority before device access.

## Pages preview with Playwright MCP

- Enter `nix develop`, or load the development environment through direnv. Nix supplies `miniserve` and Chromium-only `playwright-mcp`.
- Reuse a suitable preview, or start `just pages-port` from the project root. It serves `pages/` at `http://127.0.0.1:18080` without a build.
- If the port is occupied, use another port, for example `just pages-port 18081`. Never stop an existing service automatically.
- Start, restart, or reload the client from the development environment, with normal approval checks.
- Use the configured `playwright-mcp --headless --isolated` server in Claude Code, Codex, OpenCode, or Pi. In Pi, discover browser tools through the MCP proxy's lazy tool loading.
- Use the connected MCP browser to navigate to the preview URL, with the selected port. MCP starts its own browser. Do not start a manual stdio session, CDP browser, or `run-cdp`.
- When a proxy is necessary, set `HTTPS_PROXY` in the development environment. The shared Nix wrapper passes the proxy only when `HTTPS_PROXY` is non-empty.
- Do not hard-code a proxy or reduce sandbox, certificate, or file-access restrictions.
- Verify actual CDN, font, and icon requests in the browser. Capture and inspect light and dark screenshots at narrow and wide viewport sizes. Do not treat HTTP 200 alone as proof that the page works.

## Testing

- Use synthetic transports and temporary captures for failure tests. Do not make tests depend on connected hardware.
- Keep authentic fixtures in `internal/xfkey/testdata/hardware-20260911` byte-for-byte unchanged, including their warnings and paths.
- Copy fixtures to temporary directories before corruption tests. Never regenerate authentic evidence to match changed output.
- For protocol or apply changes, test framing, unspecified-byte preservation, guards, durable backup failures, upload/commit failures, deadlines, and no-op behaviour.
- Test exact post-write comparison separately from hardware effects and persistence. Query fixtures prove neither upload nor restore.
- Test restore with moved captures, failed-transaction backups, incompatible sources, corrupt records, links, source replacement, and different current unknown bytes.
- Test XDG fallback, invalid HOME, ignored retired variables, source selection, default-Yes confirmation, and cancellation without captures.

## Security and secrets

- Obtain explicit user authority before device inspection, queries, writes, event streams, helper execution, or permission changes.
- Do not treat README hardware examples as permission to execute them.
- For public changes, acquire expected identity/version freshly. Retain the shared guarded transaction and automatic backup root.
- Auto-select one compatible device. For multiple matches, require terminal selection tied to a displayed physical path, never persistent list numbering. Revalidate selected descriptors, node, path, identity, and version before the transaction.
- Refuse public changes without a terminal before device access. Read and show current-to-proposed values before confirmation. Then save, reopen, and validate a fresh durable backup before upload.
- Use `Save settings? [Y/n]:` followed by one space, with no bypass. Accept Enter, `y`, or `yes`, ignoring letter case. Cancel on EOF, `n`, `no`, or invalid input.
- Share one buffered reader across device selection, capture selection, and confirmation. Cancel device or capture selection on blank input, EOF, or invalid input. Reject settings drift after preview, including drift that makes the request a no-op.
- Keep bare `key` and `rgb` query-only with no saved captures. Keep no-args/help and rejected syntax free of hardware access. No-op and cancellation must send no settings upload or commit.
- Require a new durable backup and revalidate it before upload. A saved offline plan never authorises a live write.
- Preserve exclusive capture creation, private permissions, the root lifecycle lock, and file/directory synchronisation, including capture-root ancestors. Never overwrite existing captures or ACL records.
- Treat a public key expression as the complete combination, clearing unspecified modifiers. Preserve omitted `--on` and RGB colour. Change no unrelated bytes. Reject unsupported current layouts even for RGB-only changes.

## Restore safety

- Restore only the saved key, modifiers, trigger, RGB mode, and colour through the shared guarded transaction. Preserve every other current configuration byte. Show when different current unknown bytes will remain unchanged.
- Require supported layouts in both the source and current configuration. Check model, version, and protocol identifier compatibility. These values do not prove physical-device identity.
- Require `result.json`, consistent raw replies, and the exact reconstructed configuration. Use bounded, descriptor-relative reads through `loadCaptureAt`. Open the source directory without following links. Keep validated source bytes in memory through confirmation.
- Accept complete backups from failed transactions and moved captures without retention ownership or matching historical paths.
- For bare `restore`, list compatible captures newest first before creating a backup. Exclude captures whose supported settings already match. Require selection even for one candidate. Use one concise row per capture, with settings first and one short local date and time. Omit raw directory names and repeated labels.
- Tie capture selection numbers only to the displayed list. Add no automatic latest selection or confirmation bypass. If no eligible capture exists, explain the result without an upload.
- Show the selected device, source capture, and current-to-proposed changes before the shared default-Yes confirmation. Treat a matching explicit source as a no-op without a new backup.

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
