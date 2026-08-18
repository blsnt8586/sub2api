# Sub2API Provider Health Timeline Plan

**Date:** 2026-08-14
**Status:** Implemented and verified
**Scope:** Sub2API upstream provider list, account-level probes, probe history, overview API, and scheduler cadence

## Goals

1. Add an honest 24-hour health timeline to every Provider panel.
2. Separate control-plane availability from real data-plane evidence so an idle but reachable Provider is not presented as fully verified or fully unknown.
3. Reduce the Provider panel height while surfacing more useful operational data.
4. Replace per-card health requests with one bounded batch overview request.
5. Keep the change migration-free by aggregating the existing append-only probe run table.
6. Make data-plane evidence explicit per linked account, with independent opt-in and status.
7. Keep raw failure details out of summary cards while retaining them for on-demand diagnosis.

## Non-goals

- Do not enable data-plane or media probes automatically.
- Do not calculate or advertise contractual SLA uptime.
- Do not infer healthy state across missing samples.
- Do not change existing Provider, account, billing, or generation behavior.
- Do not add or modify database tables in this change.
- Do not run real data-plane probes implicitly while viewing or editing the settings dialog.

## Status Semantics

The card displays two independent dimensions:

| Dimension | Source | Meaning |
|-----------|--------|---------|
| Control availability | `control_status`, with existing failure/recovery hysteresis | Login, `/health`, Keys, and Groups endpoints are reachable and responsive. |
| Real-chain evidence | `overall_status`, `data_status`, and `traffic_status` | A real model request or recent production traffic has verified the data path. |

The timeline is explicitly labelled as a **control-plane** timeline. It must not use `overall_status` as the green-state source because the existing overall status intentionally remains `unknown` when control checks pass but no data-plane or traffic evidence exists.

## Timeline Contract

- Window: rolling 24 hours.
- Bucket count: 48.
- Bucket width: 30 minutes.
- Bucket state:
  - `healthy`: at least one healthy control sample and no degraded/unhealthy control sample.
  - `degraded`: a degraded control sample, or an unhealthy control sample that has not reached the persisted unhealthy state.
  - `unhealthy`: at least one control sample whose persisted overall state is unhealthy.
  - `unknown`: no control-plane sample in the bucket.
- Multiple samples in one bucket use the worst state.
- Missing buckets remain unknown; the UI never carries a previous healthy state forward.
- Each bucket also returns sample counts, status counts, maximum `/health` latency, and the last error for inspection.

The UI displays a compact textual summary alongside the strip so color is not the only status signal. The strip is one keyboard focus target; Left/Right/Home/End select a bucket and expose the same detail available on pointer hover.

## Backend Design

Add a batch endpoint:

```text
GET /api/v1/admin/sub2api-providers/health-overview?ids=1,2,3
```

Constraints:

- Positive, unique Provider IDs only.
- Maximum 100 IDs per request.
- Empty `ids` returns an empty list.
- A single repository query loads runs for all requested Providers inside the fixed 24-hour window.
- Existing `(provider_id, created_at)` index supports this access pattern.

Each overview contains:

- Provider ID.
- Latest health sample in the 24-hour window.
- Window start/end and bucket duration.
- 48 ordered buckets.
- Healthy, degraded, unhealthy, and unknown bucket totals.

Each new probe run also stores additive JSON details for the selected accounts:

- `account_id`, display name, platform, status, latency, checked time, error category, and raw error detail.
- `data_probe_enabled`, effective data interval, and selected account count.
- When a control-only run is due before the next data run, the latest account snapshot is carried forward so the UI does not regress to `unknown`.

The same platform may have multiple selected accounts. Account selection is an independent toggle; a media account that is not allowed to create a real task is reported as `disabled`, not as an upstream failure.

Probe history accepts `since_seconds` (default `3600`, bounded to 60-86400) and remains append-only. The UI shows only the last hour by default and reveals raw errors only after an individual history row is expanded.

The Provider list remains usable if overview loading fails; the frontend renders a stable no-data state and does not issue per-card fallbacks.

## Scheduler Correction

The current minute cron plus per-run jitter can extend a nominal 300-second interval to roughly six minutes. Change the runner scan cadence to 15 seconds and consider a probe due after the configured interval itself. This keeps the configured interval truthful within one 15-second scan tick without changing any stored configuration.

## Frontend Design

Target panel height: approximately 340-380px at 1280px desktop and 375px mobile widths.

Information order:

1. Provider name, enabled state, and overflow menu.
2. Provider protocol, URL, linked-account count, and one-line optional note.
3. Latest control-plane state, `/health` latency, and relative check time.
4. Fixed-size 24-hour status strip with bucket summary.
5. Real-chain evidence row: traffic metrics when present; otherwise one explicit no-traffic/data-probe message.
6. Compact sync/path state and three actions: linked accounts, probe history/settings, and run now.

Density rules:

- Remove the duplicate linked-account count.
- Do not reserve two lines for an absent note.
- Do not render empty success-rate and P95 metric columns when there is no traffic.
- Replace the full updated-time cell with relative operational timestamps and exact-time tooltips.
- Keep controls at least 44px high and preserve visible keyboard focus.
- Use existing semantic light/dark tokens and fixed strip dimensions to avoid layout shift.

Responsive grid:

- One column on mobile.
- Two columns from `md`.
- Three columns from `xl`.
- Four columns from `2xl`, only after the compact panel is in place.

## Files

Backend:

- `backend/internal/service/sub2api_provider_probe_service.go`
- `backend/internal/repository/sub2api_provider_probe_repository.go`
- `backend/internal/handler/admin/sub2api_provider_handler.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/service/sub2api_provider_probe_service_test.go`

Frontend:

- `frontend/src/api/admin/sub2apiProviders.ts`
- `frontend/src/components/admin/Sub2APIProviderHealthTimeline.vue`
- `frontend/src/components/admin/Sub2APIProviderCard.vue`
- `frontend/src/components/admin/__tests__/Sub2APIProviderCard.spec.ts`
- `frontend/src/components/admin/__tests__/Sub2APIProviderHealthTimeline.spec.ts`
- `frontend/src/views/admin/Sub2APIProvidersView.vue`
- Provider i18n locale files.

## Verification

Backend:

- Unit-test bucket boundaries, worst-state aggregation, no-data buckets, latest state, and configured interval behavior.
- Run the Provider probe service unit tests.
- Run backend unit tests.

Frontend:

- Unit-test timeline summary, status classes, pointer detail, and keyboard navigation.
- Unit-test compact card states with and without traffic evidence.
- Run TypeScript checks and the production build.

Browser:

- Verify 1280x720 and 1440x900 desktop layouts.
- Verify 375x812 mobile layout without horizontal scrolling.
- Verify light and dark modes.
- Verify bucket hover/focus detail, card actions, and loading/no-data states.
- Verify history rows hide raw errors until expanded and default to the last hour.
- Verify two linked accounts can be enabled together and each row reports its own probe state.

## Rollout and Compatibility

- API change is additive.
- No schema generation or migration is required.
- Existing probe history and configuration endpoints remain compatible; history only gains optional query parameters.
- `probe/history` gains an optional bounded `since_seconds` query; the default remains backward compatible at one hour in the admin UI.
- Existing data-plane and media probe defaults remain disabled.
- Rollback only requires reverting application code; no database rollback is needed for this change.

## Implementation Result

- The Provider list now loads with one list request and one health-overview request; it no longer issues one health request per card.
- Every overview returns 48 fixed 30-minute buckets, plus separate latest control availability and latest real-chain evidence.
- The runner scans every 15 seconds and uses the configured interval without extra per-Provider jitter.
- Measured card height is 340-356px at 1440px and 340-355.5px at 375px; neither viewport has horizontal overflow.
- Light, dark, English, and Chinese states were checked in the browser. The timeline exposes one focus target and responds to Left/Right/Home/End.
- Backend unit tests, focused frontend component tests, TypeScript checking, and the production frontend build pass.
- No Ent schema was changed for this timeline and no migration was added by this implementation.
- Account-level probe results use the existing `details` JSONB column; no migration or destructive history cleanup is required.
