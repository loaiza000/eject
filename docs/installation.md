# Installation

EJECT is a single static executable with no runtime dependencies. Docker is needed only for the optional `docker compose config` syntax check and for runtime verification.

## Release archives

Each GitHub Release contains:

| File | Contents |
| --- | --- |
| `eject_<version>_<os>_<arch>.tar.gz` | `eject` (or `eject.exe`), `LICENSE`, `THIRD_PARTY_NOTICES.md`, third-party license texts |
| `eject_<os>_<arch>[.exe]` | The same executable, uncompressed |
| `SHA256SUMS` | SHA-256 of every archive, raw binary and license file |

`<os>` is `linux`, `darwin` (macOS) or `windows`; `<arch>` is `amd64` (x86-64) or `arm64` (Apple silicon, ARM servers, Windows on ARM). Run `uname -m` (Linux/macOS) or `$env:PROCESSOR_ARCHITECTURE` (Windows) if unsure: `x86_64`/`AMD64` means `amd64`, while `aarch64`/`arm64`/`ARM64` means `arm64`.

Always verify the checksum before running a downloaded executable. A checksum detects corruption and substitution of individual files; it does not prove who built the release, so download both files from the official repository page.

### Linux

```sh
VERSION=v0.1.0
ASSET=eject_${VERSION}_linux_amd64.tar.gz      # or linux_arm64
curl -fLO "https://github.com/loaiza000/eject/releases/download/$VERSION/$ASSET"
curl -fLO "https://github.com/loaiza000/eject/releases/download/$VERSION/SHA256SUMS"
grep " $ASSET\$" SHA256SUMS | sha256sum -c -    # must print "<asset>: OK"
tar -xzf "$ASSET" eject
./eject version
mkdir -p ~/.local/bin && mv eject ~/.local/bin/  # optional; ensure ~/.local/bin is on PATH
```

### macOS

```sh
VERSION=v0.1.0
ASSET=eject_${VERSION}_darwin_arm64.tar.gz     # Intel Macs: darwin_amd64
curl -fLO "https://github.com/loaiza000/eject/releases/download/$VERSION/$ASSET"
curl -fLO "https://github.com/loaiza000/eject/releases/download/$VERSION/SHA256SUMS"
grep " $ASSET\$" SHA256SUMS | shasum -a 256 -c -
tar -xzf "$ASSET" eject
./eject version
```

The binaries are not signed or notarized by Apple. Files downloaded with `curl` are normally not quarantined. If you downloaded the archive with a browser and macOS refuses to open `eject`, verify the checksum first, then remove the quarantine attribute from that one file: `xattr -d com.apple.quarantine ./eject`.

macOS builds are cross-compiled and have not yet been executed on a Mac for v0.1.0. Please report results in an issue.

### Windows (PowerShell)

```powershell
$Version = "v0.1.0"
$Asset = "eject_${Version}_windows_amd64.tar.gz"   # Windows on ARM: windows_arm64
Invoke-WebRequest "https://github.com/loaiza000/eject/releases/download/$Version/$Asset" -OutFile $Asset
Invoke-WebRequest "https://github.com/loaiza000/eject/releases/download/$Version/SHA256SUMS" -OutFile SHA256SUMS
$Expected = ((Select-String -Path SHA256SUMS -Pattern " $([regex]::Escape($Asset))$").Line -split '\s+')[0]
$Actual = (Get-FileHash $Asset -Algorithm SHA256).Hash
if ($Actual -ne $Expected.ToUpper()) { throw "checksum mismatch: $Actual" } else { "checksum OK" }
tar -xzf $Asset eject.exe
.\eject.exe version
```

`tar` is built into Windows 10 (1803) and later. The executable is not code-signed. If SmartScreen warns about a browser download, confirm the checksum before choosing to run it. Administrator rights are not required; run EJECT as a standard user.

To call `eject` from any directory, move `eject.exe` to a folder such as `$env:LOCALAPPDATA\Programs\eject` and add that folder to your user `PATH` (Settings → System → About → Advanced system settings → Environment Variables).

### WSL

Use the Linux archive inside WSL and keep generated projects on the Linux filesystem (for example under `~`). Windows drives mounted at `/mnt/c` usually cannot enforce private Unix permissions, so EJECT refuses to write projects there.

## Build from source

Requires Go 1.27.3 and Git. AWS and Docker are not needed.

```sh
git clone https://github.com/loaiza000/eject.git
cd eject
go build -o eject ./cmd/eject          # Windows: go build -o eject.exe ./cmd/eject
./eject version                        # "eject dev"
```

## First run

```sh
./eject doctor
./eject inspect  --manifest examples/aws-basic/manifest.json --output out/infrastructure.json
./eject plan     --input out/infrastructure.json --output out/plan.json
./eject generate --plan out/plan.json --output out/project
./eject verify   --project out/project
```

These steps use the synthetic example in the repository. For the commands above, clone the repository or download its source archive from the release page. See the [README quick start](../README.md#quick-start-offline-about-one-minute) for the expected output.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `output exists; use --force to replace it` (exit 3) | `inspect`/`plan` never overwrite silently. Choose a new path or add `--force`. |
| `create output without overwrite` or `output directory differs from generated files` (exit 3) | `generate` writes only to a new directory, or accepts an identical earlier generation. Use a new `--output` directory. |
| `filesystem did not enforce private ... permissions` | The target filesystem ignores Unix modes (common on `/mnt/c` in WSL, some network or USB drives). Write to a local Linux filesystem path. |
| `existing output has unsafe access permissions` | An existing directory grants access to other users (on Windows, a folder created by another tool with inherited ACLs). Let `generate` create a new directory. |
| `symlink in output path` | EJECT refuses paths that traverse symbolic links. Use the real path. |
| `plan differs from recalculated compatibility and steps` (exit 2) | The plan was edited or produced by a different EJECT version. Regenerate it with `eject plan`. |
| `verify` reports `COMPOSE_SYNTAX: SKIPPED` | Docker Compose is not installed or not reachable. Static file checks still ran. |
| `RUNTIME_CONFIG: FAILED` | The private `.env` in the project lacks a required value. Copy `.env.example` to `.env` and fill in every variable. |
| `STARTUP: FAILED` | An image is missing locally (pass `--allow-pull` to permit downloads), a service is unhealthy, or Docker is not running. |
| `macOS cannot verify the developer` | See the macOS section above. |
