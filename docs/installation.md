# Installation

EJECT v0.1.0 is still under review. No release archive is published. These commands build the current review branch from source with Go 1.27.3; they do not require AWS or Docker.

## Windows (PowerShell)

```powershell
git clone --branch codex/v0.1.0 https://github.com/loaiza000/eject.git
cd eject
go build -o eject.exe ./cmd/eject
./eject.exe version
./eject.exe doctor
```

## Linux and macOS (shell)

```sh
git clone --branch codex/v0.1.0 https://github.com/loaiza000/eject.git
cd eject
go build -o eject ./cmd/eject
./eject version
./eject doctor
```

Run the [synthetic offline example](../README.md#install-and-run-from-source) after building. Runtime verification is optional and requires Docker, trusted local images, and explicit `--allow-start`; image downloads require separate `--allow-pull` authorization.

After a release is approved and published, download the archive matching your OS and CPU architecture plus `SHA256SUMS` from GitHub Releases. Compare the archive's SHA-256 hash with its entry in `SHA256SUMS` before extracting. On Linux use `sha256sum ARCHIVE`; on macOS use `shasum -a 256 ARCHIVE`; on Windows use `Get-FileHash ARCHIVE -Algorithm SHA256`. Then run `tar -xzf ARCHIVE` and invoke the included `eject_linux_amd64`, `eject_darwin_arm64`, or `eject_windows_amd64.exe` variant matching your platform. Other supported archives use the corresponding `amd64` or `arm64` suffix. The archives include EJECT's license and third-party notices.
