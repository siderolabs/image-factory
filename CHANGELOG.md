## [image-factory 0.1.2](https://github.com/siderolabs/image-factory/releases/tag/v0.1.2) (2023-11-08)

Welcome to the v0.1.2 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Noel Georgi
* Utku Ozdemir
* budimanjojo

### Changes
<details><summary>5 commits</summary>
<p>

* [`58378e0`](https://github.com/siderolabs/image-factory/commit/58378e0da6aa7a9cb30393e60186d022757f82b2) chore: bump dependencies and Talos
* [`db21b76`](https://github.com/siderolabs/image-factory/commit/db21b76ca8b34e9a9534342ba4516246fbb1bae4) fix: parse profiles for 'digital-ocean' platform
* [`43a6388`](https://github.com/siderolabs/image-factory/commit/43a638843689df7b04db8d8292a2d0983507d408) release(v0.1.1): prepare release
* [`4211a5c`](https://github.com/siderolabs/image-factory/commit/4211a5c52d6a5c407cecb793f47c17eb6a529051) chore: update Talos
* [`fcc8cb5`](https://github.com/siderolabs/image-factory/commit/fcc8cb503835de8b549c99281302f55acd4880c2) fix: small UI updates
</p>
</details>

### Changes since v0.1.1
<details><summary>2 commits</summary>
<p>

* [`58378e0`](https://github.com/siderolabs/image-factory/commit/58378e0da6aa7a9cb30393e60186d022757f82b2) chore: bump dependencies and Talos
* [`db21b76`](https://github.com/siderolabs/image-factory/commit/db21b76ca8b34e9a9534342ba4516246fbb1bae4) fix: parse profiles for 'digital-ocean' platform
</p>
</details>

### Changes from siderolabs/talos
<details><summary>13 commits</summary>
<p>

* [`75d3987c0`](https://github.com/siderolabs/talos/commit/75d3987c05390d3c0a7cf4de855895f1d10c8a84) chore: drop sha1 from genereated pcr json
* [`6f32d2990`](https://github.com/siderolabs/talos/commit/6f32d2990f438a9e8134d7e94558a54b3912854e) feat: add `.der` output `talosctl gen secureboot pcr`
* [`87c40da6c`](https://github.com/siderolabs/talos/commit/87c40da6cc5d9ae62d20984ba5d3762da734a49e) fix: proper logging in machined on startup
* [`a54da5f64`](https://github.com/siderolabs/talos/commit/a54da5f641886d723465e0a8cfa95b15bc2e96aa) fix: image build for nanopi_4s
* [`6f3cd0593`](https://github.com/siderolabs/talos/commit/6f3cd05935a2faaf14d16c2e643f54e6f9134c0f) refactor: update packet capture to use 'afpacket' interface
* [`813442dd7`](https://github.com/siderolabs/talos/commit/813442dd7a08b2781829ef190b110aa38c725932) fix: don't validate machine.install if installed
* [`dff60069c`](https://github.com/siderolabs/talos/commit/dff60069c0230ecf531c5593724211fd75f26d7c) feat: update Kubernetes to 1.29.0-alpha.3
* [`c97db5dfe`](https://github.com/siderolabs/talos/commit/c97db5dfe174032f012bdd525a3479ebea200c93) chore: bump Go dependencies
* [`807a9950a`](https://github.com/siderolabs/talos/commit/807a9950ac5cb542e41d65af0f9f80f1c73550a3) fix: use custom Talos/kernel version when generating UKI
* [`eb94468a6`](https://github.com/siderolabs/talos/commit/eb94468a659b4518b317398f92346b62e6adefe4) docs: add documentation for Image Factory
* [`2e78513e1`](https://github.com/siderolabs/talos/commit/2e78513e16b2eb0d83a4a7e107c470058d30837d) refactor: drop the dependency link platform -> network ctrl
* [`6dc776b8a`](https://github.com/siderolabs/talos/commit/6dc776b8aaa2d9382737d41a90023e8e4ea1a601) fix: when writing to META in the installer/imager, use fixed name
* [`3703041e9`](https://github.com/siderolabs/talos/commit/3703041e989c83c1ad7496851c6687f729cb207f) chore: remove uneeded code
</p>
</details>

### Dependency Changes

* **github.com/siderolabs/talos**                cbe6e7622d01 -> 75d3987c0539
* **github.com/siderolabs/talos/pkg/machinery**  cbe6e7622d01 -> 75d3987c0539
* **github.com/sigstore/cosign/v2**              v2.2.0 -> v2.2.1
* **golang.org/x/sync**                          v0.4.0 -> v0.5.0
* **golang.org/x/sys**                           v0.13.0 -> v0.14.0

Previous release can be found at [v0.1.0](https://github.com/siderolabs/image-factory/releases/tag/v0.1.0)

## [image-factory 0.1.1](https://github.com/siderolabs/image-factory/releases/tag/v0.1.1) (2023-11-02)

Welcome to the v0.1.1 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* budimanjojo

### Changes
<details><summary>2 commits</summary>
<p>

* [`4211a5c`](https://github.com/siderolabs/image-factory/commit/4211a5c52d6a5c407cecb793f47c17eb6a529051) chore: update Talos
* [`fcc8cb5`](https://github.com/siderolabs/image-factory/commit/fcc8cb503835de8b549c99281302f55acd4880c2) fix: small UI updates
</p>
</details>

### Changes from siderolabs/talos
<details><summary>3 commits</summary>
<p>

* [`2e78513e1`](https://github.com/siderolabs/talos/commit/2e78513e16b2eb0d83a4a7e107c470058d30837d) refactor: drop the dependency link platform -> network ctrl
* [`6dc776b8a`](https://github.com/siderolabs/talos/commit/6dc776b8aaa2d9382737d41a90023e8e4ea1a601) fix: when writing to META in the installer/imager, use fixed name
* [`3703041e9`](https://github.com/siderolabs/talos/commit/3703041e989c83c1ad7496851c6687f729cb207f) chore: remove uneeded code
</p>
</details>

### Dependency Changes

* **github.com/siderolabs/talos**                cbe6e7622d01 -> 2e78513e16b2
* **github.com/siderolabs/talos/pkg/machinery**  cbe6e7622d01 -> 2e78513e16b2

Previous release can be found at [v0.1.0](https://github.com/siderolabs/image-factory/releases/tag/v0.1.0)


## [image-factory 0.1.0](https://github.com/siderolabs/image-factory/releases/tag/v0.1.0) (2023-11-01)

Welcome to the v0.1.0 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Tim Jones
* Andrew Rynhard
* Noel Georgi

### Changes
<details><summary>29 commits</summary>
<p>

* [`1a4d836`](https://github.com/siderolabs/image-factory/commit/1a4d8364de3c94552bf96f3ce0d0e81427583d92) feat: implement metrics for Image Factory
* [`661dc70`](https://github.com/siderolabs/image-factory/commit/661dc70db3603816163560abfbf326cdca52dc04) fix: implement insecure option for cache repository
* [`3d99e0a`](https://github.com/siderolabs/image-factory/commit/3d99e0a6964b8616743c25488047891094d38ee4) fix: generation of SBC images
* [`354baca`](https://github.com/siderolabs/image-factory/commit/354baca8a0d22a85e75b9bdadb0f771be8221886) feat: implement boot asset cache
* [`3dcb29d`](https://github.com/siderolabs/image-factory/commit/3dcb29d8ee0d6b951f9e3614f50620df83ffeccd) feat: sign generated installer image
* [`c43564f`](https://github.com/siderolabs/image-factory/commit/c43564fc007862b14b7058031472806f9675625a) feat: use OCI layout when passing images to the imager
* [`6daded9`](https://github.com/siderolabs/image-factory/commit/6daded9cac70d7c6506a072484c20c9df44e70be) feat: support 'META' customization in schematics
* [`8286f4e`](https://github.com/siderolabs/image-factory/commit/8286f4e953337c6f3a93feba039cc1f75d8eeac9) fix: update Go to 1.21.3
* [`2efc7b9`](https://github.com/siderolabs/image-factory/commit/2efc7b951ab79629987128f5f69ec2293c4ecb40) chore: rekres
* [`6ae0d38`](https://github.com/siderolabs/image-factory/commit/6ae0d38aac4cb88b3fd2f92b932264a25d03b480) fix: check the already built installer image correctly
* [`10d78fa`](https://github.com/siderolabs/image-factory/commit/10d78fa04eea435a4e7c275cc2090462dc50537b) fix: allow pulling installer image from insecure registry
* [`f5e3ef7`](https://github.com/siderolabs/image-factory/commit/f5e3ef7ac0e8eb18a708d311c8c614d0dc8df097) feat: support insecure endpoint for internal repository
* [`9f5d43b`](https://github.com/siderolabs/image-factory/commit/9f5d43b93383e4bef875448851d22a3ca5e3a635) fix: asset links
* [`ad67f1e`](https://github.com/siderolabs/image-factory/commit/ad67f1e1729b1342f2f6814a6a1b510777feb41a) fix: template filenames after renames
* [`a0b6a8a`](https://github.com/siderolabs/image-factory/commit/a0b6a8a67db7a8d5a7ce6245fb6184a1912b994b) feat: add support for insecure image registry
* [`25100a6`](https://github.com/siderolabs/image-factory/commit/25100a658a6f9ef141e707d53b2f8b5673dc28a7) fix: various (small) fixes for registry operations
* [`f88dafa`](https://github.com/siderolabs/image-factory/commit/f88dafadf88e2519904616667365c2a2015e162e) chore: migrate to GitHub Actions
* [`92a4cfd`](https://github.com/siderolabs/image-factory/commit/92a4cfd2cdd564177528bb9fe3bc4b655d5e62d1) fix: import Talos with initramfs generation fixes
* [`91bbcd2`](https://github.com/siderolabs/image-factory/commit/91bbcd2c8217660a41078071c94ad742c58c3cfb) chore: rename with new nomenclature
* [`7bb02a8`](https://github.com/siderolabs/image-factory/commit/7bb02a8cc6c82c0c55d47452a6cc546fccdeb6b8) chore: add no-op github workflow
* [`2f92d92`](https://github.com/siderolabs/image-factory/commit/2f92d92c57c0c67e819a3a5f3e011af30168ce1b) feat: implement simple UI for the Image Service
* [`cf73db9`](https://github.com/siderolabs/image-factory/commit/cf73db9b91246deed4b879d971638498da9054b3) feat: implement support for system extensions
* [`b730f09`](https://github.com/siderolabs/image-factory/commit/b730f093a079ffd0fdd731a641d069de25e22ae9) feat: add a virtual extension with flavor ID to generated assets
* [`cf250cd`](https://github.com/siderolabs/image-factory/commit/cf250cd1039ee29433b3b0ee18e87b8a1fc455bc) chore: rename 'configuration' to 'flavor'
* [`47c6aea`](https://github.com/siderolabs/image-factory/commit/47c6aeabc448b71b8165ad55b4463022cd1276bd) feat: implement registry frontend
* [`f8fed5c`](https://github.com/siderolabs/image-factory/commit/f8fed5c6c72d591d2c9eb73281541de17a835a6f) feat: use OCI registry as a configuration storage
* [`a4aa38c`](https://github.com/siderolabs/image-factory/commit/a4aa38c9ecd07379752f2e07b876841840011259) feat: implement PXE frontend
* [`803ffa1`](https://github.com/siderolabs/image-factory/commit/803ffa15a5a1cf8b0b54b7418b403893ce758a74) feat: initial version
* [`d2c7fe4`](https://github.com/siderolabs/image-factory/commit/d2c7fe414ed33c8ba49cf5713955d8c74d4fb8f5) chore: initial commit
</p>
</details>

### Dependency Changes

This release has no dependency changes

