# WonKey

## Architecture notes

- Keep `cmd/wonkey` limited to process entry and exit handling. Keep CLI, protocol, and Linux device logic in `internal/xfkey` and `internal/ch57`.
- Expose only `key`, `trigger`, `macro`, `mouse`, `media`, `rgb`, and `led`, with help and validated command-specific options. Do not add developer commands, legacy aliases, or a second transaction engine.
- Use WonKey/`wonkey` for project branding and the executable, not for genuine hardware or compatibility identifiers.
- Preserve `XFKEY`, One Key Max, `internal/xfkey`, upstream URLs, and captured metadata.
- Keep protocol changes grounded in the [protocol and source evidence](wiki/protocol.md#evidence). The upstream TECH.md has known descriptor/layout errors.

## Build and test

After code or test changes, run these mandatory checks from the project root on Linux before a commit or PR:

```sh
just test
just lint
```

Use only `just build`, `just test`, and `just lint` for quality work.
Use `just lint` or `just lint check` for read-only checks.
Use `just lint correct` only when source correction is authorised.
Keep project quality work in `just/project/build.sh`, `test.sh`, or `lint.sh`.
Keep lint script check mode read-only. The lint script receives `check` or `correct`.
Do not replace the mandatory recipes with direct `go test` or `go vet` checks.
Call `tailor alter`, `tailor baste`, and `tailor measure` directly, without Just wrappers.
Keep `just setup` and release commands separate from quality checks. Never run them implicitly.
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

Use `just build` only in a temporary project copy for build validation.
The managed build writes `bin/wonkey`, then `just/project/build.sh` writes `wonkey` with `-buildvcs=false`.
Repeated builds replace both generated files. Never overwrite an existing root binary for validation.
`-buildvcs=false` disables VCS stamping, not compilation checks.
Keep tests independent of connected hardware.
Obtain explicit user authority before device access.

## Pages preview with Playwright MCP

- Enter `nix develop`, or load the development environment through direnv. Nix supplies `miniserve` and Chromium-only `playwright-mcp`.
- Reuse a suitable preview, or start `just pages` from the project root. It serves `pages/` at `http://127.0.0.1:18473` without a build.
- If the port is occupied, use another port, for example `miniserve --index index.html --interfaces 127.0.0.1 --port 18081 pages/`. Never stop an existing service automatically.
- Start, restart, or reload the client from the development environment, with normal approval checks.
- Use the configured `playwright-mcp --headless --isolated` server in Claude Code, Codex, OpenCode, or Pi. In Pi, discover browser tools through the MCP proxy's lazy tool loading.
- Use the connected MCP browser to navigate to the preview URL, with the selected port. MCP starts its own browser. Do not start a manual stdio session, CDP browser, or `run-cdp`.
- When a proxy is necessary, set `HTTPS_PROXY` in the development environment. The shared Nix wrapper passes the proxy only when `HTTPS_PROXY` is non-empty.
- Do not hard-code a proxy or reduce sandbox, certificate, or file-access restrictions.
- Verify actual CDN, font, and icon requests in the browser. Capture and inspect light and dark screenshots at narrow and wide viewport sizes. Do not treat HTTP 200 alone as proof that the page works.

## Testing

- Use synthetic transports for failure tests. Do not make tests depend on connected hardware.
- Keep authentic fixtures in `internal/xfkey/testdata/hardware-20260911` byte-for-byte unchanged, including their warnings and paths.
- Copy fixtures to temporary directories before corruption tests. Never regenerate authentic evidence to match changed output.
- For protocol or apply changes, test framing, unspecified-byte preservation, guards, upload/commit failures, deadlines, and no-op behaviour.
- Test supported, validated keyboard, mouse, media, and multi layouts offline, including action conversions and rejection of ambiguous layouts. Keep hardware effects and persistence unverified until user-authorised hardware tests.
- Test exact post-write comparison separately from hardware effects and persistence. Query fixtures do not prove upload.
- Test device selection and no-op behaviour without writes.

## Security and secrets

- Obtain explicit user authority before device inspection, queries, writes, event streams, helper execution, or permission changes.
- Do not treat README hardware examples as permission to execute them.
- For public changes, acquire expected identity/version freshly and retain the shared guarded transaction.
- Auto-select one compatible device. For multiple matches on a write command, require `--device <N>` using the deterministic USB-path-sorted device number shown by `wonkey`. Revalidate selected descriptors, node, path, identity, and version before the transaction.
- Bare `wonkey` enumerates and inspects all compatible devices without terminal selection.
- Read and show current-to-proposed values before writing settings.
- Reject settings drift before upload, including drift that makes the request a no-op.
- Keep bare `wonkey` query-only. Keep help and rejected syntax (including bare `key`, `trigger`, `macro`, `mouse`, `media`, `rgb`, and `led`) free of hardware access. A no-op must send no settings upload or commit.
- A saved offline plan never authorises a live write.
- Treat a public key expression as the complete combination, clearing unspecified modifiers. Preserve an existing keyboard trigger, or use press when replacing another action. Use `trigger` to change `press`, `release`, or `both` on devices that support key triggers. Preserve omitted RGB colour.
- Accept only supported, validated keyboard, mouse, media, and multi layouts. Reject ambiguous action conversions. Change no unrelated bytes. Reject unsupported current layouts even for RGB-only changes.

## Gotchas

- Keep query commands restricted to zero-padded AF01/06/07/08. Keep AF02 upload and AF04 commit behind the separate guarded transaction.
- Keep vendor payloads and replies at 64 bytes. Only Linux hidraw output gets a leading zero placeholder, making 65 bytes.
- Require exact 64-byte upload/commit echoes. Preserve unused reply bytes as evidence.
- Preserve fresh descriptor/node checks before each exchange and the shared write/read deadline. Use only the selected vendor hidraw node.
- Do not infer identity from hidraw numbering or the shared `XFKEY` serial. Reinspect after reconnecting or moving the device.
- Stop on transaction errors without automatic retries or rollback. A timed-out submitted write can still complete in the kernel.
- Retain the private duplicate descriptor until its submitted output syscall finishes. `O_NONBLOCK` does not bound that syscall.
- Read [apply safety](wiki/hardware.md#apply-safety) before changing transaction order.
- If keyd holds an exclusive input grab, another event viewer can see no events. Do not treat silence as failed configuration.
- Obtain separate authority before stopping keyd or changing its configuration to test input events.
