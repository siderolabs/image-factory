# Authentication

The factory authenticates callers with one of two providers, selected by `authentication.provider`.
See [Configuration](configuration.md) for the full list of settings.

Only one provider is active at a time.
There is no mode that accepts both htpasswd credentials and Auth0 tokens.

## htpasswd

Basic authentication against an `htpasswd` file.
The caller identity is the username.

## auth0

Bearer token authentication against an Auth0 tenant.
Tokens are validated locally against the tenant's JWKS: signature, issuer, audience and expiry.

The caller identity is the token's `org_id` claim, so tokens must be issued to organization-scoped clients.
Tokens without the claim are rejected, since there would be no principal to attribute the request to.

Tokens are accepted either as `Authorization: Bearer <token>` or in the password field of a Basic credential, because OCI and Talos registry clients only speak Basic auth.

### Machine credentials

Talos nodes need a long-lived credential to pull installers, but a credential sitting on a node is more exposed than one held by a person.
Setting `authentication.auth0.machineScope` names a scope that marks a token as such a credential.

Tokens carrying it may only fetch artifacts: `GET` and `HEAD` on `/image/` and on the `/v2/` OCI registry.
Everything else is rejected with `403`, including reading a schematic definition, so a stolen node credential cannot enumerate how the organization's images are built.

Both the `scope` and `permissions` claims are consulted, since Auth0 uses the former for plain client-credentials grants and the latter when RBAC is enabled on the API.
Tokens without the scope are unaffected, and leaving the setting empty gives every valid token full access.

### Migrating from htpasswd

Switching an existing deployment from `htpasswd` to `auth0` changes the identity namespace from usernames to `org_...` identifiers.
Consequences:

- Schematics owned by a username are not reachable by any Auth0 organization, and vice versa.
  Existing ownership does not carry over.
- Audit history written under the old provider records usernames, so records from before and after the switch cannot be correlated by principal.
- Because only one provider is active at a time, there is no gradual rollout.
  The switch takes effect for every caller at restart.

Plan the cutover accordingly, or start a fresh deployment.

### Revocation

Access tokens are validated offline, so revoking a client at the tenant does not invalidate tokens it has already been issued.
Those remain usable until they expire; keep token lifetimes short if that matters.

The same applies to download tokens minted from a JWT.
They stay valid for their own TTL regardless of what happens to the JWT they were obtained with.
