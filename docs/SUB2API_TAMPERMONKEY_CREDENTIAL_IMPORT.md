# Sub2API Tampermonkey Credential Import

The userscript is served by the frontend at:

```text
/sub2api-credential-exporter.user.js
```

It is a browser-side export helper, not an upstream authorization protocol.
The target Sub2API instance does not need any new route or deployment change.

## Data Flow

1. The script waits until the current origin contains the standard Sub2API
   `auth_token` and `refresh_token` local-storage entries.
2. It verifies the current site through the existing public settings endpoint.
3. Export runs only after the user clicks the button and confirms the warning.
4. The script opens an in-page result panel with token status and up to five
   cookie names/scopes (never cookie values). The user explicitly clicks the
   copy button; if clipboard access is blocked, the complete JSON can be shown
   and copied manually. The script performs no cross-origin request and does
   not upload credentials automatically.
5. Provider management validates the bundle format and source origin, fills the
   existing token-pair form, and clears the raw bundle from component memory.
6. The existing backend path encrypts and persists only the access and refresh
   tokens. Cookie values are not sent to or stored by Sub2API.

Use a dedicated or temporary browser profile for the upstream login and close
that session after import. Sub2API refresh tokens rotate; continuing to use the
exporting browser and the provider manager at the same time can make either
side lose the refresh-token race and require a new import.

## Cookie Boundary

The script uses `GM_cookie.list` when Tampermonkey exposes it and falls back to
`document.cookie`. A normal Sub2API login is primarily stored in the
`auth_token` and `refresh_token` local-storage entries, so a successful export
with zero cookies is valid and does not affect token-pair import. Tampermonkey
documents HttpOnly-cookie access as Beta-only, so stable releases may not expose
those cookies. Each exported cookie carries its actual `httpOnly` flag and the
bundle states the capture method. The management UI reports only counts and
deliberately discards cookie values after extracting the token pair.

Cloudflare cookies are not durable server credentials. A `cf_clearance` value
can be bound to the browser IP, User-Agent, and challenge state; importing the
token pair avoids interactive login but does not bypass a Cloudflare rule that
blocks subsequent API calls from the Sub2API server.
