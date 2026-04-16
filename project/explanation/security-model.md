# Security model

## CSRF protection via Fetch Metadata

drynn protects against Cross-Site Request Forgery (CSRF) using [Fetch
Metadata request headers](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Sec-Fetch-Site)
instead of the traditional synchronizer-token pattern. Every state-changing
request (POST, PUT, PATCH, DELETE) passes through
`internal/middleware.FetchMetadata`, which delegates the core same-origin
check to Go's standard-library `net/http.CrossOriginProtection` (Go 1.25+)
and layers a strict default-reject on top.

### Why not CSRF tokens?

Token-based CSRF is a cross-cutting concern: you generate a token, thread it
through session state, render it into every form, and validate it on every
state-changing handler. For a single-droplet template that otherwise keeps
its handlers and templates small and flat, that is a lot of machinery at a
lot of seams.

Fetch Metadata puts the same protection at a single seam: one middleware on
one chain. OWASP's CSRF cheat sheet and Filippo Valsorda both endorse Fetch
Metadata as a complete alternative to synchronizer tokens for applications
targeting modern browsers. For drynn's audience and scale, it is the
lighter and more consistent choice.

### What the middleware allows

- All safe methods (GET, HEAD, OPTIONS). These never trigger a check; the
  standard library documents them as safe and applications must not perform
  state changes in response to them.
- State-changing requests with `Sec-Fetch-Site: same-origin`. This is what
  a legitimate form submission from a page we served looks like.
- State-changing requests with `Sec-Fetch-Site: none`. This covers
  navigations a user initiates directly (address bar, bookmark, shortcut).
  `none` only appears on top-level navigations that were not triggered by
  another page, which is exactly the "user really meant it" case.
- State-changing requests whose `Origin` header host matches the request
  `Host` header, per the standard library's fallback path.

### What the middleware rejects

- Any state-changing request with `Sec-Fetch-Site: cross-site` or
  `same-site` (not `same-origin`). These are cross-origin browser requests
  by definition.
- **Any state-changing request that arrives without a `Sec-Fetch-Site`
  header at all.** This is a strict default-reject that goes beyond the
  standard library's behavior. All modern browsers have sent
  `Sec-Fetch-Site` since 2023 (Chrome 76 in 2019, Firefox 90 in 2021,
  Safari 16.4 in 2023). A state-changing request without the header is
  either a non-browser client (curl, scripts, abandoned tooling) or a
  browser old enough that the template is not trying to serve it. Rejecting
  both cases is the desired posture for a template targeting modern
  browsers, and it removes a class of "well, what if…" questions from the
  reviewer.

### Operational notes

- There is no allow-list of trusted origins. Everything runs on one
  droplet behind one hostname; there is no legitimate cross-origin caller
  to exempt. If a fork ever adds one (an embedded SPA on a different
  subdomain, a partner integration), use `CrossOriginProtection.AddTrustedOrigin`
  rather than weakening the global check.
- Rejection is HTTP 403 with a short plain-text body. No flash message,
  no redirect — a legitimate user with a modern browser should never see
  this; if they do, the request was not one the application should
  silently paper over.
- If integration tests or local tooling turn out to need state-changing
  requests without `Sec-Fetch-Site`, the strict top-up is a single `if`
  block in `internal/middleware/fetchmetadata.go` and can be removed
  without touching the stdlib-backed path below it.
