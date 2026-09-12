# Protocol and evidence

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

**`set` and legacy `apply --write` can upload and commit settings.** `set --dry-run` only reads settings. There is no arbitrary packet sender, firmware writer, reset, or bootloader command.

The authentic user capture in `internal/xfkey/testdata/hardware-20260911` confirms identify/readback framing on this unit:
model `0112`, version `1014`, identifier `be077ba2`, Enter on press without modifiers, and RGB API mode 0 with channels `255,255,255`.
The four report descriptors also match. Separate user-run tests confirmed upload/commit echoes and full 128-byte readback after F13 and steady-blue changes.
The user confirmed F13 press/release events, steady-blue RGB, and persistence of both settings after reconnect. Other settings and devices remain unverified.
The parser's conservative `host-derived, hardware-unverified` status remains in existing output and capture metadata. The included query fixture does not record those later upload tests.
A settings capture is **not a firmware backup** or a proven restore image.

Run command examples from the project root after [building WonKey](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
See [device selection](hardware.md#find-the-device-path) for descriptor requirements and [apply safety](hardware.md#apply-safety-and-records) for upload framing.

## Existing offline operations

`wonkey protocol preview` remains offline and deliberately constructs a complete replacement. It is not an apply input.
All `protocol` commands always write one JSON value to stdout.
Every field is required, and `--replace-all` acknowledges resetting all other bytes to zero.

For the parser examples, load the authentic fixture replies with Python 3. These commands only read local files.

```sh
fixture=internal/xfkey/testdata/hardware-20260911
IDENTIFY_HEX=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).read_bytes().hex())' "$fixture/reply-01.bin")
READ6_HEX=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).read_bytes().hex())' "$fixture/reply-06.bin")
READ7_HEX=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).read_bytes().hex())' "$fixture/reply-07.bin")
READ8_HEX=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).read_bytes().hex())' "$fixture/reply-08.bin")
```

```sh
./wonkey protocol preview --replace-all --key enter --modifiers 0 --trigger 1 \
  --rgb-mode 1 --red 0 --green 0 --blue 255
./wonkey protocol parse-identify --hex "$IDENTIFY_HEX"
./wonkey protocol parse-readback --identify "$IDENTIFY_HEX" \
  --read6 "$READ6_HEX" --read7 "$READ7_HEX" --read8 "$READ8_HEX"
```

Each supplied reply must be exactly 128 hex characters, with no spaces or report-ID placeholder.
Identify has model at offsets 2–3, version at 4–5, both big-endian, and identifier at 6–9.
Readback copies `RX6[2:64]` to `C[0:62]`, `RX7[2:64]` to `C[62:124]`, and `RX8[2:6]` to `C[124:128]`.
All raw replies remain available, including unused RX8 bytes. Offline parsing retains unknown RGB values, labelled `unknown`.

See [lighting](usage.md#lighting) for the full vendor mode table and [configuration fields](usage.md#plan-against-a-settings-capture) for writable offsets.

The hidden top-level `preview`, `parse-identify`, and `parse-readback` aliases remain available for compatibility.

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
