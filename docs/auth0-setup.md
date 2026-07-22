# Auth0 Integration Guide

This document covers the Auth0 integration for image-factory enterprise: the
provisioning model, token flows, image-factory configuration, and what Omni needs
to implement.

---

## Architecture Overview

Image-factory uses Auth0 **Organizations** as the unit of tenancy.
Every authenticated request — browser, Omni M2M, or Talos node — must carry an access token scoped to an organization.
The `org_id` claim is extracted and used as the resource owner for all schematics and images.

Three token acquisition paths exist:

| Client | Flow | Token source | Scopes |
| ------ | ---- | ----------- | ------ |
| Browser (human user) | Authorization Code + PKCE | Auth0 Universal Login | Image Factory API |
| Omni (automation) | Client Credentials | Auth0 `/oauth/token` | Image Factory API + Auth0 Management API |
| Talos node (updates) | Client Credentials | Auth0 `/oauth/token` | Image Factory API only |

### Provisioning model

```text
Sidero Labs controller
  └─ creates Auth0 org for the customer
  └─ creates Omni M2M app (Image Factory API + Management API)
       └─ hands credentials to Omni

Omni (using its M2M token)
  └─ creates node M2M app for the org (Image Factory API only)
  └─ fetches org-scoped node token
  └─ injects token as registry credential into Talos machine config

User (via Omni UI)
  └─ can trigger node credential rotation
       └─ Omni rotates the node M2M app credentials in Auth0
       └─ Omni fetches a new token and pushes it to machine configs
```

The key principle: **Omni's token has broad scopes** (Management API access to create and rotate M2M apps); **the node token has the narrowest possible scopes** (Image Factory API only).
If a node credential is compromised, the blast radius is limited to pulling images for that org.

### SAML users and direct image factory access

Omni supports SAML authentication for users whose organizations use an external identity provider (Okta, Azure AD, etc.).
Image-factory uses a **separate** Auth0 tenant and does not share Omni's SAML configuration.

SAML users who want direct browser access to image-factory need a separate Auth0 account:

1. Omni adds the user's email to the image-factory Auth0 org when provisioning
   (the email comes from the SAML assertion Omni already has)
2. The user visits the image-factory login page and uses "Forgot password" to set a
   password for their Auth0 account — no explicit sign-up step needed if the account
   already exists as an org member
3. From then on, they can log into image-factory directly with their Auth0 credentials

**Direct image-factory access is optional for SAML users.**
Users who only interact with Talos through the Omni UI never need an Auth0 account — Omni handles all image-factory requests via its M2M token on their behalf.
Only users who want to build custom images or inspect schematics directly need to set up the Auth0 account.

---

## Auth0 Dashboard Setup

> **Note:** The steps below describe what the Sidero Labs controller and Omni automate.
> They are documented here for reference, troubleshooting, and understanding what the
> automation does — not as a manual process for each customer.

### 1. Create a Custom API (once, per environment)

Auth0's built-in Management API must **not** be used as the Image Factory audience.

1. **APIs → Create API**
2. Name: `Image Factory`
3. Identifier (audience): `https://image-factory.example.com`
   — this is the value you set in `authentication.auth0.audience` in config.yaml
4. Signing algorithm: RS256
5. Save

### 2. Create the Browser Login Application (once, per environment)

This application handles human users logging in via browser.

1. **Applications → Create Application**
2. Type: **Regular Web Application**
3. Name: `Image Factory Browser`
4. Settings tab:
   - **Allowed Callback URLs**: `https://factory.example.com/callback`
   - **Allowed Logout URLs**: `https://factory.example.com`
   - **Allowed Web Origins**: `https://factory.example.com`
5. **Organizations tab** (appears after saving):
   - Business Type: **Business Users**
   - Login Flow: **Prompt for Credentials**
6. Note the **Client ID** and **Client Secret** — these go into image-factory config

### 3. Create the Omni M2M Application (once, per environment)

The Sidero Labs controller creates this application.
It needs access to both the Image Factory API and the Auth0 Management API so Omni can manage per-org node apps.

1. **Applications → Create Application**
2. Type: **Machine to Machine Application**
3. Name: `Omni`
4. Authorize it for:
   - The **Image Factory** API (step 1)
   - The **Auth0 Management API** with scopes: `create:clients`, `update:clients`,
     `read:client_keys`, `create:client_grants`, `read:organizations`,
     `create:organization_member_roles`
5. **Organizations tab**: Business Type: **Business Users**
6. Note the **Client ID** and **Client Secret** — the controller hands these to Omni

### 4. Per-Customer: Create an Auth0 Organization (automated by controller)

Each customer gets one Auth0 Organization.

1. **Organizations → Create Organization**
2. Name: URL-safe slug, e.g. `acme-corp`
3. **Connections tab**: enable the identity provider(s) users log in with
4. **Members tab**: add the human users for this org
5. **Machine to Machine Access tab**: authorize the Omni M2M application for this org
   — this allows Omni to request tokens scoped to this specific org
6. Note the **org_id** (e.g. `org_DLjYJtUTeq28LMAp`) — this is the stable ownership key

### 5. Per-Customer: Create the Node M2M Application (automated by Omni)

Omni creates this application using its Management API token.
One app per org.

1. Create a Machine to Machine application scoped **only** to the Image Factory API
2. Authorize it for the org (so tokens carry `org_id`)
3. Note the **Client ID** and **Client Secret** — used to fetch node tokens

---

## Image-Factory Configuration

Minimum `config.yaml` for Auth0:

```yaml
authentication:
  enabled: true
  provider: auth0
  auth0:
    domain: your-tenant.us.auth0.com
    audience: https://image-factory.example.com

    # Browser login (omit all four to run M2M-only):
    clientID: <browser-app-client-id>
    # clientSecret injected via IF_AUTHENTICATION_AUTH0_CLIENTSECRET env var
    redirectURL: https://factory.example.com/callback
    # sessionKey injected via IF_AUTHENTICATION_AUTH0_SESSIONKEY env var
```

Environment variables to inject at runtime:

```bash
# 32-byte AES-256 key for session cookie encryption, base64-encoded
IF_AUTHENTICATION_AUTH0_SESSIONKEY=$(openssl rand -base64 32)

# Client secret for the browser login application
IF_AUTHENTICATION_AUTH0_CLIENTSECRET=<secret-from-auth0-dashboard>
```

---

## Token Flows

### Omni M2M token request

Omni fetches a token scoped to the customer's org.
The `organization` parameter is required — without it, Auth0 will not include `org_id` in the JWT and image-factory will reject the token.

```http
POST https://<domain>/oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=<omni-m2m-client-id>
&client_secret=<omni-m2m-client-secret>
&audience=https://image-factory.example.com
&organization=<auth0-org-id>
```

### Node M2M token request

Same flow, using the node M2M app's credentials:

```http
POST https://<domain>/oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=<node-m2m-client-id>
&client_secret=<node-m2m-client-secret>
&audience=https://image-factory.example.com
&organization=<auth0-org-id>
```

### Using a token

As a Bearer header (Omni, most API clients):

```http
Authorization: Bearer eyJ...
```

As the password field in Basic Auth (Talos OCI client, which only supports Basic):

```http
Authorization: Basic <base64(any-username:eyJ...)>
```

Image-factory accepts the JWT in either position.

### Token lifecycle

- Default expiry is 24 hours (configurable per API in Auth0 dashboard, max 30 days)
- Omni should cache the token and refresh it before expiry (e.g. at 80% of lifetime)
- A new `client_credentials` request can be made at any time; there is no refresh token
  in the M2M flow

---

## What Omni Needs to Build

### Omni M2M flow

1. **Auth0 client credentials fetcher**
   - Store `client_id`, `client_secret`, `domain`, `audience`, and `org_id` per org
   - POST to `https://<domain>/oauth/token` with the `organization` parameter
   - Cache `access_token` with its `expires_in`; proactively refresh at ~80% of lifetime
   - On 401 from image-factory, immediately re-fetch and retry once

2. **Schematic creation**
   - `POST /schematics` with the Bearer token
   - Image-factory tags the resulting schematic as owned by the org (`org_id`)
   - Store the returned schematic ID for use in Talos machine config

3. **Image requests**
   - All `/image/`, `/pxe/`, and `/v2/` (OCI registry) routes require auth
   - Pass the Bearer token on every request

### Node M2M app management

1. **Node M2M app creation** (using Omni's Management API token)
   - `POST /api/v2/clients` to create a new M2M application scoped to the Image Factory API and associated with the customer's org
   - Store the returned `client_id` and `client_secret`

2. **Node token injection**
   - Fetch an org-scoped token using the node app credentials
   - Inject it as a registry credential into the Talos machine config:
     - Registry: `https://factory.example.com`
     - Username: any value (ignored by image-factory)
     - Password: the JWT access token

3. **Node credential rotation** (triggered by user from Omni UI)
   - `PATCH /api/v2/clients/<client-id>` to rotate the client secret (or create a new app)
   - Fetch a new token with the new credentials
   - Push the updated registry credential to all affected machine configs
   - Old tokens expire naturally (or can be revoked via `DELETE /api/v2/device-credentials`)

### Org ID vs Org Name

Image-factory uses `org_id` (e.g. `org_DLjYJtUTeq28LMAp`) as the ownership key.
This is the stable, opaque identifier Auth0 assigns at creation time.
Always store and use `org_id`, not the human-readable `org_name` — names can be changed.

---

## Application Summary

| Application | Type | Created by | Scopes | Used by |
| ---------- | ---- | --------- | ------ | ------- |
| Image Factory Browser | Regular Web App | Sidero Labs (once) | Image Factory API | Human users via browser |
| Omni | Machine to Machine | Sidero Labs controller (per env) | Image Factory API + Management API | Omni automated requests |
| Node (per org) | Machine to Machine | Omni (per org) | Image Factory API only | Talos node image pulls |

Each Auth0 organization must have:

- At least one connection enabled (so users can log in)
- Members added (for browser users)
- The Omni M2M application authorized under Machine to Machine Access
- The node M2M application authorized under Machine to Machine Access

---

## Verifying a Token Locally

```bash
# Fetch a token
TOKEN=$(curl -s -X POST https://<domain>/oauth/token \
  -d grant_type=client_credentials \
  -d client_id=<id> \
  -d client_secret=<secret> \
  -d audience=<audience> \
  -d organization=<org_id> | jq -r .access_token)

# Decode the payload (no signature verification)
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```

Confirm `org_id` is present in the output.
