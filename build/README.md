# Build

Packaging and distribution inputs. Nothing here is imported by the Go module.

- Inno Setup script for the Windows installer
- Dockerfile and compose for self-hosters who prefer containers
- goreleaser config for tagged releases

`make dist` cross-compiles the binaries; this directory is what wraps them.
