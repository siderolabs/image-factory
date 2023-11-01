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

