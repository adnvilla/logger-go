## [1.3.0](https://github.com/adnvilla/logger-go/compare/v1.2.0...v1.3.0) (2026-09-29)

### Features

* add NewProduction to emit the production schema ([#47](https://github.com/adnvilla/logger-go/issues/47)) ([e1782a5](https://github.com/adnvilla/logger-go/commit/e1782a53ba2963fd747e4607b9320be63044f179)), closes [#27](https://github.com/adnvilla/logger-go/issues/27) [#11](https://github.com/adnvilla/logger-go/issues/11)
* define the production log schema ([#43](https://github.com/adnvilla/logger-go/issues/43)) ([7a0acbc](https://github.com/adnvilla/logger-go/commit/7a0acbccb423bcfe7751d5b138a00c383c6bc36a)), closes [#11](https://github.com/adnvilla/logger-go/issues/11)
* **otel:** correlate logs with OpenTelemetry traces ([#46](https://github.com/adnvilla/logger-go/issues/46)) ([642833c](https://github.com/adnvilla/logger-go/commit/642833c8701350f6bfece80c68c614fee115ea61)), closes [#24](https://github.com/adnvilla/logger-go/issues/24)
* redact sensitive attributes in a handler middleware ([#45](https://github.com/adnvilla/logger-go/issues/45)) ([88f1bcd](https://github.com/adnvilla/logger-go/commit/88f1bcdb587cef262b448a00acc78d1f071024dd)), closes [#25](https://github.com/adnvilla/logger-go/issues/25)
* serialize errors following the production schema ([#44](https://github.com/adnvilla/logger-go/issues/44)) ([80083db](https://github.com/adnvilla/logger-go/commit/80083db394124706272875230a1672723a43fb09)), closes [#26](https://github.com/adnvilla/logger-go/issues/26)

## [1.2.0](https://github.com/adnvilla/logger-go/compare/v1.1.0...v1.2.0) (2026-09-29)

### Features

* carry request attributes in context through a handler middleware ([#41](https://github.com/adnvilla/logger-go/issues/41)) ([adb4d5e](https://github.com/adnvilla/logger-go/commit/adb4d5e8a4d273c390d4d780142b74d65567b1a2)), closes [#23](https://github.com/adnvilla/logger-go/issues/23)
* separate default-logger configuration from context storage ([#40](https://github.com/adnvilla/logger-go/issues/40)) ([1eed80c](https://github.com/adnvilla/logger-go/commit/1eed80cdc05a04b687036d69f6fba1a9a340348e)), closes [#10](https://github.com/adnvilla/logger-go/issues/10)

## [1.1.0](https://github.com/adnvilla/logger-go/compare/v1.0.1...v1.1.0) (2026-09-29)

### Features

* **zap:** delegate the slog bridge to zapslog ([#35](https://github.com/adnvilla/logger-go/issues/35)) ([e2f82bf](https://github.com/adnvilla/logger-go/commit/e2f82bf10e701a2c83b61b973352d158bf1648ac)), closes [#19](https://github.com/adnvilla/logger-go/issues/19) [#7](https://github.com/adnvilla/logger-go/issues/7) [#8](https://github.com/adnvilla/logger-go/issues/8)

### Bug Fixes

* report the real call site from the level helpers ([#36](https://github.com/adnvilla/logger-go/issues/36)) ([a119640](https://github.com/adnvilla/logger-go/commit/a1196406312a211c9c3057a8cb01cde1257bd3bc)), closes [#20](https://github.com/adnvilla/logger-go/issues/20) [#9](https://github.com/adnvilla/logger-go/issues/9)

## [1.0.1](https://github.com/adnvilla/logger-go/compare/v1.0.0...v1.0.1) (2026-09-06)

### Bug Fixes

* propagate context to slog handlers ([#16](https://github.com/adnvilla/logger-go/issues/16)) ([fc38886](https://github.com/adnvilla/logger-go/commit/fc388865bb17cbf50ae1e6e83ab8ec82499e6c80))

## 1.0.0 (2025-10-14)

### ⚠ BREAKING CHANGES

* Initial release

* Add-changelog ([#3](https://github.com/adnvilla/logger-go/issues/3)) ([1f7f275](https://github.com/adnvilla/logger-go/commit/1f7f275ab411e4512eb6c9a125e0ab247d0e5c76))
