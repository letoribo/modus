# Change Log

# UNRELEASED

- Update metrics collection to remove labels [#163](https://github.com/gohypermode/runtime/pull/163)
- Add environment and version to health endpoint [#164](https://github.com/gohypermode/runtime/pull/164)

# 2024-04-29 - Version 0.6.2

- Traces and non-user errors are now sent to Sentry [#158](https://github.com/gohypermode/runtime/issues/158)
- Fix OpenAI text generation [#161](https://github.com/gohypermode/runtime/issues/161)

# 2024-04-26 - Version 0.6.1

- Fix GraphQL error when resulting data contains a nested null field [#150](https://github.com/gohypermode/runtime/issues/150)
- Fix GraphQL error when resolving `__typename` fields; also add `HYPERMODE_TRACE` debugging flag [#151](https://github.com/gohypermode/runtime/issues/151)
- Collect metrics and expose metrics and health endpoints [#152](https://github.com/gohypermode/runtime/issues/152)
- Add graceful shutdown for HTTP server  [#153](https://github.com/gohypermode/runtime/issues/153)
  - Note: It works correctly for system-generated and user-generated (`ctrl-C`) terminations, but [not when debugging in VS Code](https://github.com/golang/vscode-go/issues/120).
- Add version awareness [#155](https://github.com/gohypermode/runtime/issues/155)

# 2024-04-25 - Version 0.6.0

Baseline for the change log.

See git commit history for changes for this version and prior.