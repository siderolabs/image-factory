# Authentication

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

The factory authenticates callers with one of two providers, selected by `authentication.provider`.
See [Configuration](configuration.md) for the full list of settings.

Only one provider is active at a time.
There is no mode that accepts both htpasswd credentials and Auth0 tokens.

## The `Authorization` header

Every endpoint marked `Auth: required` in the [API reference](api.md#authentication) takes its credential in the `Authorization` request header.
What that header may contain depends on the active provider:

| Provider   | Accepted `Authorization` value                          | Credential is                                     |
| ---------- | ------------------------------------------------------- | ------------------------------------------------- |
| `htpasswd` | `Basic base64(<username>:<password>)`                   | a user from the `htpasswd` file                   |
| `auth0`    | `Bearer <jwt>`, or `Basic base64(<any-username>:<jwt>)` | an Auth0 access token for the configured audience |

Details that decide whether a request authenticates:

- The scheme is matched case-insensitively, so `bearer <jwt>` works.
  Any other scheme, `Token <jwt>` for example, is not a credential and the request is treated as unauthenticated.
- Under `auth0` the Basic form carries the JWT in the **password** field and the username is ignored; it exists for OCI and Talos registry clients, which only speak Basic auth.
  So `docker login -u unused -p <jwt> factory.example.com` is a valid way to present a token.
- Only one `Authorization` value is read.
  Sending Basic and Bearer as two separate header values authenticates with the first one only.
- URL userinfo is the same header by another name: HTTP clients encode `https://<user>:<password>@factory.example.com/...` into `Authorization: Basic`.
  That is how `GET /pxe/...` is authenticated, since iPXE cannot set request headers.
- A missing or invalid credential is `401`.
  With `htpasswd`, the response carries one `WWW-Authenticate: Basic` challenge.
  With `auth0`, it carries separate Basic and Bearer challenges, in that order.
  The Auth0 order is deliberate: OCI clients take the first scheme they recognize, and only the Basic form carries a usable token for those clients.
  With [browser login](#browser-login) configured, a page navigation is sent to `/login` with a `303` instead, and an XHR gets the `401` with no challenge on it.

There are two alternatives to this header: the [`?token=` query parameter](#the-token-query-parameter), accepted on image downloads and PXE scripts only, and the session cookie [browser login](#browser-login) issues.
The factory also issues credentials of its own, which the header carries too; see [API tokens](#api-tokens).

## htpasswd

Basic authentication against an `htpasswd` file.
The caller identity is the username.

## auth0

Bearer token authentication against an Auth0 tenant.
Tokens are validated locally against the tenant's JWKS: signature, issuer, audience and expiry.

The caller identity is the token's `org_id` claim, so tokens must be issued to organization-scoped clients.
Tokens without the claim are rejected, since there would be no principal to attribute the request to.

Tokens are accepted either as `Authorization: Bearer <token>` or in the password field of a Basic credential; see [The `Authorization` header](#the-authorization-header).

Interactive login is opt-in; see [Browser login](#browser-login).
Until it is configured, a browser hitting an authenticated route gets a `401` with a Basic challenge, not a redirect to Auth0, so a person must present a token in the authorization header.

### Browser login

Without it, a person opening the factory in a browser gets a `401` and nothing else.
Enabling it redirects them to the tenant's login page instead, using an OAuth2 authorization code flow with PKCE.

It is opt-in: set `clientID`, `clientSecret` and `sessionKey` together, or leave all three empty.
A partial set is rejected at startup rather than half-enabling the flow.
Bearer token authentication is unaffected either way.

One thing must be set up in the Auth0 tenant, which the factory cannot do for you: add `<http.externalURL>/callback` to the application's **Allowed Callback URLs**, and `http.externalURL` itself to its **Allowed Logout URLs**.
Auth0 rejects the round trip outright otherwise.
Both are derived from `http.externalURL`; the callback route is always `/callback` and is not configurable.

A session lasts as long as the access token it was established with, whose lifetime is set on the API in the tenant.
There are no refresh tokens: `offline_access` is not requested, so nothing renews a session in place.
When the access token expires, the next page load is sent back through `/login`, and the tenant's own SSO session normally answers it without the person entering anything.
The visible cost is that one navigation; an in-flight background request that lands on the expiry is redirected instead of completing, so the action is retried.
Raise the API's token lifetime to make this rarer.

`sessionKey` is a 32-byte AES-256 key, base64-encoded, injected through `IF_AUTHENTICATION_AUTH0_SESSIONKEY`.
Generate one with `openssl rand -base64 32`; surrounding whitespace is trimmed, so a value read from a file or a mounted secret works as-is.
Session cookies are encrypted with it, so every replica must be given the same one: a cookie issued by one replica is read by another.
Changing it signs everyone out.

Session cookies are `SameSite=Lax`, which is what stops a cross-site form from making a mutating request with the visitor's session.
There is no separate CSRF token, so relaxing that attribute — to `None` for iframe embedding, say — would need one adding first.

Responses authenticated by a session cookie are sent with `Cache-Control: no-store` and `Vary: Cookie`, since a shared cache that stored one would serve a visitor's session to whoever asked next.

Sessions are held in an encrypted cookie rather than server-side, which is what lets any replica serve any request without shared session storage.
The cookie is capped at the 4096 bytes browsers accept; a tenant configured to emit very large access tokens (a long `permissions` array, typically) can exceed it, which is reported at login rather than failing silently.

### Migrating from htpasswd

Switching an existing deployment from `htpasswd` to `auth0` changes the identity namespace from usernames to `org_...` identifiers.
Consequences:

- Schematics owned by a username are not reachable by any Auth0 organization, and vice versa.
  Existing ownership does not carry over; see [Ownership](#ownership).
- Audit history written under the old provider records usernames, so records from before and after the switch cannot be correlated by principal.
- Because only one provider is active at a time, there is no gradual rollout.
  The switch takes effect for every caller at restart.

Plan the cutover accordingly, or start a fresh deployment.

### Revocation

Access tokens are validated offline, so revoking a client at the tenant does not invalidate tokens it has already been issued.
Those remain usable until they expire; keep token lifetimes short if that matters.

The same applies to an [API token](#api-tokens) minted from a JWT.
It stays valid for its own lifetime regardless of what happens to the JWT it was obtained with.
A [stored](#stored-and-ephemeral-tokens) token can at least be taken back explicitly; an ephemeral one cannot, which is why `authentication.tokens.ttl.ephemeral` gives it a shorter lifetime policy.

## Ownership

Creating a schematic with authentication enabled stamps the caller identity into its `owner` field.
`owner` is part of the canonical schematic body, and the schematic ID is the hash of that body, so two identities submitting the same customization get two different IDs.

Reading a schematic, or any artifact derived from one, requires the `owner` to match the caller:

- A mismatch is `403`.
- An unauthenticated caller is `401`, whether or not the schematic exists, so its existence is not leaked.
- A schematic with no `owner` is `403` for every caller once authentication is enabled, since no identity matches an empty owner.

That last case covers schematics created before authentication was turned on, and the well-known public IDs such as the default schematic.
Neither is reachable in an authenticated deployment; each identity creates its own.

## API tokens

An API token is a JWT the factory issues and verifies with its own key, rather than one obtained from the configured provider.
What it may do is decided by the **scopes** it carries, not by the endpoint that minted it.

There is one token type, one active signing key and one JWKS containing every configured verification key.
A short-lived download link and a year-long node credential are the same credential with different scopes, lifetimes and storage.

### Scopes

Scopes are resource-first, atomic capabilities.
The factory recognizes exactly these values:

| Scope | Authenticates |
| --- | --- |
| `image:read` | generated image downloads, PXE scripts and generated installer OCI pulls |
| `source:pull` | proxied upstream/source OCI pulls under `/v2/siderolabs/` |
| `schematic:create` | `POST /schematics` |
| `schematic:read` | `GET` and `HEAD` on schematic definitions |
| `report:read` | `GET` and `HEAD` on SPDX, VEX and vulnerability reports |
| `token:issue` | `POST /tokens` |
| `token:read` | `GET /tokens` |
| `token:revoke` | `POST /tokens/:id/revoke` |

The route map is code-defined and is the single source of truth for self-issued credentials.

Scopes do not imply one another.
`schematic:create` does not grant `schematic:read`, and `token:issue` does not grant listing or revocation.
A token carrying several scopes reaches the union of their routes.
Ownership remains a separate mandatory check, so permission to read a schematic-derived resource never grants access to another subject's resources.

A token's lifetime and supported transports follow whether it is stored, not which scopes it carries.
See [Stored and ephemeral tokens](#stored-and-ephemeral-tokens).

### Browser actor profiles

The browser UI does not expose individual scope or delegation controls.
It offers fixed actor profiles whose scope lists are code-owned alongside the scope catalog:

| Actor | Executable scopes | Issuable scopes |
| --- | --- | --- |
| Talos | `image:read` | none |
| Automation (Omni / Terraform) | `image:read`, `report:read`, `schematic:create`, `schematic:read`, `token:issue` | the same Automation scopes |
| Operator | the non-token Automation scopes plus `source:pull` | none |
| Admin | `image:read`, `report:read`, `schematic:create`, `schematic:read`, `source:pull`, `token:issue`, `token:read`, `token:revoke` | the same Admin scopes |

Automation can issue bounded credentials for work it controls, such as a Talos or PXE pull token, and can mint a replacement Automation credential before its current credential expires.
Admin can issue every actor profile.
These are ordinary independently valid credentials rather than OAuth refresh tokens: issuing a replacement does not revoke its predecessor, and each stored token remains independently revocable until it expires.
No browser actor carries `any_subject`, so Automation and Admin remain confined to their own authenticated identity.
The HTTP API continues to accept explicit scopes for clients that need finer control.

### Stored and ephemeral tokens

Every token says, in the JWT itself, whether the factory keeps a record of it.
That is the `stored` claim, set once when the token is minted and never re-derived afterwards.

A **stored** token is written to the per-org index.
Presence in that index is what keeps it valid, which is what makes it revocable, and it is what `GET /tokens` lists.
It needs a `name`, because the name is what an operator picks it out of that list by, and it counts against `authentication.tokens.maxPerOrg`.

An **ephemeral** token is a signed string and nothing else.
Nothing records it, so it cannot be listed or revoked; expiry is the only way it leaves circulation.
It needs no name, costs no registry write, and it is the only kind read from a [`?token=` query parameter](#the-token-query-parameter).

Because the only way to withdraw an ephemeral token is to wait, stored and ephemeral tokens have separate lifetime policies.
`authentication.tokens.ttl.stored` defaults to one year with a one-year maximum; `authentication.tokens.ttl.ephemeral` defaults to five minutes with an eight-hour maximum.
The caller still chooses storage explicitly, but cannot request a lifetime outside the selected policy.

On the verification path the claim decides the work: an ephemeral token is accepted on its signature and expiry alone, and a stored one is looked up directly by its deterministic registry tag.
That makes a create or revoke visible to every replica immediately without listing every organization's tokens.

### Delegation and the bootstrap credential

Executable capabilities and delegation authority are separate JWT claims:

- `scope` says what this credential may do itself;
- `issuable_scopes` says what capabilities it may place in a child credential;
- `any_subject` is independent cross-subject authority carried only by the CLI bootstrap credential.

A token authenticated request to `POST /tokens` must carry `token:issue`.
Both the child's executable scopes and its delegation ceiling must be subsets of the parent's `issuable_scopes`:

```text
child.scope           ⊆ parent.issuable_scopes
child.issuable_scopes ⊆ parent.issuable_scopes
```

This supports explicitly bounded delegation without conflating it with the caller's own capabilities.
A parent may issue a leaf token by omitting `issuable_scopes`, or an attenuated delegating token by supplying a smaller ceiling.
Unknown values fail closed.
The API cannot grant `any_subject`, even when the caller has it.

A full htpasswd or interactive Auth0 credential has no signed delegation claim; the server treats it as authorized to issue the current code-defined catalog for its own identity.
This authority is evaluated at request time and is not a durable wildcard in a JWT.

The `admin` scope no longer exists.
Cross-subject provisioning uses the CLI-only bootstrap credential issued by the `admin-token` subcommand.
That credential has token lifecycle capabilities, an explicit snapshot of the current catalog in `issuable_scopes`, and `any_subject=true`.
It is never stored, cannot issue another credential with `any_subject`, and cannot be minted through HTTP.

### Minting a bootstrap credential

```shell
image-factory admin-token --config /etc/image-factory/config.yaml --subject org_abc123
eyJ...
```

The command exists in Enterprise builds only.
The token is written to stdout, while human-readable diagnostics go to stderr, so `> token` leaves a usable credential file.

`--subject` is required and establishes the bootstrap credential's own identity.
`--ttl` must fit `authentication.tokens.ttl.bootstrap`; this is a dedicated CLI-only lifetime policy.
`authentication.tokens.keyPaths` must contain an active private key, because an in-memory key generated by another process would not match the running replicas.

The bootstrap credential is intentionally exceptional and ephemeral.
Removing its signing key from the verification list retires it early, along with every other token signed by that key.
Merely moving a new key to the first position does not invalidate it while its old key remains configured for verification.
Keep it offline and use the shortest practical lifetime.

### Minting one

`POST /tokens`, authenticated like any other route:

```shell
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"name":"rack-3","scopes":["image:read"],"ttl":"8760h"}'
{"id":"0d1c...","name":"rack-3","scopes":["image:read"],"issuable_scopes":[],"token":"eyJ...","org_id":"org_abc123","created_at":"...","expires_at":"...","stored":true}
```

`token` is shown exactly once.
The factory does not store it and cannot show it again; only the record naming it is kept.
`stored` in the response echoes what was actually recorded.

`stored` defaults to `true`, so a caller who says nothing gets a token that can still be withdrawn.
A download link asks for the other kind:

```shell
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"scopes":["image:read"],"stored":false,"ttl":"1h"}'
```

The `/ui/tokens` page mints stored tokens only: name the token, select one of the fixed actor profiles, set a lifetime, and copy the result.
The server expands the selected actor to its executable scopes and delegation ceiling; the page does not accept custom scopes.
The lifetime field takes the same Go duration as `ttl`; leaving it empty uses the server default.
The listing shows how long each token has left, with the exact expiry on hover.
An ephemeral token has no name and never appears in that list, so there would be nothing for the page to show afterwards; the API is where those are minted.
The result is shown with the identity it authenticates as, `org_id` or the username under htpasswd, because that is the registry username the token pairs with as the password.
A token carrying `image:read` is additionally offered as a `RegistryAuthConfig` machine config patch ready to paste into a Talos node.

`ttl` is optional and must fall within `authentication.tokens.ttl.stored` or `.ephemeral`, selected by the `stored` field.
Omitting it takes that policy's default.
Scopes do not shorten the lifetime.
A long-lived, revocable Omni credential may combine only the atomic capabilities its workflow needs.

`name` is required only for a stored token, because the name is what an operator picks it out of the list by.

The subject of every token is the caller identity — `org_id`, or the username under htpasswd — never a schematic and never a path.
Ownership is then enforced the same way it is for any other request, which means a token reaches every image owned by that identity for its lifetime, within its scopes.

### Minting for another identity

`POST /tokens` normally mints for the caller: the subject of the new token is the identity that authenticated the request.
A `subject` field overrides that, and only a CLI bootstrap credential carrying `any_subject` may send it.

```shell
curl -s -X POST -H "Authorization: Bearer $BOOTSTRAP_TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"name":"rack-3","scopes":["image:read"],"subject":"org_abc123"}'
```

The minted token belongs to `org_abc123` in every sense that matters: it authenticates as that identity, ownership checks resolve against it, the record lands in that organization's index, it counts against that organization's `maxPerOrg`, and the response reports it as `org_id`.

Anything other than the bootstrap credential is refused with `403` and `only a bootstrap credential may mint for another identity`.
That includes a full htpasswd or Auth0 credential, which is otherwise unrestricted in what it may mint.
The asymmetry is deliberate: a provider credential carries authority over its own identity, and naming another identity reaches across tenants instead.
Naming your own identity is not a cross-tenant mint, so it is allowed and does nothing.

`subject` must be a single-line value of at most 256 bytes.
It ends up in the JWT, in the token index and in every audit record the minted token produces, so whitespace and control characters are refused rather than escaped.

One asymmetry to plan around: minting is cross-tenant, listing and revocation are not.
`GET /tokens` and `POST /tokens/:id/revoke` still act on the caller's own identity, so a token bootstrapped into another organization is revoked by a credential belonging to that organization, not by the bootstrap credential that created it.

### The `?token=` query parameter

The query string is for callers that cannot set a header: a browser, an appliance, a link handed to someone else, an iPXE boot.

- Its value is an API token, and nothing else.
  An Auth0 JWT or an htpasswd password is not one and does not work here.
- Only an [ephemeral](#stored-and-ephemeral-tokens) token is read from the query string.
  A stored one is rejected there and must use the header: query strings are recorded by proxy and CDN access logs, which is survivable for the hours an ephemeral token can live and not for the year a stored one can.
- A token carrying any token-management capability (`token:issue`, `token:read` or `token:revoke`) is refused there whatever its lifetime.
  Such credentials do not belong in access logs.
  Other ephemeral scopes may travel this way; in practice `image:read` is the capability used for download and PXE URLs.
- It is read only on `GET` and `HEAD`, so it never authenticates a write.
- On `/pxe/` the same token is forwarded into the kernel, initramfs and UKI URLs of the generated script, so an iPXE boot needs no credential of its own.
  This happens whichever transport the token arrived on, since the URL is the only one iPXE has.
  Nothing is minted there: the script expires with the token it was fetched with, and re-fetching the script with an expiring token cannot extend that lifetime.
  A boot fetches its assets seconds after the script, so the ephemeral-token default of `5m` covers it; request a longer lifetime for a script that is kept and reused, up to `authentication.tokens.ttl.ephemeral.max`.
- It is checked before the header.
  A valid token authenticates the request on its own; a missing, expired or malformed one falls back to the header, so a request carrying only a bad token gets the ordinary `401` rather than a distinct error.

Treat such a URL as the credential it is.

### Listing and revocation

Stored tokens are recorded in a per-org index kept in the OCI repository at `authentication.tokens.storage`.
Presence in that index is what keeps such a token valid, so `POST /tokens/:id/revoke` takes it out of circulation by removing the record.

Revocation is immediate after the backing registry accepts the tombstone.
Each replica verifies a stored token by reading its deterministic tag, so it does not wait for an organization-wide listing cache to refresh.

`authentication.tokens.maxPerOrg` (`10` by default) caps how many unexpired recorded tokens an organization may hold at once; a create beyond it is `409`.
Expired records are omitted from the list and no longer count against the cap.

Ephemeral tokens do not appear in a listing, cannot be revoked and do not count against the cap.
Indexing them would mean a registry write per download link, so they use the deliberately short `authentication.tokens.ttl.ephemeral` policy.

### Signing key

`authentication.tokens.keyPaths` is an ordered list of PEM files.
The first entry must contain an ECDSA P-256 private key, in either SEC1 or PKCS#8 form, and is the only key used to mint new tokens.
Later entries are verification-only and may contain an ECDSA P-256 private key, PKIX public key or X.509 certificate.
The public half of every configured key is served, in the same order, at `/.well-known/jwks.json` so that a proxy in front of the factory can verify tokens without holding private keys.
Duplicate public keys are rejected at startup.
An X.509 certificate is treated as an explicitly trusted public-key container; its validity period and chain are not evaluated.

```yaml
authentication:
  tokens:
    keyPaths:
      - /etc/image-factory/token-active.key
      - /etc/image-factory/token-previous.crt
```

Leaving the list empty generates a key pair at startup, which works for a single replica only: a token minted by one replica fails verification on every other, so requests fail intermittently behind a load balancer.
Configure the key paths for any deployment running more than one replica.

Rotate keys in two deployments so every replica can verify both keys before any replica starts signing with the new one:

1. Append the new key or certificate after the current active key and roll out that verification set everywhere.
2. Move the new private key to the first position, keep the old key later in the list, and roll out again.
3. Remove the old verification key only after every token it signed has expired, or when intentionally invalidating those tokens.

Prepending a new key in a single rolling deployment is unsafe: updated replicas can mint tokens that replicas still running the previous configuration cannot verify.

Verification allows 30s of clock leeway.
