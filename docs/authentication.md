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
Until it is configured, a browser hitting an authenticated route gets a `401` with a Basic challenge, not a redirect to Auth0, so a person presents a token the same way a machine does.

### Obtaining a token

`org_id` is only present when the token is issued for a specific organization, which requires Auth0 Organizations on the tenant and the client authorized for that organization.
For a client-credentials grant that means passing `organization` alongside `audience`:

```shell
curl -s -X POST https://<tenant>.auth0.com/oauth/token \
  -d grant_type=client_credentials \
  -d client_id=<id> -d client_secret=<secret> \
  -d audience=<authentication.auth0.audience> \
  -d organization=org_xxx
```

A token issued without `organization` carries no `org_id` and is rejected, even though it is otherwise valid.
A client serving several organizations requests one token per organization.

### Machine credentials

Talos nodes need a long-lived credential to pull installers, but a credential sitting on a node is more exposed than one held by a person.
Setting `authentication.auth0.machineScope` names a scope that marks a token as such a credential.

A token carrying it gets exactly the [`pull` scope](#scopes): `GET` and `HEAD` on `/image/` and on the `/v2/` OCI registry, and nothing else.
Everything else is rejected with `403`, including reading a schematic definition, so a stolen node credential cannot enumerate how the organization's images are built.
This is the same table a self-issued pull token is checked against, so the two cannot drift apart.

Both the `scope` and `permissions` claims are consulted, since Auth0 uses the former for plain client-credentials grants and the latter when RBAC is enabled on the API.
Tokens without the scope are unaffected, and leaving the setting empty gives every valid token full access.

The factory does not refresh these tokens: whatever is provisioned onto the node is used until it expires, and the node then needs a new one from somewhere.
Because validation is offline (see [Revocation](#revocation)), the token's Auth0 lifetime is the entire exposure window for a credential lifted off a node, so choose it deliberately rather than taking the tenant default.

A [self-issued pull token](#api-tokens) is the alternative: the factory signs and tracks it, so it can be listed and revoked, which an Auth0 token cannot.

A machine-scoped token also cannot mint an [API token](#api-tokens), since `POST /tokens` is not an artifact fetch.
A deployment that both provisions nodes and hands out download links needs two clients: one carrying the machine scope, one without it.

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
A [stored](#stored-and-unstored-tokens) token can at least be taken back explicitly; an unstored one cannot, which is what caps its lifetime at `authentication.tokens.ttl.unstoredMax`.

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

A API token is a JWT the factory issues and verifies with its own key, rather than one obtained from the configured provider.
What it may do is decided by the **scopes** it carries, not by the endpoint that minted it.

There is one token type, one signing key and one JWKS.
A short-lived download link and a year-long node credential are the same credential with different scopes, lifetimes and storage.

### Scopes

| Scope       | Authenticates                                                             |
| ----------- | ------------------------------------------------------------------------- |
| `download`  | `GET`/`HEAD` under `/image/`, and `GET` under `/pxe/`                     |
| `pull`      | `GET`/`HEAD` under `/image/`, and `GET`/`HEAD` on the `/v2/` OCI registry |
| `schematic` | `POST /schematics`, and `GET`/`HEAD` under `/schematics/`                 |
| `token`     | the `/tokens` routes                                                      |
| `admin`     | the `/tokens` routes                                                      |

One table in the factory defines these route sets, and the Auth0 [machine scope](#machine-credentials) is checked against the `pull` row of it, so a machine credential and a self-issued pull token reach exactly the same surface.

`pull` deliberately excludes `/pxe/`.
A PXE script exposes the kernel command line a schematic produces, and so the extensions and kernel arguments it was built from; a credential sitting on a node should not be able to read that.
It also excludes `/schematics/`, for the same reason: `schematic` is the scope for a build pipeline, not for a node.

`schematic` covers reading as well as creating, since an endpoint that mints an ID and then refuses to return what is behind it is not usable.
Reads are ownership-enforced like any other request, so the scope never reaches another organization's schematics.

A token may carry more than one scope, in which case it reaches the union of their routes and takes the tightest lifetime bound of any of them.

Which transports a token may arrive on, and whether it can be listed and revoked, are not scope properties: they follow from whether the factory records it.
See [Stored and unstored tokens](#stored-and-unstored-tokens).

### Stored and unstored tokens

Every token says, in the JWT itself, whether the factory keeps a record of it.
That is the `stored` claim, set once when the token is minted and never re-derived afterwards.

A **stored** token is written to the per-org index.
Presence in that index is what keeps it valid, which is what makes it revocable, and it is what `GET /tokens` lists.
It needs a `name`, because the name is what an operator picks it out of that list by, and it counts against `authentication.tokens.maxPerOrg`.

An **unstored** token is a signed string and nothing else.
Nothing records it, so it cannot be listed or revoked; expiry is the only way it leaves circulation.
It needs no name, costs no registry write, and it is the only kind read from a [`?token=` query parameter](#the-token-query-parameter).

Because the only way to withdraw an unstored token is to wait, lifetime and storage are tied together at issue time:

| Requested lifetime                                   | May be issued            |
| ---------------------------------------------------- | ------------------------ |
| below `authentication.tokens.ttl.storedMin` (`1h`)   | unstored only            |
| above `authentication.tokens.ttl.unstoredMax` (`8h`) | stored only              |
| in between                                           | either, the caller picks |

So a five-minute download link cannot be recorded, a year-long node credential cannot escape revocation, and neither rule depends on which scopes the token carries.

On the verification path the claim decides the work: an unstored token is accepted on its signature and expiry alone, and a stored one is additionally looked up in the index, so a revoked token stops working without every download paying for a registry read.

### The `token` and `admin` scopes

`token` is the one scope that hands out authority, so it is attenuating.
A caller authenticated **by an API token** may mint a token only within these two rules:

- It may not grant a scope it does not hold itself.
  A token carrying `token` and `pull` can mint pull tokens, and nothing else.
- It may never grant `token`, even though it holds it.

The second rule is what bounds the damage.
Without it a leaked minting token could mint a fresh minting token before anyone noticed, and revoking the original would not reach the successor; with it, every token a minting token produces is a leaf, and revoking the minting token ends the chain.

Neither rule applies to a caller holding a full provider credential — an htpasswd user or an Auth0 client — which carries no scopes and can mint anything.
That is the credential a person uses in the UI.

A `403` with `the authenticating token may not grant these scopes` is one of these rules firing.

`admin` is the exception to the first rule and the bootstrap credential for the second.
It reaches the same routes as `token`, and may hand out `token`, which nothing else can, along with any other scope it does not itself hold.
It may not hand out `admin`, not even to a caller already holding it, so a leaked admin token cannot mint a successor that outlives it.

It is not mintable over HTTP at all.
`POST /tokens` rejects the scope for every caller, a full htpasswd or Auth0 credential included, with a `400`.
The only thing that issues one is the [`admin-token` subcommand](#minting-an-admin-token).

An admin token is also the only credential that may mint for an identity other than its own; see [Minting for another identity](#minting-for-another-identity).

An admin token is never recorded, which means nothing can revoke it.
That is deliberate, and it is the reason for the other limits on it: it cannot mint another admin token, it is refused in a `?token=` query parameter, and its lifetime is the only bound on a leak.
Rotating `authentication.tokens.keyPath` retires one early, at the cost of invalidating every other self-issued token too.
Keep it offline, and give it the shortest lifetime the deployment can work with.

### Minting an admin token

```shell
image-factory admin-token --config /etc/image-factory/config.yaml --subject org_abc123
eyJ...
```

The command exists in Enterprise builds only, since it is API token management that a community build registers no routes for.
`image-factory --help` lists the commands a build has, and a community build refuses this one with `unknown command "admin-token"`.

The token goes to stdout and everything a human should read goes to stderr, so `> token` leaves a usable file.

`--subject` is required and is the identity the token authenticates as, an `org_id` under Auth0 or a username under htpasswd.
Every token minted with it belongs to that identity, so it decides which organization the credential can act for.

`--ttl` requests a lifetime within `authentication.tokens.ttl.admin`, which defaults to 90 days and allows up to 10 years.
`authentication.tokens.unstoredMax` does not apply: that cap is for a caller who declined a record the factory would have kept, and `admin` never had that choice.

`authentication.tokens.keyPath` has to be set.
Without it the factory generates a signing key at startup, so the subcommand would print a token signed with a key no running replica holds.

### Minting one

`POST /tokens`, authenticated like any other route:

```shell
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"name":"rack-3","scopes":["pull"],"ttl":"8760h"}'
{"id":"0d1c...","name":"rack-3","scopes":["pull"],"token":"eyJ...","org_id":"org_abc123","created_at":"...","expires_at":"..."}
```

`token` is shown exactly once.
The factory does not store it and cannot show it again; only the record naming it is kept.
`stored` in the response echoes what was actually recorded.

`stored` defaults to `true`, so a caller who says nothing gets a token that can still be withdrawn.
A download link asks for the other kind:

```shell
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"scopes":["download"],"stored":false,"ttl":"1h"}'
```

The `/ui/tokens` page mints stored tokens only: name the token, tick the permissions it should carry, optionally set a lifetime, and copy the result.
The lifetime field takes the same Go duration the `ttl` field does, and leaving it empty takes the server default.
The listing shows how long each token has left, with the exact expiry on hover.
An unstored token has no name and never appears in that list, so there would be nothing for the page to show afterwards; the API is where those are minted.
The result is shown with the identity it authenticates as, `org_id` or the username under htpasswd, because that is the registry username the token pairs with as the password.
A token carrying `pull` is additionally offered as a `RegistryAuthConfig` machine config patch ready to paste into a Talos node.

`ttl` is optional and must fall within the bounds configured for the requested scopes, and within the [storage bounds](#stored-and-unstored-tokens).
Omitting it takes `authentication.tokens.ttl.<scope>.default`, pulled into whichever window `stored` leaves.
An unstored `pull` token therefore defaults to `unstoredMax` rather than a year, and a stored `download` token to `storedMin` rather than five minutes.

`name` is required only for a stored token, because the name is what an operator picks it out of the list by.

The subject of every token is the caller identity — `org_id`, or the username under htpasswd — never a schematic and never a path.
Ownership is then enforced the same way it is for any other request, which means a token reaches every image owned by that identity for its lifetime, within its scopes.

### Minting for another identity

`POST /tokens` normally mints for the caller: the subject of the new token is the identity that authenticated the request, and no field says otherwise.
A `subject` field overrides that, and only an admin token may send it.

```shell
curl -s -X POST -H "Authorization: Bearer $ADMIN_TOKEN" https://factory.example.com/tokens \
  -H 'Content-Type: application/json' \
  -d '{"name":"rack-3","scopes":["pull"],"subject":"org_abc123"}'
```

The minted token belongs to `org_abc123` in every sense that matters: it authenticates as that identity, ownership checks resolve against it, the record lands in that organization's index, it counts against that organization's `maxPerOrg`, and the response reports it as `org_id`.

Anything other than an admin token is refused with `403` and `only an admin token may mint for another identity`.
That includes a full htpasswd or Auth0 credential, which is otherwise unrestricted in what it may mint.
The asymmetry is deliberate: a provider credential carries authority over its own identity, and naming another identity reaches across tenants instead.
Naming your own identity is not a cross-tenant mint, so it is allowed and does nothing.

`subject` must be a single-line value of at most 256 bytes.
It ends up in the JWT, in the token index and in every audit record the minted token produces, so whitespace and control characters are refused rather than escaped.

One asymmetry to plan around: minting is cross-tenant, listing and revocation are not.
`GET /tokens` and `POST /tokens/:id/revoke` still act on the caller's own identity, so a token an admin minted into another organization is revoked by a credential belonging to that organization, not by the admin token that created it.

### The `?token=` query parameter

The query string is for callers that cannot set a header: a browser, an appliance, a link handed to someone else, an iPXE boot.

- Its value is an API token, and nothing else.
  An Auth0 JWT or an htpasswd password is not one and does not work here.
- Only an [unstored](#stored-and-unstored-tokens) token is read from the query string.
  A stored one is rejected there and must use the header: query strings are recorded by proxy and CDN access logs, which is survivable for the hours an unstored token can live and not for the year a stored one can.
- A token carrying `token` or `admin` is refused there whatever its lifetime.
  A minting credential in an access log is a minting credential leaked, and no expiry short enough to fix that is long enough to be useful.
  Every other scope may travel this way when the token is unstored, which in practice means `download`, since that is the scope a URL needs.
- It is read only on `GET` and `HEAD`, so it never authenticates a write.
- On `/pxe/` the same token is forwarded into the kernel, initramfs and UKI URLs of the generated script, so an iPXE boot needs no credential of its own.
  This happens whichever transport the token arrived on, since the URL is the only one iPXE has.
  Nothing is minted there: the script expires with the token it was fetched with, and re-fetching the script with an expiring token cannot extend that lifetime.
  A boot fetches its assets seconds after the script, so the `download` default of `5m` covers it; request a longer lifetime for a script that is kept and reused, up to `authentication.tokens.ttl.unstoredMax`.
- It is checked before the header.
  A valid token authenticates the request on its own; a missing, expired or malformed one falls back to the header, so a request carrying only a bad token gets the ordinary `401` rather than a distinct error.

Treat such a URL as the credential it is.

### Listing and revocation

Stored tokens are recorded in a per-org index kept in the OCI repository at `authentication.tokens.storage`.
Presence in that index is what keeps such a token valid, so `POST /tokens/:id/revoke` takes it out of circulation by removing the record.

Revocation is not instant.
Each replica reads the index through a cache, so a revoked token keeps working for up to `authentication.tokens.verificationCacheRefreshInterval` (`5m` by default) on replicas that have not refreshed yet.

`authentication.tokens.maxPerOrg` (`10` by default) caps how many recorded tokens an organization may hold at once; a create beyond it is `409`.

Unstored tokens do not appear in a listing, cannot be revoked and do not count against the cap.
Indexing them would mean a registry write per download link, to take back a credential that expires in hours anyway.
That is why the factory refuses to record anything shorter than `authentication.tokens.ttl.storedMin`.

### Signing key

`authentication.tokens.keyPath` points at a PEM-encoded ECDSA P-256 private key, in either SEC1 or PKCS#8 form.
One key signs every scope, and its public half is served unauthenticated at `/.well-known/jwks.json` so that a proxy in front of the factory can verify a token without holding the private key.

Leaving it empty generates a key pair at startup, which works for a single replica only: a token minted by one replica fails verification on every other, so requests fail intermittently behind a load balancer.
Configure the key path for any deployment running more than one replica.

Verification allows 30s of clock leeway.

### Upgrading from separate download and node tokens

Before this, the factory issued two token kinds with separate audiences, separate keys and separate configuration, and later one scoped token whose scopes decided everything about it.
They are now one token that says what it is, which changes this much for an existing deployment:

- **Tokens issued by an earlier version stop working.** They carry either the old audience or no `stored` claim, so the factory does not accept them.
Re-issue them from `/ui/tokens`; the machine config patch the page produces is unchanged.
- **The `/node-tokens` and `/download-token` routes are gone.** Use `POST /tokens` with `"scopes": ["pull"]` for the first and `{"scopes":["download"],"stored":false}` for the second.
Only the factory's own UI and Go client called them, and both moved.
- **The configuration moved.** `authentication.downloadTokenKeyPath`, `authentication.downloadTokenTTL` and `enterprise.nodeTokens` are gone, replaced by `authentication.tokens`.
The old keys are rejected at startup rather than ignored, so a stale config fails loudly instead of quietly generating a throwaway signing key.
- **One key signs every scope.** If the two key paths were set to different files, pick one and point `authentication.tokens.keyPath` at it.
- **`authentication.tokens.ttl.storedMin` and `.unstoredMax` are new**, and decide which lifetimes may be recorded; see [Stored and unstored tokens](#stored-and-unstored-tokens).
- **`POST /tokens` takes `stored`**, and its response reports `stored` where it used to report `revocable`.

The token index also moved, to `authentication.tokens.storage` (`ghcr.io/siderolabs/image-factory/tokens` by default).
