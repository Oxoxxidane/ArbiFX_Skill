# Distribution Validation Record

Date: 2026-09-14. Toolchain: Go 1.27.1 windows/amd64. Standard library only, with CGO_ENABLED=0.

- Source go vet and go test passed.
- The standalone Windows amd64 executable passed tests for all 23 commands against random-port localhost HTTP fixtures, checking POST paths, JSON fields, and UTF-8 Content-Length.
- The same executable passed dry-run/no-connection, Unicode/multiline/file/stdin, configuration precedence, secret exclusion, instance resolution, unknown-command, input-boundary, API/script/network/timeout, proxy bypass, redirect rejection, and no-replay tests.
- The standalone executable passed offline doctor from a temporary working directory with no Python/Node/Go on PATH. It does not depend on the source directory.
- Read-only status and instances requests succeeded against the local AE host, confirming online/port, nested comp/layer objects, and zero-based indices. Start was not running during that check; live Start writes, submission, save, and load were not performed. Their coverage comes from HTTP fixtures.
- Windows ARM64, macOS Intel/ARM64, and Linux amd64/ARM64 binaries were cross-compiled. They have not been run on native machines with those OS/CPU combinations. Successful compilation does not establish host integration on those platforms.
- The project's HTTP listener currently has a Windows-only server implementation. Cross-platform client binaries do not change server platform support. The original ArbiFX3D project was not modified by this task.

The English edition updates skill instructions, references, UI metadata, and CLI-authored messages. Unicode test fixtures intentionally retain non-English input to verify content preservation. Rebuilds rerun source and native-executable tests. Update statements about live AE/Start and other native platforms only after performing those checks.
