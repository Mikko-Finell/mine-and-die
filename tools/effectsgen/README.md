# effectsgen

`effectsgen` is a Go command that will bridge the authoritative server contracts and the TypeScript client bindings described in the architecture docs. The implementation work is tracked in [`docs/architecture/effectsgen-roadmap.md`](../../docs/architecture/effectsgen-roadmap.md) and [`docs/architecture/effectsgen-spec.md`](../../docs/architecture/effectsgen-spec.md).

## Layout

```
tools/effectsgen/
├── cmd/effectsgen/      # CLI entry point wired up with Cobra-style patterns
├── internal/cli/        # Flag parsing and top-level command orchestration
└── internal/pipeline/   # Future packages for contract loading and generation stages
```

Each package currently holds scaffolding so contributors can flesh out the pipeline without reworking the project structure. Follow standard Go workspace conventions (`cmd/` for binaries, `internal/` for implementation packages) when adding new code.

## Status

The command does not perform any work yet. Contributors should start from the roadmap tasks before adding behaviour here.
