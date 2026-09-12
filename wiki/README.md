# WonKey documentation

WonKey reads and changes key combinations and RGB settings on a descriptor-verified device on Linux.
Start with the [project overview and build instructions](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run every command example from the project root, not from `wiki/`.

| Page | Use it to |
|---|---|
| [Usage and configuration](usage.md) | Set a complete key combination or choose an RGB mode. |
| [Developer tools](development.md) | Build separate legacy tooling and analyse saved captures offline. |
| [Hardware operations and safety](hardware.md) | Inspect the mapping, grant temporary access, back up settings, apply changes, and check restore limits. |
| [Protocol and evidence](protocol.md) | Parse replies offline and check encodings, captured identifiers, and source evidence. |

`key COMBINATION` and `rgb MODE [RGB]` change stored settings after confirmation. Bare `key` and `rgb` only read settings.
Read the hardware safety conditions before any live operation.
Configuration is not firmware flashing. Captures prove neither upload effects nor persistence.

For development and offline checks, read [AGENTS.md](https://github.com/wimpysworld/WonKey/blob/main/AGENTS.md).
