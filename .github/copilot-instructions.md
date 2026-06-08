# Copilot Instructions

## Commands

```shell
make test          # run unit tests
make testacc       # run acceptance tests (requires real App Store Connect credentials)
make lint          # run golangci-lint
make fmt           # format code with gofmt
make generate      # regenerate docs (must be committed — CI checks for drift)
make build         # compile the provider
```

Run a single test:
```shell
go test ./internal/provider/ -run TestUserResource_Update_SetsVisibleAppsNullWhenAllAppsVisible -v
```

## Architecture

This is a [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) provider. All provider logic lives in `internal/provider/`.

The provider wraps the [`appstore-go`](https://github.com/oliver-binns/appstore-go) SDK client, which handles App Store Connect API authentication (JWT via `.p8` private key) and HTTP calls. The provider itself never calls the API directly.

Currently one resource is implemented: `appstoreconnect_user` (`user_resource.go`). New resources follow the same pattern:
- Implement `resource.Resource` (CRUD) + `resource.ResourceWithValidateConfig` + `resource.ResourceWithModifyPlan`
- Register in `provider.go` → `Resources()`

## Key Conventions

### TDD is preferred
Write a failing unit test before implementing a fix or feature. Unit tests in `*_unit_test.go` use `mockUserClient` and run without network access — prefer these for all logic and edge cases.

### `populateState` is the single source of truth for API → state mapping
After any API call (Create, Read, Update), always use `r.populateState(ctx, &data, user, resp.Diagnostics)` to write the response into state. Do **not** assign fields manually. This ensures special-case logic (e.g. setting `visible_apps` to null when `all_apps_visible` is true) is applied consistently everywhere.

### Two test layers
- `*_unit_test.go` — fast, no network, use `mockUserClient` to stub the API.
- `*_test.go` — acceptance tests that hit real App Store Connect. Require `TF_ACC=1` and env vars `TF_VAR_issuer_id`, `TF_VAR_key_id`, `TF_VAR_private_key`.

### `ValidateConfig` for mutually exclusive attributes
Config-level validation (e.g. `all_apps_visible = true` must not be combined with `visible_apps`) lives in `ValidateConfig`, not in Create/Update.

### `ModifyPlan` for forced replacement
`ModifyPlan` detects when a user hasn't accepted their invite yet and marks all attributes as requiring replacement, with a warning diagnostic.

### Conventional Commits required
All PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/) — this drives automated versioning. Enable the repo's Git hooks to catch this locally:
```shell
git config core.hooksPath hooks
```
