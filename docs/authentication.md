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

The one alternative to this header is the [`?token=` query parameter](#the-token-query-parameter), accepted on image downloads only.

## htpasswd

Basic authentication against an `htpasswd` file.
The caller identity is the username.

## auth0

Bearer token authentication against an Auth0 tenant.
Tokens are validated locally against the tenant's JWKS: signature, issuer, audience and expiry.

The caller identity is the token's `org_id` claim, so tokens must be issued to organization-scoped clients.
Tokens without the claim are rejected, since there would be no principal to attribute the request to.

Tokens are accepted either as `Authorization: Bearer <token>` or in the password field of a Basic credential; see [The `Authorization` header](#the-authorization-header).

There is no interactive login.
A browser hitting an authenticated route gets a `401` with a Basic challenge, not a redirect to Auth0, so a person presents a token the same way a machine does.

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

Tokens carrying it may only fetch artifacts: `GET` and `HEAD` on `/image/` and on the `/v2/` OCI registry.
Everything else is rejected with `403`, including reading a schematic definition, so a stolen node credential cannot enumerate how the organization's images are built.

Both the `scope` and `permissions` claims are consulted, since Auth0 uses the former for plain client-credentials grants and the latter when RBAC is enabled on the API.
Tokens without the scope are unaffected, and leaving the setting empty gives every valid token full access.

The factory does not refresh these tokens: whatever is provisioned onto the node is used until it expires, and the node then needs a new one from somewhere.
Because validation is offline (see [Revocation](#revocation)), the token's Auth0 lifetime is the entire exposure window for a credential lifted off a node, so choose it deliberately rather than taking the tenant default.

A machine-scoped token also cannot mint a [download token](#download-tokens), since `POST /download-token` is not an artifact fetch.
A deployment that both provisions nodes and hands out download links needs two clients: one carrying the machine scope, one without it.

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

The same applies to download tokens minted from a JWT.
They stay valid for their own TTL regardless of what happens to the JWT they were obtained with.

## Ownership

Creating a schematic with authentication enabled stamps the caller identity into its `owner` field.
`owner` is part of the canonical schematic body, and the schematic ID is the hash of that body, so two identities submitting the same customization get two different IDs.

Reading a schematic, or any artifact derived from one, requires the `owner` to match the caller:

- A mismatch is `403`.
- An unauthenticated caller is `401`, whether or not the schematic exists, so its existence is not leaked.
- A schematic with no `owner` is `403` for every caller once authentication is enabled, since no identity matches an empty owner.

That last case covers schematics created before authentication was turned on, and the well-known public IDs such as the default schematic.
Neither is reachable in an authenticated deployment; each identity creates its own.

## Download tokens

A download token is a short-lived JWT that authenticates an image download through the URL alone, for callers that cannot set an `Authorization` header: a browser, an appliance, a link handed to someone else.

Mint one with `POST /download-token`, authenticated like any other route:

```shell
curl -s -X POST -H "Authorization: Bearer $TOKEN" https://factory.example.com/download-token
{"access_token":"eyJ...","token_type":"Bearer","expires_in":300}
```

Then append it to an image URL:

```shell
curl -LO "https://factory.example.com/image/<schematic>/v1.13.0/metal-amd64.iso?token=eyJ..."
```

Tokens are signed with ECDSA P-256, and the public key is served unauthenticated at `/.well-known/jwks.json` so that a proxy in front of the factory can verify one without holding the private key.
`authentication.downloadTokenTTL` sets the lifetime, default `5m`; verification allows a further 30s of clock leeway.

### The `?token=` query parameter

- Its value is the `access_token` from `POST /download-token`, and nothing else.
- It is accepted only on `GET` and `HEAD` under `/image/`.
  It does not work on `/pxe/`, on the `/v2/` registry, on schematic routes, or on the SBOM, VEX and scan routes: elsewhere the parameter is ignored and the [`Authorization` header](#the-authorization-header) is the only credential.
- It is not interchangeable with that header.
  A download token is signed with the factory's own key rather than issued by the provider, so it is rejected as `Authorization: Bearer`; conversely an Auth0 JWT or an htpasswd password is not a download token and does not work as `?token=`.
- It is checked before the header.
  A valid token authenticates the request on its own; a missing, expired or malformed one falls back to the header, so a request carrying only a bad token gets the ordinary `401` rather than a distinct error.

### Scope

The subject of a download token is the caller identity — `org_id`, or the username under htpasswd — not a schematic and not a path.
Ownership is then enforced the same way it is for any other request, which means one token grants read access to every image owned by that identity for its lifetime, not only the URL it was pasted into.

Treat the URL as the credential it is.
It carries the token in the query string, where proxy and CDN access logs tend to record it.

### Signing key

`authentication.downloadTokenKeyPath` points at a PEM-encoded ECDSA P-256 private key, in either SEC1 or PKCS#8 form.

Leaving it empty generates a key pair at startup, which works for a single replica only: a token minted by one replica fails verification on every other, so downloads fail intermittently behind a load balancer.
Configure the key path for any deployment running more than one replica.
