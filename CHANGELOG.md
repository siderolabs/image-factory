## [image-factory 0.2.2](https://github.com/siderolabs/image-factory/releases/tag/v0.2.2) (2024-01-23)

Welcome to the v0.2.2 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Utku Ozdemir
* Anthony ARNAUD
* Artem Chernyshev
* Dmitriy Matrenichev
* ExtraClock
* Jonomir
* Serge Logvinov
* Steve Francis

### Changes
<details><summary>3 commits</summary>
<p>

* [`c603b11`](https://github.com/siderolabs/image-factory/commit/c603b11f7798faafc43ff8553e80d715a19d6640) feat: update Talos version
* [`9a030ec`](https://github.com/siderolabs/image-factory/commit/9a030ec311c36826ce9573a5cf4d0b37e81aa4bb) fix: reverse version slice on a copy
* [`8e62c9d`](https://github.com/siderolabs/image-factory/commit/8e62c9dbd3cf87e28961c809bfba42bad660fd94) feat: fetch extensions descriptions from the extensions image
</p>
</details>

### Changes from siderolabs/talos
<details><summary>25 commits</summary>
<p>

* [`e0dfbb8fb`](https://github.com/siderolabs/talos/commit/e0dfbb8fba3c50652d0ecbae1db0b0660d0766a6) fix: allow META encoded values to be compressed
* [`d677901b6`](https://github.com/siderolabs/talos/commit/d677901b672eec46b8b5edf57c680813b8fcf697) feat: implement device selector for 'physical'
* [`7d1117289`](https://github.com/siderolabs/talos/commit/7d1117289658ac04707b09f64a1dc70514a9fba9) docs: add missing talosconfig flag
* [`8a1732bcb`](https://github.com/siderolabs/talos/commit/8a1732bcb12deb4444ae87d22cc15d8b968b867d) fix: pull in `mptspi` driver
* [`c1e45071f`](https://github.com/siderolabs/talos/commit/c1e45071f0cb0e48ee35d2f87b483fffb05c6123) refactor: use etcd configuration from the EtcdSpec resource
* [`4e9b688d3`](https://github.com/siderolabs/talos/commit/4e9b688d3f8bc809e0b2f012d5e58c27de85d1e0) fix: use correct TTL for talosconfig in `talosctl config new`
* [`fb5ad0555`](https://github.com/siderolabs/talos/commit/fb5ad05551e08404cb8acde01202c4ae88ddd25a) feat: update Kubernetes default to 1.29.1
* [`fe24139f3`](https://github.com/siderolabs/talos/commit/fe24139f3c0b3f37c8266e5d6c5091950e3a647c) docs: fork docs for v1.7
* [`1c2d10ccc`](https://github.com/siderolabs/talos/commit/1c2d10ccccb84a6d1e008af23866fa13cc14d094) chore: bump dependencies
* [`a599e3867`](https://github.com/siderolabs/talos/commit/a599e38674af448fe5cac210f5d80826d3b08a12) chore: allow custom registry to build installer/imager
* [`3911ddf7b`](https://github.com/siderolabs/talos/commit/3911ddf7bd630286358f1696adf9bdac207e1b9d) docs: add how-to for cert management
* [`b0ee0bfba`](https://github.com/siderolabs/talos/commit/b0ee0bfba3f4c9172c76422a8f8f10a4046c352b) fix: strategic patch merging for audit policy
* [`474eccdc4`](https://github.com/siderolabs/talos/commit/474eccdc4cb1d0fab3ba0b370cc388bc8c9d363a) fix: watch bufer overrun for RouteStatus
* [`cc06b5d7a`](https://github.com/siderolabs/talos/commit/cc06b5d7a659a7f5a35e86a82ee242344c303302) fix: fix .der output in `talosctl gen secureboot`
* [`1dbb4abf4`](https://github.com/siderolabs/talos/commit/1dbb4abf43695d1dd18d51b0386cf644aba67d73) fix: update discovery service client to v0.1.6
* [`9782319c3`](https://github.com/siderolabs/talos/commit/9782319c31e496d998bdf9d505f32a4d8e6e937e) fix: support KubePrism settings in Kubernetes Discovery
* [`6c5a0c281`](https://github.com/siderolabs/talos/commit/6c5a0c2811e3c0f3e1ca2a8fb871065df5bf9b46) feat: generate a single JSON schema for multidoc config
* [`f70b47ddd`](https://github.com/siderolabs/talos/commit/f70b47dddc2599a618c68d8b403d9b37c61f2b71) fix: force KubePrism to connect using IPv4
* [`d5321e085`](https://github.com/siderolabs/talos/commit/d5321e085eb6c877b1b5b38d69eabb839b505297) fix: update kmsg with utf-8 fix
* [`7fa7362dd`](https://github.com/siderolabs/talos/commit/7fa7362ddc0e8a0b85cffcaebc38abd772b355e2) fix: fix nodes on dashboard footer when node names are used in `--nodes`
* [`ba88678f1`](https://github.com/siderolabs/talos/commit/ba88678f1a42b4e9f6c9de25bdc827330cfb254c) fix: merge ports and ingress configs correctly in NetworkRuleConfig
* [`dea9bda2d`](https://github.com/siderolabs/talos/commit/dea9bda2d00feeb29bf4b2c91c2ca24b6cd362f2) fix: disk UUID & WWID always empty in `talosctl disks`
* [`8dc112f36`](https://github.com/siderolabs/talos/commit/8dc112f36bd77ec72e5c501755aa4f056803efd0) chore: pull in NBD modules
* [`f6926faab`](https://github.com/siderolabs/talos/commit/f6926faab5a8b878c600d60ef9d693026277f3ee) fix: default priority for ipv6
* [`e8758dcba`](https://github.com/siderolabs/talos/commit/e8758dcbad6d3188dfccd235dbab04c19dd1a6ed) chore: support http downloads for assets in talosctl cluster create
</p>
</details>

### Dependency Changes

* **github.com/google/go-containerregistry**     v0.17.0 -> v0.18.0
* **github.com/prometheus/client_golang**        v1.17.0 -> v1.18.0
* **github.com/siderolabs/talos**                265f21be09d6 -> e0dfbb8fba3c
* **github.com/siderolabs/talos/pkg/machinery**  v1.6.0 -> e0dfbb8fba3c
* **github.com/sigstore/cosign/v2**              v2.2.1 -> v2.2.2
* **github.com/sigstore/sigstore**               v1.7.5 -> v1.8.1
* **github.com/u-root/u-root**                   v0.11.0 -> v0.12.0
* **golang.org/x/net**                           v0.19.0 -> v0.20.0
* **golang.org/x/sync**                          v0.5.0 -> v0.6.0
* **golang.org/x/sys**                           v0.15.0 -> v0.16.0

Previous release can be found at [v0.2.1](https://github.com/siderolabs/image-factory/releases/tag/v0.2.1)

## [image-factory 0.2.1](https://github.com/siderolabs/image-factory/releases/tag/v0.2.1) (2023-12-22)

Welcome to the v0.2.1 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Alexey Palazhchenko
* Andrey Smirnov
* Artem Chernyshev
* Dmitriy Matrenichev
* Tim Jones

### Changes
<details><summary>3 commits</summary>
<p>

* [`0ca3869`](https://github.com/siderolabs/image-factory/commit/0ca3869d23db1372c4dd082aff9b6249083ca018) fix: memory usage when building an installer
* [`a1421e0`](https://github.com/siderolabs/image-factory/commit/a1421e070168c09cc2159311be50b9a01dbcf4e6) feat: implement compatibility with Talos 1.2-1.3
* [`cde9b39`](https://github.com/siderolabs/image-factory/commit/cde9b3954cd2982341422cfd8c44034a1238a2df) fix: update Talos version listing
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>7 commits</summary>
<p>

* [`43d9100`](https://github.com/siderolabs/go-debug/commit/43d9100eba3a30ff0d7f1bed0058e6631243cc47) chore: allow enabling pprof manually
* [`c1bc4bf`](https://github.com/siderolabs/go-debug/commit/c1bc4bf306e54879ce9f4b002527876ac0cbf88f) chore: rekres, rename, etc
* [`3d0a6e1`](https://github.com/siderolabs/go-debug/commit/3d0a6e1bf5e3c521e83ead2c8b7faad3638b8c5d) feat: race build tag flag detector
* [`5b292e5`](https://github.com/siderolabs/go-debug/commit/5b292e50198b8ed91c434f00e2772db394dbf0b9) feat: disable memory profiling by default
* [`c6d0ae2`](https://github.com/siderolabs/go-debug/commit/c6d0ae2c0ee099fa0940405401e6a02716a15bd8) fix: linters and CI
* [`d969f95`](https://github.com/siderolabs/go-debug/commit/d969f952af9e02feea59963671298fc236ca4399) feat: initial implementation
* [`b2044b7`](https://github.com/siderolabs/go-debug/commit/b2044b70379c84f9706de74044bd2fd6a8e891cf) Initial commit
</p>
</details>

### Changes from siderolabs/talos
<details><summary>10 commits</summary>
<p>

* [`265f21be0`](https://github.com/siderolabs/talos/commit/265f21be09d68cc23764d690e9f9479b9d92d749) fix: replace the filemap implementation to not buffer in memory
* [`8db3c5b3c`](https://github.com/siderolabs/talos/commit/8db3c5b3c63ad67043b876265ac4687cdcb0f0ff) fix: pick correctly base installer image layers
* [`0a30ef784`](https://github.com/siderolabs/talos/commit/0a30ef78456e854419d0c593f9c97f40166102f3) fix: imager should support different Talos versions
* [`d6342cda5`](https://github.com/siderolabs/talos/commit/d6342cda53027eb5d46dcb6f57fbb1cc31f920dd) docs: update latest version to v1.6.1
* [`e6e422b92`](https://github.com/siderolabs/talos/commit/e6e422b92ade5f24c898e09affdb6de8ee671cb0) chore: bump dependencies
* [`5a19d078a`](https://github.com/siderolabs/talos/commit/5a19d078ad3205d201b11e0d60d5e07b379aba91) fix: properly overwrite files on install
* [`9eb6cea78`](https://github.com/siderolabs/talos/commit/9eb6cea7890854173917a096bcffd6202487d38c) docs: secureboot sd-boot menu clarification
* [`01f0cbe61`](https://github.com/siderolabs/talos/commit/01f0cbe61c32b3ff6e9d05f2c14c83223ce043fa) feat: support iPXE direct booting in `talosctl cluster create`
* [`3ba84701d`](https://github.com/siderolabs/talos/commit/3ba84701d9f87f533b3039395d350b311f4a484f) feat: pull in kernel modules for mlx Infiniband and VFIO
* [`ba993e0ed`](https://github.com/siderolabs/talos/commit/ba993e0edd20f927ff8d59f418e47c6cbf8a95b3) docs: announce that SecureBoot is available
</p>
</details>

### Dependency Changes

* **github.com/google/go-containerregistry**  v0.16.1 -> v0.17.0
* **github.com/siderolabs/go-debug**          v0.2.3 **_new_**
* **github.com/siderolabs/talos**             241bc9312edc -> 265f21be09d6

Previous release can be found at [v0.2.0](https://github.com/siderolabs/image-factory/releases/tag/v0.2.0)

## [image-factory 0.2.0](https://github.com/siderolabs/image-factory/releases/tag/v0.2.0) (2023-12-18)

Welcome to the v0.2.0 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Dmitriy Matrenichev
* Noel Georgi
* Oscar Utbult
* Artem Chernyshev
* Sebastian Gaiser
* Steve Francis
* Utku Ozdemir
* budimanjojo

### Changes
<details><summary>15 commits</summary>
<p>

* [`1318f30`](https://github.com/siderolabs/image-factory/commit/1318f301ce589a21f4461eb55c70ad4355b1a4dd) fix: azure secureboot signing
* [`296e953`](https://github.com/siderolabs/image-factory/commit/296e9533e53d71f369084cead8e44f7db7fdb30c) fix: generation of SBC images
* [`25fc50d`](https://github.com/siderolabs/image-factory/commit/25fc50d09f0be71b5c65cac92191b94136f568f1) feat: provide configuration for a custom PXE endpoint
* [`87e6f04`](https://github.com/siderolabs/image-factory/commit/87e6f04e5eaf61adadfe39ed0dfc63f5926835db) feat: update dependencies for Talos 1.6.0
* [`548128c`](https://github.com/siderolabs/image-factory/commit/548128ca9aad5cf06dfec05ce7d2502953e525a2) chore: define public const for the schematic ID extension name
* [`01fcbf1`](https://github.com/siderolabs/image-factory/commit/01fcbf16c4d70126f9c4b5693a194ca3a9a41217) feat: implement HTTP API client
* [`84113ca`](https://github.com/siderolabs/image-factory/commit/84113ca06a142aab0ab8d3146fe25d33da26a85a) feat: implement SecureBoot asset generation
* [`f82ff73`](https://github.com/siderolabs/image-factory/commit/f82ff7374546cdef6852d1d929487e044ba1f3bf) fix: properly handle from ghcr.io
* [`f36ab82`](https://github.com/siderolabs/image-factory/commit/f36ab8245aa97741cc99d10df6323090b3e0add3) fix: skip validating image index before pushing
* [`6625a89`](https://github.com/siderolabs/image-factory/commit/6625a89de2ba7c9355257138e65330bbf0c1dac0) release(v0.1.2): prepare release
* [`58378e0`](https://github.com/siderolabs/image-factory/commit/58378e0da6aa7a9cb30393e60186d022757f82b2) chore: bump dependencies and Talos
* [`db21b76`](https://github.com/siderolabs/image-factory/commit/db21b76ca8b34e9a9534342ba4516246fbb1bae4) fix: parse profiles for 'digital-ocean' platform
* [`43a6388`](https://github.com/siderolabs/image-factory/commit/43a638843689df7b04db8d8292a2d0983507d408) release(v0.1.1): prepare release
* [`4211a5c`](https://github.com/siderolabs/image-factory/commit/4211a5c52d6a5c407cecb793f47c17eb6a529051) chore: update Talos
* [`fcc8cb5`](https://github.com/siderolabs/image-factory/commit/fcc8cb503835de8b549c99281302f55acd4880c2) fix: small UI updates
</p>
</details>

### Changes from siderolabs/talos
<details><summary>76 commits</summary>
<p>

* [`241bc9312`](https://github.com/siderolabs/talos/commit/241bc9312edcadce83a64e92db807dbca74c80cc) fix: update the way secureboot signer fetches certificate (azure)
* [`59b62398f`](https://github.com/siderolabs/talos/commit/59b62398f6265f310108954e9a775e4b8c080679) chore: modernize machined/pkg/controllers/k8s
* [`760f793d5`](https://github.com/siderolabs/talos/commit/760f793d55f3965792f58fa3194977aea4f90e03) fix: use correct prefix when installing SBC files
* [`0b94550c4`](https://github.com/siderolabs/talos/commit/0b94550c42730121c3d270758286dbefa95ea61c) chore: fix the gvisor test
* [`3a787c1d6`](https://github.com/siderolabs/talos/commit/3a787c1d67ddca5102c7d9cbdab4ef1c17a605f4) docs: update 1.6 docs with Noel's feedback
* [`d803e40ef`](https://github.com/siderolabs/talos/commit/d803e40ef2cf1030aab522006ba7287bac8b64c4) docs: provide documentation for Talos 1.6
* [`9a185a30f`](https://github.com/siderolabs/talos/commit/9a185a30f79a8d3481606235609c0e5a11c880cc) feat: update Kubernetes to v1.29.0
* [`5934815d2`](https://github.com/siderolabs/talos/commit/5934815d2fe975c4d8ddb2a26ef733d29565cdb2) chore: split more kernel modules on amd64
* [`10c59a6b9`](https://github.com/siderolabs/talos/commit/10c59a6b90310b8c58babf5beb108b59f4d74e4d) fix: leave discovery service later in the reset sequence
* [`0c86ca1cc`](https://github.com/siderolabs/talos/commit/0c86ca1cc68e2646d63d19d96b01d3d5486dfc42) chore: enable kubespan+firewall for cilium tests
* [`98fd722d5`](https://github.com/siderolabs/talos/commit/98fd722d5110b1422a15ede23873bcd15ab9562e) feat: provide compatibility for future Talos 1.7
* [`131a1b167`](https://github.com/siderolabs/talos/commit/131a1b1671899666d8676b5082cef39efb8f0fa1) fix: add a KubeSpan option to disable extra endpoint harvesting
* [`4547ad9af`](https://github.com/siderolabs/talos/commit/4547ad9afa206405032618f9d94470d00ace8684) feat: send `actor id` to the SideroLink events sink
* [`04e774547`](https://github.com/siderolabs/talos/commit/04e774547146f0733633b296c4432f4eef847265) docs: cap max heading level
* [`6bb1e99aa`](https://github.com/siderolabs/talos/commit/6bb1e99aa3a8132508479b4ca8606522545d8d9a) chore: optimize pcap dump
* [`4f9d3b975`](https://github.com/siderolabs/talos/commit/4f9d3b975fa689dc9eea4e44ff453d8b68ae54ef) feat: update Kubernetes to v1.29.0-rc.2
* [`46121c9fe`](https://github.com/siderolabs/talos/commit/46121c9fecb3603c2d2ae2de6152861ee7f19eaf) docs: rework machine config documentation generation
* [`e128d3c82`](https://github.com/siderolabs/talos/commit/e128d3c827a406f96457322da87cbde2af233fa0) fix: talosctl cluster create not to enforce kubeprism always
* [`320064c5a`](https://github.com/siderolabs/talos/commit/320064c5a869de6d52ba9a23394acaa5549e7aa1) feat: update Go 1.21.5, Linux 6.1.65, etcd 3.5.11
* [`270604bea`](https://github.com/siderolabs/talos/commit/270604bead50423697d6fabffa6bbd7c7b2fbe9e) fix: support user disks via symlinks
* [`4f195dd27`](https://github.com/siderolabs/talos/commit/4f195dd271eb38446561f8708a9623324072a0e9) chore: fix the release.toml
* [`474fa0480`](https://github.com/siderolabs/talos/commit/474fa0480dd68d112a608548e4d0a0c4efa39e20) fix: store and execute desired action on emergency action
* [`515ae2a18`](https://github.com/siderolabs/talos/commit/515ae2a184374e0ac72e3321104265918e45e391) docs: extend hetzner-cloud docs for arm64
* [`eecc4dbd5`](https://github.com/siderolabs/talos/commit/eecc4dbd5198cca5b66e5c3018c407cd38b13c80) fix: trim leading spaces\newlines in inline manifest contents
* [`dbf274ddf`](https://github.com/siderolabs/talos/commit/dbf274ddf7b819941c88932e28d2fe362876ec68) fix: skip writing the file if the contents haven't changed
* [`6329222bd`](https://github.com/siderolabs/talos/commit/6329222bdcfd5ab29bc46ca03bb0b1d22ada9424) fix: do not panic in `merge.Merge` if map value is nil
* [`d8a435f0e`](https://github.com/siderolabs/talos/commit/d8a435f0e4e093570325b737f89ff3b1205de48e) fix: initialize boot assets with defaults early
* [`c6835de17`](https://github.com/siderolabs/talos/commit/c6835de17a31260af9524054483461ce3050fef2) fix: pick etcd adverised addresses from 'current' addresses
* [`6b5bc8b85`](https://github.com/siderolabs/talos/commit/6b5bc8b85b259d4ea36936d074212dd2a678208a) feat: update Linux to 6.1.64
* [`e71e3e416`](https://github.com/siderolabs/talos/commit/e71e3e41614b14f9e2ef99b69915acc0dd2c3222) feat: support extra arguments for `flanneld`
* [`36c8ddb5e`](https://github.com/siderolabs/talos/commit/36c8ddb5e1586ee8c099d768f72b3977d0381a84) feat: implement ingress firewall rules
* [`0b111ecb8`](https://github.com/siderolabs/talos/commit/0b111ecb818026c1014bbaae39b8b968baf793a9) fix: support slices of enums and fix NfTablesConntrackStateMatch
* [`9a8521741`](https://github.com/siderolabs/talos/commit/9a85217412e81e5b7228b4facb1e91330eed5cc3) feat: improve nftables backend
* [`db4e2539d`](https://github.com/siderolabs/talos/commit/db4e2539d4dbf4533e628df370e5dc9963235524) feat: update Kubernetes 1.29.0-rc.1 and other bumps
* [`7a4a92854`](https://github.com/siderolabs/talos/commit/7a4a92854f960dc8976ca190160545516a4820b3) feat: support sanitized kernel args
* [`f041b2629`](https://github.com/siderolabs/talos/commit/f041b262996874b17700ae55e87d8afc20381b50) chore: add tests for mdadm extension
* [`e46e6a312`](https://github.com/siderolabs/talos/commit/e46e6a312f314bf2ab8131736f690584e4d5b5f1) feat: implement nftables backend
* [`ba827bf8b`](https://github.com/siderolabs/talos/commit/ba827bf8b8b9a5acdb43c45116b88add99e952bf) chore: support getting multiple endpoints from the `Provision` rpc call
* [`dd45dd06c`](https://github.com/siderolabs/talos/commit/dd45dd06cf885a3fe89f17de81491bdd18d88e68) chore: add custom node taints
* [`8e2307466`](https://github.com/siderolabs/talos/commit/8e230746655d5cb10d7b982f27ea078da5fd0374) docs: fix talosctl pcap argument
* [`e4a050cb1`](https://github.com/siderolabs/talos/commit/e4a050cb1d765efc7f52241563a6f3649ee8339c) docs: fix talosctl inspect dependencies example indentation
* [`fbcf4264f`](https://github.com/siderolabs/talos/commit/fbcf4264ff0875b67e8852b7fc2099d0d55a6a13) docs: fix talosctl dashboard cli docs
* [`70d53ee13`](https://github.com/siderolabs/talos/commit/70d53ee13c17a351e4a9509a2fdd43171fcc929b) chore: deprecate .persist and .extensions
* [`95e33f6fc`](https://github.com/siderolabs/talos/commit/95e33f6fcef8993afeb73e876044e8ddbab718f0) release(v1.6.0-alpha.2): prepare release
* [`514e514ba`](https://github.com/siderolabs/talos/commit/514e514ba650419a4caad4ee87c52a367ce1e323) feat: update Linux 6.1.63, containerd 1.7.9
* [`aca8b5e17`](https://github.com/siderolabs/talos/commit/aca8b5e179962c8e1dc27ca8de527e981f763004) fix: ignore kernel command line in container mode
* [`020a0eb63`](https://github.com/siderolabs/talos/commit/020a0eb63ea39d25faa8eba8568584243d814457) docs: fix table formatting for bootstraprequest
* [`0eb245e04`](https://github.com/siderolabs/talos/commit/0eb245e04374cd21a369d298b73e8bc6db11d153) docs: fix talosctl pcap example indentation
* [`de6caf534`](https://github.com/siderolabs/talos/commit/de6caf5348f815dddbd4a595d40d4c4ad71282bc) docs: fix table formatting for machineservice api
* [`27d208c26`](https://github.com/siderolabs/talos/commit/27d208c26bd1fe5a37b127cd83cab76b5671758a) feat: implement OAuth2 device flow for machine config
* [`5c8fa2a80`](https://github.com/siderolabs/talos/commit/5c8fa2a80382b6ea83d81c434b2e28a9901fdcad) chore: start containerd early in boot
* [`95a252cfc`](https://github.com/siderolabs/talos/commit/95a252cfc91eeeeb48ac3b3e3cd6ad7ba14ab1eb) docs: fix link in what is new page
* [`0d3c3ed71`](https://github.com/siderolabs/talos/commit/0d3c3ed716670c80d33351d912620e5b91f6c7e3) feat: support kube scheduler config
* [`06941b7e5`](https://github.com/siderolabs/talos/commit/06941b7e5ca4f937c1996828e5a543967902656d) fix: allow rootfs propagation configuration for extension services
* [`57dc796f3`](https://github.com/siderolabs/talos/commit/57dc796f381e87f398cfed3ac7cd87ff51454b75) docs: update lastRelease to v1.5.5 in _index.md
* [`21d944a64`](https://github.com/siderolabs/talos/commit/21d944a643d8eec104d703cc8995e9ac80d2417b) docs: add timezone information
* [`4f1ad16c7`](https://github.com/siderolabs/talos/commit/4f1ad16c764e643f7bf71ed8ca46e840875011ec) feat: support kubelet credentialprovider config
* [`71a3bf0e3`](https://github.com/siderolabs/talos/commit/71a3bf0e3e42117e7283b41116419d7d2f28d82c) fix: allow extra kernel args for secureboot installer
* [`f38eaaab8`](https://github.com/siderolabs/talos/commit/f38eaaab87f77f33b0317d4405c84575023ee0da) feat: rework secureboot and PCR signing key
* [`6eade3d5e`](https://github.com/siderolabs/talos/commit/6eade3d5ef5c5356d0bfc0e3d52263a39d2e9f1a) chore: add ability to rewrite uuids and set unique tokens for Talos
* [`e9c7ac17a`](https://github.com/siderolabs/talos/commit/e9c7ac17a9b707950b249e08e11ed7ddac64e8ae) fix: set max msg recv size when proxying
* [`e22ab440d`](https://github.com/siderolabs/talos/commit/e22ab440d7794a9c46edf1357124571057b6b19d) feat: update Linux 6.1.61, containerd 1.7.8, runc 1.1.10
* [`8245361f9`](https://github.com/siderolabs/talos/commit/8245361f9cfb66d68bc54330a47814eb730eb839) feat: show first 32 bytes of response body on download error
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

* **github.com/siderolabs/talos**                cbe6e7622d01 -> 241bc9312edc
* **github.com/siderolabs/talos/pkg/machinery**  cbe6e7622d01 -> v1.6.0
* **github.com/sigstore/cosign/v2**              v2.2.0 -> v2.2.1
* **golang.org/x/net**                           v0.17.0 -> v0.19.0
* **golang.org/x/sync**                          v0.4.0 -> v0.5.0
* **golang.org/x/sys**                           v0.13.0 -> v0.15.0

Previous release can be found at [v0.1.0](https://github.com/siderolabs/image-factory/releases/tag/v0.1.0)

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

