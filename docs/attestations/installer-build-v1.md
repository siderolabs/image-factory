<!--
This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
-->

# Enterprise Installer Build Evidence v1

Enterprise Image Factory publishes cryptographically signed build evidence for every newly generated Installer OCI image using Talos 1.13.0 or newer.
Talos 1.13 prereleases are outside this contract.
Evidence publication is mandatory for this supported range and has no runtime enablement, builder identity, build type, or referrer-mode configuration.

## Build type

The SLSA build type is:

```text
https://github.com/siderolabs/image-factory/blob/<version>/docs/attestations/installer-build-v1.md
```

`<version>` is the Image Factory release tag exposed as `version.Tag`.

The builder identity is:

```text
https://github.com/siderolabs/image-factory
```

These values identify this build contract and its producer.
Changing either value requires a new contract rather than a deployment configuration change.

## Evidence graph

A generated multi-platform Installer has the following graph:

```text
linux/amd64 manifest -> SPDX 2.3 JSON SBOM attestation
linux/arm64 manifest -> SPDX 2.3 JSON SBOM attestation
multi-platform index -> SLSA Provenance v1 attestation
multi-platform index -> Image Factory completion signature
```

Every OCI subject, in-toto subject, and requested digest is immutable and must match exactly.
The runnable Installer index contains only runnable platform manifests; evidence is not embedded as `unknown/unknown` platform entries.
The index and its platform manifests are separate attestation subjects: Cosign does not descend from the index to find platform SBOMs.

Each platform SBOM inventories the Talos base and selected official system extensions.
Schematics with the same Talos version, architecture, and official extension set share the same SPDX predicate even when non-package customization differs.
Overlay content, kernel arguments, machine configuration, and secure-boot packaging are outside the SBOM inventory; their specific platform manifest remains the attestation subject.

Attestations use:

- DSSE 1.0.2 envelopes;
- in-toto Statement v1;
- SPDX 2.3 JSON predicates;
- SLSA Provenance v1 predicates;
- Sigstore bundle v0.3 artifacts;
- OCI referrers, using the native OCI 1.1 API when available and the OCI referrers-tag schema otherwise.

## Provenance contract

The index-level SLSA provenance records:

- the immutable multi-platform index and platform manifests as subjects;
- the requested Installer variant, Talos version, schematic, secure-boot mode, and hardware platform;
- invocation identity and build start/finish timestamps;
- Image Factory version information;
- the schematic digest;
- exact architecture-specific base Installer, system extension, and overlay manifest digests consumed by the build.

Materials are deterministically ordered before serialization.
Mutable source tags are not provenance materials.
Overlay processing records both the target overlay and any builder-architecture overlay consumed as a build tool; exact duplicate materials are emitted once.

## Publication and cache behavior

Image Factory stages the index by digest, publishes and verifies all required evidence, publishes and verifies the completion signature, and only then promotes the version tag.
A failed SBOM, provenance, signature, registry, or verification operation leaves the final tag unpublished.

Cache hits are accepted when the cached index has a valid completion signature.
Evidence is verified during initial publication, before the version tag is promoted;
it is not re-fetched and re-verified on every subsequent manifest request.
Indexes signed by an earlier Image Factory version are accepted by this cache path and are not backfilled with evidence.

## OCI referrer compatibility

Image Factory automatically uses the native OCI 1.1 referrers API when the backing registry implements it.
For registries without that endpoint, including the reference `registry:2` and `registry:3` implementations, publication uses the OCI referrers-tag schema defined by the Distribution specification.
That fallback updates a shared tag with read-modify-write semantics; deployments with concurrent publishers should prefer a registry with native referrers support.

This selection is automatic and has no operator-facing mode.
Image Factory does not publish legacy Cosign `.att` tags.

Clients discover evidence through:

```text
GET /v2/<installer>/<schematic>/referrers/<subject-digest>
```

The endpoint resolves both native referrers and the referrers-tag schema and returns an OCI image index.
The standard `artifactType` query parameter filters the returned descriptors in either mode.

## Verification with Cosign

### Build the image reference

Cosign expects an OCI reference:

```text
<registry-host>[:<port>]/<repository>@sha256:<digest>
```

Download the static public key when Image Factory uses local-key signing:

```shell
curl -o signing-key.pub https://factory.example.com/oci/cosign/signing-key.pub
```

Set the repository and immutable index reference used by the following commands:

```shell
IMAGE=registry.example.com/metal-installer/<schematic>
INDEX_DIGEST=sha256:<index-digest>
INDEX_REF="$IMAGE@$INDEX_DIGEST"
```

### Verify index provenance

The multi-platform index has the SLSA Provenance attestation:

```shell
cosign verify-attestation \
  --key signing-key.pub \
  --type https://slsa.dev/provenance/v1 \
  --insecure-ignore-tlog \
  "$INDEX_REF"
```

The predicate URI is exactly `https://slsa.dev/provenance/v1`.

### Find the platform manifest digests

SPDX attestations are attached to the platform manifests, not to the multi-platform index.
Inspect the index before verifying an SBOM:

```shell
crane manifest "$INDEX_REF" | \
  jq -r '.manifests[] | [.platform.os, .platform.architecture, .digest] | @tsv'
```

Example output:

```text
linux  amd64  sha256:<amd64-manifest-digest>
linux  arm64  sha256:<arm64-manifest-digest>
```

Use the digest for the platform you want to verify:

```shell
PLATFORM_DIGEST=sha256:<platform-manifest-digest>
PLATFORM_REF="$IMAGE@$PLATFORM_DIGEST"
```

### Verify a platform SBOM

```shell
cosign verify-attestation \
  --key signing-key.pub \
  --type https://spdx.dev/Document/v2.3 \
  --insecure-ignore-tlog \
  "$PLATFORM_REF"
```

The expected subject and predicate combinations are:

| Subject | Predicate type |
| --- | --- |
| Multi-platform index digest | `https://slsa.dev/provenance/v1` |
| `linux/amd64` manifest digest | `https://spdx.dev/Document/v2.3` |
| `linux/arm64` manifest digest | `https://spdx.dev/Document/v2.3` |

If Cosign reports that it found SLSA Provenance while you requested SPDX, you passed the index digest instead of a platform manifest digest.

### Development registries

The registry transport flags are independent of signature and transparency-log verification:

- use `--allow-http-registry=true` for a plain HTTP registry;
- use `--allow-insecure-registry=true` for HTTPS with an invalid or self-signed certificate;
- use `--registry-cacert <path>` instead when the registry has a private CA that the host does not trust.

Pass the same transport option to `crane` when discovering platform digests; for a plain HTTP registry, use `crane manifest --insecure "$INDEX_REF"`.
A connection attempt to port 443 usually means the reference omitted the registry's explicit port or the client was not told to allow HTTP.

`--insecure-ignore-tlog` is required for static-key attestations because those bundles intentionally have no Rekor transparency-log material.
It does not enable HTTP or relax registry TLS validation.

For Google Service Account signing, omit `--key` and `--insecure-ignore-tlog`, and supply the expected certificate identity and OIDC issuer according to the deployment's signing identity policy.
Those bundles include Fulcio, Rekor, and timestamp verification material.

## Relationship to downloadable assets

Installer attestations describe OCI build evidence.
Detached `.sigstore.json` bundles for downloadable assets are a separate feature that authenticates the exact downloaded bytes.
Verifying one does not replace verifying the other.
