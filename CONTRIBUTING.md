# Contributing

Thanks for your interest in contributing to Uptime-Monitor!

## Ways to contribute

- Report bugs or request features via GitHub Issues
- Improve documentation (including `docs/`)
- Submit pull requests with fixes or enhancements

## Development workflow

1. Fork the repository and clone it.
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make changes and run checks:

   ```bash
   go vet ./...
   go test ./...
   ```

4. Verify the binary builds for the default target:

   ```bash
   bash build/build.sh
   ```

5. Commit with a clear message and push; open a Pull Request against `main`.

## Style & conventions

- Keep the public surface small; implementation lives under `internal/`.
- Comments are written in Chinese in this project (keep consistent).
- New functionality should be configurable via YAML + `UM_*` env overrides
  (see `internal/config/config.go`).
- Add or update unit tests for changed logic.

## Commit message guidelines

Use conventional prefixes:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `refactor:` code change that neither fixes a bug nor adds a feature
- `test:` adding or updating tests
- `build:` build system or dependencies

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).
