# Solovey Telegram Component

Administrative Telegram notifications and encrypted backup delivery for Solovey. This is a statically compiled feature library and a first-party UI contribution. It adds no daemon, listener, settings database or updater.

## Capabilities

- Bounded notification queue, redaction, finite retries and cancellation/drain.
- CPU alert hysteresis, scheduled reports and scheduled backup delivery through the common scheduler.
- Explicit, bounded chat detection from pending Telegram updates without consuming or acknowledging them. Chat IDs remain decimal strings.
- The existing /telegram settings page, Classic and Nexus navigation, encrypted-backup option and six locale contributions.

The administrative bot is independent of the Paid Subscriptions customer/payment bot. Both use the same non-running Bot API helper in solovey-host. Paid does not import this feature or depend on its lifecycle.

## Go integration

The root Manifest function reads component.json through the canonical shared Host parser. It performs no registration or background work. The product's existing ComponentHost admits the component and supplies its selected ports. StartScheduledJobs composes only Telegram's jobs in that scheduler; stopping removes and drains them. Importing this module starts nothing.

The service package accepts the existing scheduler.OperationLease and an approved HTTP client provider. Settings interfaces contain only Telegram-specific reads; the Panel adapter retains encrypted storage and authorized SecretRef resolution. Missing admission fails closed. The settings package owns unchanged keys, defaults, validation, aliases and field definitions. Proxy/SSRF policy remains the existing product owner and is injected for proxy-value validation.

The Backup owner creates and encrypts a consistent snapshot and supplies an accepted encrypted stream. Telegram owns delivery policy, size checks, filenames/captions and actual-outcome audit records. Database admission, encryption/envelope compatibility, restore transactions and filesystem access remain the product's Backup adapters. There is no generic DB, shell or filesystem port.

Gin routes, sessions, API-token scope, MFA/CSRF, rate limits, audit persistence, settings writes, decrypt-backup CLI registration and Drop Data authorization remain product adapters. The existing Telegram endpoints, manifest identity and durable settings are preserved.

## Frontend build input

frontend contains the sole editable Telegram UI implementation. It imports the public @solovey/component-ui API and normal Vue dependencies. HOST_UI_PIN.json binds the current UI API owner commit, API version and lock/export digests. Product builds pin this module and the complete frontend inventory, materialize verified generated input, and compile it through the same Vite application. Dynamic imports, CSS and locale closure are packaged by the existing product producer. Core/minimal profiles exclude the optional feature.

This is trusted first-party build-time code. A new source commit is a candidate for product integration, not an automatically installed component update. In-process updates require an accepted product build and the existing release channel.

## Testing

Use Go 1.26.10 with GOWORK=off and GOFLAGS=-mod=readonly:

    go test -race -count=1 ./... github.com/MalenkiySolovey/solovey-host/componentkit/telegram
    go vet ./...
    node scripts/check-source.mjs

For frontend tests, check out the exact HOST_UI_PIN.json product commit into .host-ui, run npm ci in .host-ui/frontend, then run:

    node scripts/check-frontend.mjs .host-ui/frontend

The checker verifies the source identity and reuses that checkout's locked tools for type-checking and the original UI tests. A development checkout can be supplied explicitly if it has the exact accepted identity. Tests use local deterministic HTTP fixtures and synthetic credentials. Source CI runs on Ubuntu and Windows; final product qualification owns Full/Core composition, HTTP authorization, real Backup/Restore adapters and browser rendering.

## Compatibility and licensing

Component ID telegram; component manifest version 1; delivery in-process; Host API solovey.component-host/v1; UI API 1.0. The module requires solovey-host v0.2.0. Component module SemVer is independent of product versioning. Existing setting/secret keys and encrypted data are unchanged.

Licensed under GPL-3.0-only. LICENSE and NOTICE retain the original terms and attribution. SOURCE_PROVENANCE.json traces original source blobs, authors and complete per-file history. The isolated filtered Telegram history is preserved in Git ancestry; shared Host code retains its separate source owner.
