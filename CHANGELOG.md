## [image-factory 0.3.1](https://github.com/siderolabs/image-factory/releases/tag/v0.3.1) (2024-04-10)

Welcome to the v0.3.1 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Noel Georgi
* Dmitriy Matrenichev
* Utku Ozdemir
* Dmitry Sharshakov
* Spencer Smith
* Artem Chernyshev
* Justin Garrison
* Mattias Cockburn
* Andrei Kvapil
* AvnarJakob
* Christian Mohn
* Christian WALDBILLIG
* Dmitry Sharshakov
* Evan Johnson
* Fabiano Fidêncio
* Henno Schooljan
* Jean-Tiare Le Bigot
* Kai Hanssen
* Louis SCHNEIDER
* Matthieu S
* Michael Stephenson
* Niklas Wik
* Pip Oomen
* Saiyam Pathak
* Sebastiaan Gerritsen
* Steve Francis
* bri
* ebcrypto
* edwinavalos
* fazledyn-or
* goodmost
* james-dreebot
* pardomue
* shurkys
* stereobutter

### Changes
<details><summary>13 commits</summary>
<p>

* [`762cf2b`](https://github.com/siderolabs/image-factory/commit/762cf2b40c609b460ffe8c82be49c2aa75b781df) fix: generation of SecureBoot ISO
* [`ae1f0a3`](https://github.com/siderolabs/image-factory/commit/ae1f0a3c1b6e68bd6ef5a8ea852cb7c67a49c02c) fix: sort extensions in the UI schematic generator
* [`c2de13f`](https://github.com/siderolabs/image-factory/commit/c2de13f682b1a2add2983436698d12561a7f5bf9) release(v0.3.0): prepare release
* [`7062392`](https://github.com/siderolabs/image-factory/commit/70623924c4a872b6cf7cdf08221350263f93c123) chore: update Talos dependency to 1.7.0-beta.0
* [`78f8944`](https://github.com/siderolabs/image-factory/commit/78f8944cbb8e673e0726250308b72eaf562d6290) feat: add cert issuer regexp option
* [`c0981e8`](https://github.com/siderolabs/image-factory/commit/c0981e849d2146313dd179b9174b7686f5c27846) feat: add support for -insecure-schematic-service-repository flag
* [`5d779bb`](https://github.com/siderolabs/image-factory/commit/5d779bb38adcc2a9dcd526683d8ea77eb94b0388) chore: bump dependencies
* [`93eb7de`](https://github.com/siderolabs/image-factory/commit/93eb7de1f6432ac31d34f5cccbf9ff40587e65bc) feat: support overlay
* [`df3d211`](https://github.com/siderolabs/image-factory/commit/df3d2119e49a4c6e09c8a4261e1bd679ab408a23) release(v0.2.3): prepare release
* [`4ccf0e5`](https://github.com/siderolabs/image-factory/commit/4ccf0e5d7ed44e39d97ab45040cca6665618f4fa) fix: ignore missing DTB and other SBC artifacts
* [`c7dba02`](https://github.com/siderolabs/image-factory/commit/c7dba02d17b068e576de7c155d5a5e58fa156a76) chore: run tailwindcss before creating image
* [`81f2cb4`](https://github.com/siderolabs/image-factory/commit/81f2cb437f71e4cb2d92db71a6f2a2b7becb8b56) chore: bump dependencies, rekres
* [`07095cd`](https://github.com/siderolabs/image-factory/commit/07095cd4966ab8943d93490bd5a9bc5085bec2f8) chore: re-enable govulncheck
</p>
</details>

### Changes since v0.3.0
<details><summary>2 commits</summary>
<p>

* [`762cf2b`](https://github.com/siderolabs/image-factory/commit/762cf2b40c609b460ffe8c82be49c2aa75b781df) fix: generation of SecureBoot ISO
* [`ae1f0a3`](https://github.com/siderolabs/image-factory/commit/ae1f0a3c1b6e68bd6ef5a8ea852cb7c67a49c02c) fix: sort extensions in the UI schematic generator
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`238baf9`](https://github.com/siderolabs/gen/commit/238baf95e228d40f9f5b765b346688c704052715) chore: add typesafe `SyncMap` and bump stuff
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>1 commit</summary>
<p>

* [`0c2be80`](https://github.com/siderolabs/go-debug/commit/0c2be80d9d60034f3352a34841b615ef7bb0a62c) chore: run rekres (update to Go 1.22)
</p>
</details>

### Changes from siderolabs/talos
<details><summary>158 commits</summary>
<p>

* [`145f24063`](https://github.com/siderolabs/talos/commit/145f2406307e57a6f2eb1601d4f7d542d39a9f51) fix: don't modify a global map of profiles
* [`6fe91ad9c`](https://github.com/siderolabs/talos/commit/6fe91ad9cf9f99401fc39a6ece24eed61f17b0e2) feat: provide Kubernets/Talos version compatibility for 1.8
* [`909a5800e`](https://github.com/siderolabs/talos/commit/909a5800e4a9ada42288ae15992579e9acf6c372) fix: generate secureboot ISO .der certificate correctly
* [`b0fdc3c8c`](https://github.com/siderolabs/talos/commit/b0fdc3c8caaf6ef756cdc4440dae45891bd96d01) fix: make static pods check output consistent
* [`c6ad0fcce`](https://github.com/siderolabs/talos/commit/c6ad0fcceb8220f0bf96a45e131ba999cb723f79) fix: validate that workers don't get cluster CA key
* [`3735add87`](https://github.com/siderolabs/talos/commit/3735add87cec47038a88ba641322c26cd487ac58) fix: reconnect to the logs stream in dashboard after reboot
* [`9aa1e1b79`](https://github.com/siderolabs/talos/commit/9aa1e1b79b4a02902e0573c10e1c0bf71a2341af) fix: present all accepted CAs to the kube-apiserver
* [`336e61174`](https://github.com/siderolabs/talos/commit/336e61174624741f697c77b98dd84ab9a7a749f4) fix: close the apid connection to other machines gracefully
* [`ff2c427b0`](https://github.com/siderolabs/talos/commit/ff2c427b04963d69ba2eaa1084a0a078d742b9ac) fix: pre-create nftables chain to make kubelet use nftables
* [`5622f0e45`](https://github.com/siderolabs/talos/commit/5622f0e450eda589f4b9a2af28b8517d08c2aae2) docs: change localDNS to hostDNS in release notes yaml section
* [`01d8b897c`](https://github.com/siderolabs/talos/commit/01d8b897c4f734299b9509f404493b0cdccb8789) fix: make safeReset truly safe to call multiple times
* [`653f838b0`](https://github.com/siderolabs/talos/commit/653f838b09ca7e6ac5098016eef2a03d55a6a334) feat: support multiple Docker cluster in talosctl cluster create
* [`951904554`](https://github.com/siderolabs/talos/commit/951904554ee8a464794cc2f9a9fb6bc084791c06) chore: bump dependencies (go 1.22.2)
* [`862c76001`](https://github.com/siderolabs/talos/commit/862c76001b6b1c7c98910ed54e9198e60f381985) feat: add support for CoreDNS forwarding to host DNS
* [`e8ae5ef63`](https://github.com/siderolabs/talos/commit/e8ae5ef63af13a40bd88bab03c54703eccdbfcc7) feat: add akamai platform support
* [`5c0f74b37`](https://github.com/siderolabs/talos/commit/5c0f74b377757ffee8f455ac2bcd9d81e3bf7717) fix: don't announce the VIP on acquire failure
* [`2f0fe10d5`](https://github.com/siderolabs/talos/commit/2f0fe10d557960a3dacd73922d4f0dc3e65614c5) chore: update sbc docs
* [`1b17008e9`](https://github.com/siderolabs/talos/commit/1b17008e9df8fe2988d0ad31a4f1604eba783fe3) fix: handle more OpenStack link types
* [`e7d804140`](https://github.com/siderolabs/talos/commit/e7d80414041a6911181c80d2089f0fed6e9640e6) fix: always update firewall rules (kubespan)
* [`78b9bd927`](https://github.com/siderolabs/talos/commit/78b9bd9273b1881d27c7ab364491dbb9c09d6df0) fix: report unsupported x86_64 microarchitecture level
* [`71d90ba5f`](https://github.com/siderolabs/talos/commit/71d90ba5f32c2e0679530121bd8b0d7db5285c8d) fix: retry in the fixed amount of time if grpc relay failed
* [`d320498a4`](https://github.com/siderolabs/talos/commit/d320498a44736e818e9a5f4b7a9d626cda028cd0) chore: bump dependencies
* [`3195e5d15`](https://github.com/siderolabs/talos/commit/3195e5d15cb6d1e4cc8251edf6cdcdc9f5601ed4) fix: force Flannel CNI to use KubePrism Kubernetes API endpoint
* [`917043fb5`](https://github.com/siderolabs/talos/commit/917043fb558aba95a1256523c11777d1e68fe3a9) chore: bump tools, pkgs and extra to stable
* [`f515741b5`](https://github.com/siderolabs/talos/commit/f515741b521281e832a706d7d1d5105cb627524d) chore: add equinix e2e-tests
* [`117e60583`](https://github.com/siderolabs/talos/commit/117e60583d4ec2e953174d49386fc4b1b80a7d0c) feat: add support for static extra fields for JSON logs
* [`090143b03`](https://github.com/siderolabs/talos/commit/090143b030df8387b6b44ea2fee525d493a1727c) fix: allow platform cmdline args to be platform-specific
* [`7a68504b6`](https://github.com/siderolabs/talos/commit/7a68504b6b451018e3a5e2589d03aa029b4216f6) feat: support rotating Kubernetes CA
* [`fac3dd043`](https://github.com/siderolabs/talos/commit/fac3dd04308b28a50338dcb077d399aac61adde2) fix: don't set default endpoints on gen config
* [`8dc4910c4`](https://github.com/siderolabs/talos/commit/8dc4910c48dbdb631f72512c1fb19a84b3ee1130) chore: enable "WG over GRPC" testing in siderolink agent tests
* [`bac366e43`](https://github.com/siderolabs/talos/commit/bac366e43e0c95e1d5089728e92daf8b5b8e410d) chore: add `ExtraInfo` field for extensions
* [`0fc24eeb0`](https://github.com/siderolabs/talos/commit/0fc24eeb09d3fafb5eee9df94c6bc3856b716ae0) feat: provide insecure flag to imager
* [`a6b2f5456`](https://github.com/siderolabs/talos/commit/a6b2f545648a570d06c6f840bb688e3176f28c6e) feat: update Kubernetes to 1.30.0-rc.0, etcd to 3.5.13
* [`0361ff895`](https://github.com/siderolabs/talos/commit/0361ff89560556c8f3f3039afdda78d2e4c79794) docs: quickstart video and brew install
* [`b752a8618`](https://github.com/siderolabs/talos/commit/b752a86183c990848f05d86fa1b41942b4f1610c) chore: talosctl: add openSUSE OVMF paths
* [`945648914`](https://github.com/siderolabs/talos/commit/94564891475273ca3dbaccdf32660567d8e8f3fd) feat: support hardware watchdog timers
* [`949ad11a2`](https://github.com/siderolabs/talos/commit/949ad11a2d6374c518ed50a628a1f069a05345f3) chore: import siderolink as `siderolink-launch` subcommand
* [`ee51f04af`](https://github.com/siderolabs/talos/commit/ee51f04af33ddaf7aa673225761b55599d1b7252) chore: azure e2e
* [`55dd41c0d`](https://github.com/siderolabs/talos/commit/55dd41c0dfd837e8ad20b924ffe4be4b1fcf5ec7) chore: update coredns to v1.11.2 in required section
* [`8eacc4ba8`](https://github.com/siderolabs/talos/commit/8eacc4ba8024abba834af811d1413f267f588219) feat: support rotation of Talos API CA
* [`92808e3bc`](https://github.com/siderolabs/talos/commit/92808e3bcff2fbbabf4cfd4c8f48acc0f25ef4e4) feat: report Docker node resources in cluster show
* [`84ec8c16f`](https://github.com/siderolabs/talos/commit/84ec8c16f30d2619ae85804df0601f6d92464a08) feat: support syncing to PTP clocks
* [`7d43c9aa6`](https://github.com/siderolabs/talos/commit/7d43c9aa6b7730fab8c749ef275ee3ad30a7de50) chore: annotate installer errors
* [`f737e6495`](https://github.com/siderolabs/talos/commit/f737e6495cda3588e3c71e7ee3e65823b54b9014) fix: populate routes to BGP neighbors (Equinix Metal)
* [`19f15a840`](https://github.com/siderolabs/talos/commit/19f15a840ccc5117f8729aef32449e2fb331340e) chore: bump golangci-lint to 1.57.0
* [`684011963`](https://github.com/siderolabs/talos/commit/6840119632e2b1869a30e69cfc57fd852824dbeb) docs: add docs for overlays
* [`9b6ec5929`](https://github.com/siderolabs/talos/commit/9b6ec5929a6d26ea936e7918076f2beba8d355d8) chore: bump kernel
* [`69f0466cd`](https://github.com/siderolabs/talos/commit/69f0466cd8e3a102c2e0eb4f742b324dea2055b0) docs: remove repetitive words
* [`113fb646e`](https://github.com/siderolabs/talos/commit/113fb646ec14c840c892666ebac5df3350a6a40d) chore: use `go-talos-support` library
* [`89fc68b45`](https://github.com/siderolabs/talos/commit/89fc68b4595a075753e6ed3fb52b34955611e9e0) fix: service lifecycle issues
* [`ead37abf0`](https://github.com/siderolabs/talos/commit/ead37abf097ad91cc8d29959db1f17e661edacf6) test: disable volume tests
* [`c64523a7a`](https://github.com/siderolabs/talos/commit/c64523a7a136469af7c6d39cb61f28c86678f74c) feat: update Flannel to v0.24.4
* [`15beb1478`](https://github.com/siderolabs/talos/commit/15beb147804b17060f20f4284b16213746370df8) feat: implement blockdevice watch controller
* [`06e3bc0cb`](https://github.com/siderolabs/talos/commit/06e3bc0cbd82a823963a130df72d69dffa52def7) feat: implement Siderolink wireguard over GRPC
* [`9afa70baf`](https://github.com/siderolabs/talos/commit/9afa70baf3b4f6b46705e5f1c5d25ad0c383f596) fix: patch correctly config in `talosctl upgrade-k8s`
* [`3130caf95`](https://github.com/siderolabs/talos/commit/3130caf95444318f38ba7a4a885d021868d37827) chore: re-enable DRBD extension
* [`3ba180d07`](https://github.com/siderolabs/talos/commit/3ba180d07d494032e188b401f1a0d87e8549e293) release(v1.7.0-alpha.1): prepare release
* [`403ad93c3`](https://github.com/siderolabs/talos/commit/403ad93c35b4cee9c012addb4667cb04e23e1c61) feat: update dependencies
* [`7376f34e8`](https://github.com/siderolabs/talos/commit/7376f34e823f6399ed2c66ae1296a8a47a0a00ef) fix: remove maintenance config when maintenance service is shut down
* [`952801d8b`](https://github.com/siderolabs/talos/commit/952801d8b2af27a49531b8a19f8b74400b6d4eb8) fix: handle overlay partition options
* [`465b9a4e6`](https://github.com/siderolabs/talos/commit/465b9a4e6ca9367326cb862b501f1146989b07d4) fix: update discovery client with the fix for keepalive interval
* [`1e9f866ac`](https://github.com/siderolabs/talos/commit/1e9f866aca14ec5ecc4d5619f42e02d44b6968d1) feat: update Kubernetes to v1.30.0-beta.0
* [`d118a852b`](https://github.com/siderolabs/talos/commit/d118a852b995f13fc5160acb7c95d2186adaac41) feat: implement `Install` for imager overlays
* [`cd5a5a447`](https://github.com/siderolabs/talos/commit/cd5a5a4474914cb64a23698b6656763b253a4d01) chore: migrate to go-grpc-middleware/v2
* [`e3c2a6398`](https://github.com/siderolabs/talos/commit/e3c2a639810ad325c2b5d1b1a92aa09d52ac6997) feat: set default NTP server to time.cloudflare.com
* [`32e087760`](https://github.com/siderolabs/talos/commit/32e08776078f9ca78ed27a382665589229c0ccb4) chore: print all available logs containers in `logs` command completions
* [`e89d755c5`](https://github.com/siderolabs/talos/commit/e89d755c523065a257d34dff9a88df97fc1908b3) fix: etcd config validation for worker
* [`1aa3c9182`](https://github.com/siderolabs/talos/commit/1aa3c91821fb9889e9859c880d602457791f6a14) docs: add DreeBot to ADOPTERS.md
* [`1bb6027cc`](https://github.com/siderolabs/talos/commit/1bb6027ccd7c63ae3a012eb310d1e05027ec1f80) fix: fix nil panic on maintenance upgrade with partial config
* [`aa70bfb9d`](https://github.com/siderolabs/talos/commit/aa70bfb9dc4fc886a6c5b771947a146ee2f58ef7) docs: add Redpill Linpro to adopters list
* [`f02aeec92`](https://github.com/siderolabs/talos/commit/f02aeec922b6327dad6d4fee917987b147abbf2a) fix: do not fail cluster create when input dir does not contain talosconfig
* [`1ec6683e0`](https://github.com/siderolabs/talos/commit/1ec6683e0c1d60b55a25e495c2dfc18f5bbf05b0) chore: use go-copy
* [`3c8f51d70`](https://github.com/siderolabs/talos/commit/3c8f51d707b897fb34ed3a9f7c32b7cd3e5ee5b0) chore: move cli formatters and version modules to machinery
* [`8152a6dd6`](https://github.com/siderolabs/talos/commit/8152a6dd6b7484e3f313b7cc9dd84fefba84d106) feat: update Go to 1.22.1
* [`8c7953991`](https://github.com/siderolabs/talos/commit/8c79539914324eee64dbdaf1f535fc4e20da55e8) docs: update replicated-local-storage-with-openebs-jiva.md
* [`f23bd8144`](https://github.com/siderolabs/talos/commit/f23bd81448b640b37006d6bfffa9315f84cad492) fix: syslog parser
* [`bbed07e03`](https://github.com/siderolabs/talos/commit/bbed07e03a815869cbae5aaa2667864697fd5d65) feat: update Linux to 6.6.18
* [`8125e754b`](https://github.com/siderolabs/talos/commit/8125e754b8a4c8db891dcd2dbd6ee3702daa2393) feat: imager overlay
* [`0b9b4da12`](https://github.com/siderolabs/talos/commit/0b9b4da12abe6bf19d9eaaa48b42cd1a794ca8fa) feat: update Kubernetes to 1.30.0-alpha.3
* [`3a764029e`](https://github.com/siderolabs/talos/commit/3a764029ea2d3f888c2d4d83ebffd6f97a46e3a9) docs: fix typo in word governor
* [`d81d49000`](https://github.com/siderolabs/talos/commit/d81d4900030e93cacda34646732f24816dd3d85f) chore: update CoreDNS renovate source
* [`b2ad5dc5f`](https://github.com/siderolabs/talos/commit/b2ad5dc5f809da9665b41c25d9ab6359a87ec942) fix: workaround a race in CNI setup (talosctl cluster create)
* [`457507803`](https://github.com/siderolabs/talos/commit/457507803d302a31b47f5e386ce1e398861550bd) fix: provide auth when pulling images in the imager
* [`e707175ab`](https://github.com/siderolabs/talos/commit/e707175ab5bdeb0f79ad242e2c81f36eec928342) docs: update config patch in cilium docs
* [`f8c556a1c`](https://github.com/siderolabs/talos/commit/f8c556a1ce9aa49c1af1bfe97c3694c00fcc67bc) chore: listen for dns requests on 127.0.0.53
* [`8872a7a21`](https://github.com/siderolabs/talos/commit/8872a7a2105034d8d6550e628355fe5f09131691) fix: ignore 'no such device' in addition to 'no such file'
* [`1cb544353`](https://github.com/siderolabs/talos/commit/1cb5443530abc2f6333566ec8e8429b2a784f791) chore: uki der certs in iso
* [`67ac6933d`](https://github.com/siderolabs/talos/commit/67ac6933d3c23b8ea31f01bd45d0192573e64ef3) fix: handle errors to watch apid/trustd certs
* [`c79d69c2e`](https://github.com/siderolabs/talos/commit/c79d69c2e25ee588f45a8978117300c31871f749) fix: only set gateway if set in context (opennebula)
* [`4575dd8e7`](https://github.com/siderolabs/talos/commit/4575dd8e741e99ab92ac63afdf48d816562f744c) chore: allow not preallocated disks for QEMU cluster
* [`0bddfea81`](https://github.com/siderolabs/talos/commit/0bddfea818994288285f442c27a339e6d1dc6cf0) chore: add oceanbox.io to adopters
* [`136427592`](https://github.com/siderolabs/talos/commit/1364275926df312204e006751dacc7af8e7d6726) chore: use proper `talos_version_contract` for TF tests
* [`6bf50fdc1`](https://github.com/siderolabs/talos/commit/6bf50fdc14ad97d97fd8fcec3132f0b183c93e5a) chore: disable x/net/trace in gRPC to enable dead code elimination
* [`815a8e9cc`](https://github.com/siderolabs/talos/commit/815a8e9cc5ad2c22acf11f223d8a64abbbf4b3cb) feat: add partial config support to `talosctl cluster create`
* [`64e9703f8`](https://github.com/siderolabs/talos/commit/64e9703f8648f997ff2e2e0fff932f74fd52d585) chore: add tests for the Kata Containers extension
* [`9b6291925`](https://github.com/siderolabs/talos/commit/9b62919253f16cbbfec999da26f11e8751fbb345) feat: update pkgs
* [`66f3ffdd4`](https://github.com/siderolabs/talos/commit/66f3ffdd4ad69ec690c680868cc95697eb1fba48) fix: ensure that Talos runs in a pod (container)
* [`9dbc33972`](https://github.com/siderolabs/talos/commit/9dbc33972a2ded3818fabd9b157604d26926e3c9) feat: add basic syslog implementation
* [`0b7a27e6a`](https://github.com/siderolabs/talos/commit/0b7a27e6a122e7cacb5ff82a7f6cae005435ae54) feat: allow access to all resources over siderolink in maintenance mode
* [`53721883d`](https://github.com/siderolabs/talos/commit/53721883d50bd9979edeb4f94a0f1cfcf74d4d80) feat: support AWS KMS for the SecureBoot signing
* [`7ee999f8a`](https://github.com/siderolabs/talos/commit/7ee999f8a3906eda23b7657da4c4212886a81626) fix: disable KubeSpan endpoint harvesting by default
* [`7b87c7fe9`](https://github.com/siderolabs/talos/commit/7b87c7fe97d01f33eb621bb631d482f975da3feb) chore: bump Go dependencies
* [`8e9596d3c`](https://github.com/siderolabs/talos/commit/8e9596d3c65246824e921f6cb9dfcda96b5ff52c) docs: rpi talosctl install update
* [`493bb60f8`](https://github.com/siderolabs/talos/commit/493bb60f81075181c4f71af546674871f4616067) fix: correctly handle partial configs in `DNSUpstreamController`
* [`6deb10ae2`](https://github.com/siderolabs/talos/commit/6deb10ae25efa1d96dd7416045c99b178b04e020) chore: deprecate `environmentFile` for extensions
* [`f8b4ee82a`](https://github.com/siderolabs/talos/commit/f8b4ee82aeba990d8e34b7c95debf30c4a626298) chore: update extensions test
* [`1366ce14a`](https://github.com/siderolabs/talos/commit/1366ce14a8b0bf72ac884147497e354fb33ef3fa) feat: update Kubernetes to v1.30.0-alpha.2
* [`559308ef7`](https://github.com/siderolabs/talos/commit/559308ef7e482786cc3554002bcd9fb05e0459c8) fix: use MachineStatus resource to check for boot done
* [`15e8bca2b`](https://github.com/siderolabs/talos/commit/15e8bca2b2f839ee138faa14cb3931af173d258f) feat: support environment in `ExtensionServicesConfig`
* [`3fe82ec46`](https://github.com/siderolabs/talos/commit/3fe82ec461995b680ecf060af75b47cd175a6342) feat: custom image settings for k8s upgrade
* [`fa3b93370`](https://github.com/siderolabs/talos/commit/fa3b93370501009283e110b74876b18ce6bad4f9) chore: replace fmt.Errorf with errors.New where possible
* [`d4521ee9c`](https://github.com/siderolabs/talos/commit/d4521ee9c472622fb2ef3c8570c1fa1c46332c16) feat: update kernel with sfc driver and LSM updates
* [`2f0421b40`](https://github.com/siderolabs/talos/commit/2f0421b406ee252e9197c0b4589c0b33662bef34) fix: run xfs_repair on invalid argument error
* [`f868fb8e8`](https://github.com/siderolabs/talos/commit/f868fb8e8f50e1acaa1743001d5b4f702bf29294) docs: update vmware tools url
* [`fa2d34dd8`](https://github.com/siderolabs/talos/commit/fa2d34dd8875e6a09c257acfb9321c1230658b87) chore: enable v6 support on the same port
* [`83e0b0c19`](https://github.com/siderolabs/talos/commit/83e0b0c19aaca7d413483b3a908c9dc3b4289203) chore: adjust dns sockets settings
* [`a1ec1705b`](https://github.com/siderolabs/talos/commit/a1ec1705bc5d1f7c66dbb8549af42fc3b4778400) chore: update Go to 1.22.0
* [`76b50fcd4`](https://github.com/siderolabs/talos/commit/76b50fcd4ae2a5d602997cc360c9dcb45e4243e8) chore: add Ænix to the Adopters list
* [`5324d3916`](https://github.com/siderolabs/talos/commit/5324d391671dfbf918aee1bd6b095adffadecf8e) chore: bump stuff
* [`087b50f42`](https://github.com/siderolabs/talos/commit/087b50f42932e4da883de254984bce4ad7858b90) feat: support systemd-boot ISO enroll keys option
* [`afa71d6b0`](https://github.com/siderolabs/talos/commit/afa71d6b028c33333db51495a3db41b758f38435) chore: use "handle-like" resource in `DNSResolveCacheController`
* [`013e13070`](https://github.com/siderolabs/talos/commit/013e130702758dcd8f44c84de8090d624aa5c7b9) fix: error with decoding config document with wrong apiVersion
* [`1e77bb1c3`](https://github.com/siderolabs/talos/commit/1e77bb1c3dde3c6a54bc4174eafc09846ff59e62) chore: allow custom pkgs to build talos
* [`3f8a85f1b`](https://github.com/siderolabs/talos/commit/3f8a85f1b390936cf7d76a146f6b76973be1e474) fix: unlock the upgrade mutex properly
* [`61c3331b1`](https://github.com/siderolabs/talos/commit/61c3331b148901a3137de6a087d561a6db8f4dfc) docs: update indentation in vip.md
* [`383e528df`](https://github.com/siderolabs/talos/commit/383e528df8c52ad44402c830fb3611b66c71fc7a) chore: allow uuid-based hostnames in talosctl cluster create
* [`1e6c8c4de`](https://github.com/siderolabs/talos/commit/1e6c8c4dec1e71f0d83914c3a0d7b907b21dc3b0) feat: extensions services config
* [`989ca3ade`](https://github.com/siderolabs/talos/commit/989ca3ade194bb0cd5c162d5d8973c133e381501) feat: add OpenNebula platform support
* [`914f88778`](https://github.com/siderolabs/talos/commit/914f88778838abe51f24ec3a9574e91836561e9e) docs: update nocloud.md Proxmox information
* [`a04cc8015`](https://github.com/siderolabs/talos/commit/a04cc80154ed94e970615714fd8dff9cd8cf8ca9) fix: pass TTL when generating client certificate
* [`3fe8c12ca`](https://github.com/siderolabs/talos/commit/3fe8c12ca654790695417b3d4f6bb5517e5902b5) fix: add log line about controller runtime failing
* [`ddbabc7e5`](https://github.com/siderolabs/talos/commit/ddbabc7e58e476c95d7bb15f325f612a3d8fc86c) fix: use a separate cgroup for each extension service
* [`6ccdd2c09`](https://github.com/siderolabs/talos/commit/6ccdd2c09c88eb2fe8b5b382dbd94816865381d3) chore: fix markdown-lint call
* [`4184e617a`](https://github.com/siderolabs/talos/commit/4184e617ab92b8f41c2540bf55aa4d502778dcad) chore: add test for wasmedge runtime extension
* [`95ea3a6c6`](https://github.com/siderolabs/talos/commit/95ea3a6c65a952fef533016b7116212c21609aac) chore: bump timeout in acquire tests
* [`c19a505d8`](https://github.com/siderolabs/talos/commit/c19a505d8cde234e12f729183e8c7272ac049159) chore: bump docker dind image
* [`d7d4154d5`](https://github.com/siderolabs/talos/commit/d7d4154d5dc817f91771b25b358825dae803de7f) chore: remove channel blocking in qemu launch
* [`029d7f7b9`](https://github.com/siderolabs/talos/commit/029d7f7b9b2ba610b9bd68dd00a9d8a060bfd280) release(v1.7.0-alpha.0): prepare release
* [`2ff81c06b`](https://github.com/siderolabs/talos/commit/2ff81c06bc1123af2fa7286fff15d9de0b8a868a) feat: update runc 1.1.12, containerd 1.7.13
* [`9d8cd4d05`](https://github.com/siderolabs/talos/commit/9d8cd4d058e73d30e4864e67377cf55390467725) chore: drop deprecated method EtcdRemoveMember
* [`17567f19b`](https://github.com/siderolabs/talos/commit/17567f19be39eeaf0d9a9aa3cd773b73d537814a) fix: take into account the moment seen when cleaning up CRI images
* [`aa03204b8`](https://github.com/siderolabs/talos/commit/aa03204b864d8d8ac5a7ee4986a06230863043fb) docs: document the process of building custom kernel packages
* [`7af48bd55`](https://github.com/siderolabs/talos/commit/7af48bd5598e61357cdb9b31dd57de6479b1ce7c) feat: use RSA key for kube-apiserver service account key
* [`a5e13c696`](https://github.com/siderolabs/talos/commit/a5e13c696d1e1cb8e894a4133791c74470687553) fix: retry blockdevice open in the installer
* [`593afeea3`](https://github.com/siderolabs/talos/commit/593afeea38a75de01041e3126cb0ad3443f6e1a1) fix: run the interactive installer loop to report errors
* [`87be76b87`](https://github.com/siderolabs/talos/commit/87be76b8788d179058be14c53e1092054b08c5dd) fix: be more tolerant to error handling in Mounts API
* [`03add7503`](https://github.com/siderolabs/talos/commit/03add750309dcdeb7c2b87cd72da29a3e228e56e) docs: add section on using imager with extensions from tarball
* [`ee0fb5eff`](https://github.com/siderolabs/talos/commit/ee0fb5effce82fec99860b5910e0fb6e5147b49b) docs: consolidate certificate management articles
* [`9c14dea20`](https://github.com/siderolabs/talos/commit/9c14dea209bba69b471fd43eb2e8ba05de3ff549) chore: bump coredns
* [`ebeef2852`](https://github.com/siderolabs/talos/commit/ebeef28525f71189727200115d62fe8d713d1d07) feat: implement local caching dns server
* [`4a3691a27`](https://github.com/siderolabs/talos/commit/4a3691a2739871be5eff4b313c30d454a143fbc4) docs: fix broken links in metal-network-configuration.md
* [`c4ed189a6`](https://github.com/siderolabs/talos/commit/c4ed189a6912238350efd5f0181a6ef45728fc63) docs: provide sane defaults for each release series in vmware script
* [`8138d54c6`](https://github.com/siderolabs/talos/commit/8138d54c6c9bae4255216007595fa302bc418c1a) docs: clarify node taints/labels for worker nodes
* [`b44551ccd`](https://github.com/siderolabs/talos/commit/b44551ccdb0dd0ceaffd2e484c86ce91b25fe841) feat: update Linux to 6.6.13
* [`385707c5f`](https://github.com/siderolabs/talos/commit/385707c5f39e733c8f27532435cd14f5f2ff067d) docs: update vmware.sh
* [`d1a79b845`](https://github.com/siderolabs/talos/commit/d1a79b845f025defafb468fb6b5e86957cfad4fc) docs: fix small typo in etcd maintenance guide
* [`cf0603330`](https://github.com/siderolabs/talos/commit/cf0603330a5c852163642a6b3844d1dcc3892cf6) docs: copy generated JSON schema to host
* [`f11139c22`](https://github.com/siderolabs/talos/commit/f11139c229765cf82cadc84e6fa81d860005100b) docs: document local path provisioner install
</p>
</details>

### Dependency Changes

* **github.com/google/go-containerregistry**     v0.18.0 -> v0.19.1
* **github.com/prometheus/client_golang**        v1.18.0 -> v1.19.0
* **github.com/siderolabs/gen**                  v0.4.7 -> v0.4.8
* **github.com/siderolabs/go-debug**             v0.2.3 -> v0.3.0
* **github.com/siderolabs/talos**                e0dfbb8fba3c -> 145f2406307e
* **github.com/siderolabs/talos/pkg/machinery**  e0dfbb8fba3c -> 145f2406307e
* **github.com/sigstore/cosign/v2**              v2.2.2 -> v2.2.3
* **github.com/sigstore/sigstore**               v1.8.1 -> v1.8.3
* **github.com/stretchr/testify**                v1.8.4 -> v1.9.0
* **github.com/u-root/u-root**                   v0.12.0 -> v0.14.0
* **github.com/ulikunitz/xz**                    v0.5.11 -> v0.5.12
* **go.uber.org/zap**                            v1.26.0 -> v1.27.0
* **golang.org/x/net**                           v0.20.0 -> v0.23.0
* **golang.org/x/sys**                           v0.16.0 -> v0.18.0

Previous release can be found at [v0.2.2](https://github.com/siderolabs/image-factory/releases/tag/v0.2.2)

## [image-factory 0.3.0](https://github.com/siderolabs/image-factory/releases/tag/v0.3.0) (2024-04-05)

Welcome to the v0.3.0 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Noel Georgi
* Dmitriy Matrenichev
* Utku Ozdemir
* Dmitry Sharshakov
* Spencer Smith
* Artem Chernyshev
* Justin Garrison
* Mattias Cockburn
* Andrei Kvapil
* AvnarJakob
* Christian Mohn
* Christian WALDBILLIG
* Dmitry Sharshakov
* Evan Johnson
* Fabiano Fidêncio
* Henno Schooljan
* Jean-Tiare Le Bigot
* Kai Hanssen
* Louis SCHNEIDER
* Matthieu S
* Michael Stephenson
* Niklas Wik
* Pip Oomen
* Saiyam Pathak
* Sebastiaan Gerritsen
* Steve Francis
* bri
* ebcrypto
* edwinavalos
* fazledyn-or
* goodmost
* james-dreebot
* pardomue
* shurkys
* stereobutter

### Changes
<details><summary>10 commits</summary>
<p>

* [`7062392`](https://github.com/siderolabs/image-factory/commit/70623924c4a872b6cf7cdf08221350263f93c123) chore: update Talos dependency to 1.7.0-beta.0
* [`78f8944`](https://github.com/siderolabs/image-factory/commit/78f8944cbb8e673e0726250308b72eaf562d6290) feat: add cert issuer regexp option
* [`c0981e8`](https://github.com/siderolabs/image-factory/commit/c0981e849d2146313dd179b9174b7686f5c27846) feat: add support for -insecure-schematic-service-repository flag
* [`5d779bb`](https://github.com/siderolabs/image-factory/commit/5d779bb38adcc2a9dcd526683d8ea77eb94b0388) chore: bump dependencies
* [`93eb7de`](https://github.com/siderolabs/image-factory/commit/93eb7de1f6432ac31d34f5cccbf9ff40587e65bc) feat: support overlay
* [`df3d211`](https://github.com/siderolabs/image-factory/commit/df3d2119e49a4c6e09c8a4261e1bd679ab408a23) release(v0.2.3): prepare release
* [`4ccf0e5`](https://github.com/siderolabs/image-factory/commit/4ccf0e5d7ed44e39d97ab45040cca6665618f4fa) fix: ignore missing DTB and other SBC artifacts
* [`c7dba02`](https://github.com/siderolabs/image-factory/commit/c7dba02d17b068e576de7c155d5a5e58fa156a76) chore: run tailwindcss before creating image
* [`81f2cb4`](https://github.com/siderolabs/image-factory/commit/81f2cb437f71e4cb2d92db71a6f2a2b7becb8b56) chore: bump dependencies, rekres
* [`07095cd`](https://github.com/siderolabs/image-factory/commit/07095cd4966ab8943d93490bd5a9bc5085bec2f8) chore: re-enable govulncheck
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`238baf9`](https://github.com/siderolabs/gen/commit/238baf95e228d40f9f5b765b346688c704052715) chore: add typesafe `SyncMap` and bump stuff
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>1 commit</summary>
<p>

* [`0c2be80`](https://github.com/siderolabs/go-debug/commit/0c2be80d9d60034f3352a34841b615ef7bb0a62c) chore: run rekres (update to Go 1.22)
</p>
</details>

### Changes from siderolabs/talos
<details><summary>149 commits</summary>
<p>

* [`78f971370`](https://github.com/siderolabs/talos/commit/78f97137018be65ee02bd7fa2fe975ea61ccdaee) release(v1.7.0-beta.0): prepare release
* [`01d8b897c`](https://github.com/siderolabs/talos/commit/01d8b897c4f734299b9509f404493b0cdccb8789) fix: make safeReset truly safe to call multiple times
* [`653f838b0`](https://github.com/siderolabs/talos/commit/653f838b09ca7e6ac5098016eef2a03d55a6a334) feat: support multiple Docker cluster in talosctl cluster create
* [`951904554`](https://github.com/siderolabs/talos/commit/951904554ee8a464794cc2f9a9fb6bc084791c06) chore: bump dependencies (go 1.22.2)
* [`862c76001`](https://github.com/siderolabs/talos/commit/862c76001b6b1c7c98910ed54e9198e60f381985) feat: add support for CoreDNS forwarding to host DNS
* [`e8ae5ef63`](https://github.com/siderolabs/talos/commit/e8ae5ef63af13a40bd88bab03c54703eccdbfcc7) feat: add akamai platform support
* [`5c0f74b37`](https://github.com/siderolabs/talos/commit/5c0f74b377757ffee8f455ac2bcd9d81e3bf7717) fix: don't announce the VIP on acquire failure
* [`2f0fe10d5`](https://github.com/siderolabs/talos/commit/2f0fe10d557960a3dacd73922d4f0dc3e65614c5) chore: update sbc docs
* [`1b17008e9`](https://github.com/siderolabs/talos/commit/1b17008e9df8fe2988d0ad31a4f1604eba783fe3) fix: handle more OpenStack link types
* [`e7d804140`](https://github.com/siderolabs/talos/commit/e7d80414041a6911181c80d2089f0fed6e9640e6) fix: always update firewall rules (kubespan)
* [`78b9bd927`](https://github.com/siderolabs/talos/commit/78b9bd9273b1881d27c7ab364491dbb9c09d6df0) fix: report unsupported x86_64 microarchitecture level
* [`71d90ba5f`](https://github.com/siderolabs/talos/commit/71d90ba5f32c2e0679530121bd8b0d7db5285c8d) fix: retry in the fixed amount of time if grpc relay failed
* [`d320498a4`](https://github.com/siderolabs/talos/commit/d320498a44736e818e9a5f4b7a9d626cda028cd0) chore: bump dependencies
* [`3195e5d15`](https://github.com/siderolabs/talos/commit/3195e5d15cb6d1e4cc8251edf6cdcdc9f5601ed4) fix: force Flannel CNI to use KubePrism Kubernetes API endpoint
* [`917043fb5`](https://github.com/siderolabs/talos/commit/917043fb558aba95a1256523c11777d1e68fe3a9) chore: bump tools, pkgs and extra to stable
* [`f515741b5`](https://github.com/siderolabs/talos/commit/f515741b521281e832a706d7d1d5105cb627524d) chore: add equinix e2e-tests
* [`117e60583`](https://github.com/siderolabs/talos/commit/117e60583d4ec2e953174d49386fc4b1b80a7d0c) feat: add support for static extra fields for JSON logs
* [`090143b03`](https://github.com/siderolabs/talos/commit/090143b030df8387b6b44ea2fee525d493a1727c) fix: allow platform cmdline args to be platform-specific
* [`7a68504b6`](https://github.com/siderolabs/talos/commit/7a68504b6b451018e3a5e2589d03aa029b4216f6) feat: support rotating Kubernetes CA
* [`fac3dd043`](https://github.com/siderolabs/talos/commit/fac3dd04308b28a50338dcb077d399aac61adde2) fix: don't set default endpoints on gen config
* [`8dc4910c4`](https://github.com/siderolabs/talos/commit/8dc4910c48dbdb631f72512c1fb19a84b3ee1130) chore: enable "WG over GRPC" testing in siderolink agent tests
* [`bac366e43`](https://github.com/siderolabs/talos/commit/bac366e43e0c95e1d5089728e92daf8b5b8e410d) chore: add `ExtraInfo` field for extensions
* [`0fc24eeb0`](https://github.com/siderolabs/talos/commit/0fc24eeb09d3fafb5eee9df94c6bc3856b716ae0) feat: provide insecure flag to imager
* [`a6b2f5456`](https://github.com/siderolabs/talos/commit/a6b2f545648a570d06c6f840bb688e3176f28c6e) feat: update Kubernetes to 1.30.0-rc.0, etcd to 3.5.13
* [`0361ff895`](https://github.com/siderolabs/talos/commit/0361ff89560556c8f3f3039afdda78d2e4c79794) docs: quickstart video and brew install
* [`b752a8618`](https://github.com/siderolabs/talos/commit/b752a86183c990848f05d86fa1b41942b4f1610c) chore: talosctl: add openSUSE OVMF paths
* [`945648914`](https://github.com/siderolabs/talos/commit/94564891475273ca3dbaccdf32660567d8e8f3fd) feat: support hardware watchdog timers
* [`949ad11a2`](https://github.com/siderolabs/talos/commit/949ad11a2d6374c518ed50a628a1f069a05345f3) chore: import siderolink as `siderolink-launch` subcommand
* [`ee51f04af`](https://github.com/siderolabs/talos/commit/ee51f04af33ddaf7aa673225761b55599d1b7252) chore: azure e2e
* [`55dd41c0d`](https://github.com/siderolabs/talos/commit/55dd41c0dfd837e8ad20b924ffe4be4b1fcf5ec7) chore: update coredns to v1.11.2 in required section
* [`8eacc4ba8`](https://github.com/siderolabs/talos/commit/8eacc4ba8024abba834af811d1413f267f588219) feat: support rotation of Talos API CA
* [`92808e3bc`](https://github.com/siderolabs/talos/commit/92808e3bcff2fbbabf4cfd4c8f48acc0f25ef4e4) feat: report Docker node resources in cluster show
* [`84ec8c16f`](https://github.com/siderolabs/talos/commit/84ec8c16f30d2619ae85804df0601f6d92464a08) feat: support syncing to PTP clocks
* [`7d43c9aa6`](https://github.com/siderolabs/talos/commit/7d43c9aa6b7730fab8c749ef275ee3ad30a7de50) chore: annotate installer errors
* [`f737e6495`](https://github.com/siderolabs/talos/commit/f737e6495cda3588e3c71e7ee3e65823b54b9014) fix: populate routes to BGP neighbors (Equinix Metal)
* [`19f15a840`](https://github.com/siderolabs/talos/commit/19f15a840ccc5117f8729aef32449e2fb331340e) chore: bump golangci-lint to 1.57.0
* [`684011963`](https://github.com/siderolabs/talos/commit/6840119632e2b1869a30e69cfc57fd852824dbeb) docs: add docs for overlays
* [`9b6ec5929`](https://github.com/siderolabs/talos/commit/9b6ec5929a6d26ea936e7918076f2beba8d355d8) chore: bump kernel
* [`69f0466cd`](https://github.com/siderolabs/talos/commit/69f0466cd8e3a102c2e0eb4f742b324dea2055b0) docs: remove repetitive words
* [`113fb646e`](https://github.com/siderolabs/talos/commit/113fb646ec14c840c892666ebac5df3350a6a40d) chore: use `go-talos-support` library
* [`89fc68b45`](https://github.com/siderolabs/talos/commit/89fc68b4595a075753e6ed3fb52b34955611e9e0) fix: service lifecycle issues
* [`ead37abf0`](https://github.com/siderolabs/talos/commit/ead37abf097ad91cc8d29959db1f17e661edacf6) test: disable volume tests
* [`c64523a7a`](https://github.com/siderolabs/talos/commit/c64523a7a136469af7c6d39cb61f28c86678f74c) feat: update Flannel to v0.24.4
* [`15beb1478`](https://github.com/siderolabs/talos/commit/15beb147804b17060f20f4284b16213746370df8) feat: implement blockdevice watch controller
* [`06e3bc0cb`](https://github.com/siderolabs/talos/commit/06e3bc0cbd82a823963a130df72d69dffa52def7) feat: implement Siderolink wireguard over GRPC
* [`9afa70baf`](https://github.com/siderolabs/talos/commit/9afa70baf3b4f6b46705e5f1c5d25ad0c383f596) fix: patch correctly config in `talosctl upgrade-k8s`
* [`3130caf95`](https://github.com/siderolabs/talos/commit/3130caf95444318f38ba7a4a885d021868d37827) chore: re-enable DRBD extension
* [`3ba180d07`](https://github.com/siderolabs/talos/commit/3ba180d07d494032e188b401f1a0d87e8549e293) release(v1.7.0-alpha.1): prepare release
* [`403ad93c3`](https://github.com/siderolabs/talos/commit/403ad93c35b4cee9c012addb4667cb04e23e1c61) feat: update dependencies
* [`7376f34e8`](https://github.com/siderolabs/talos/commit/7376f34e823f6399ed2c66ae1296a8a47a0a00ef) fix: remove maintenance config when maintenance service is shut down
* [`952801d8b`](https://github.com/siderolabs/talos/commit/952801d8b2af27a49531b8a19f8b74400b6d4eb8) fix: handle overlay partition options
* [`465b9a4e6`](https://github.com/siderolabs/talos/commit/465b9a4e6ca9367326cb862b501f1146989b07d4) fix: update discovery client with the fix for keepalive interval
* [`1e9f866ac`](https://github.com/siderolabs/talos/commit/1e9f866aca14ec5ecc4d5619f42e02d44b6968d1) feat: update Kubernetes to v1.30.0-beta.0
* [`d118a852b`](https://github.com/siderolabs/talos/commit/d118a852b995f13fc5160acb7c95d2186adaac41) feat: implement `Install` for imager overlays
* [`cd5a5a447`](https://github.com/siderolabs/talos/commit/cd5a5a4474914cb64a23698b6656763b253a4d01) chore: migrate to go-grpc-middleware/v2
* [`e3c2a6398`](https://github.com/siderolabs/talos/commit/e3c2a639810ad325c2b5d1b1a92aa09d52ac6997) feat: set default NTP server to time.cloudflare.com
* [`32e087760`](https://github.com/siderolabs/talos/commit/32e08776078f9ca78ed27a382665589229c0ccb4) chore: print all available logs containers in `logs` command completions
* [`e89d755c5`](https://github.com/siderolabs/talos/commit/e89d755c523065a257d34dff9a88df97fc1908b3) fix: etcd config validation for worker
* [`1aa3c9182`](https://github.com/siderolabs/talos/commit/1aa3c91821fb9889e9859c880d602457791f6a14) docs: add DreeBot to ADOPTERS.md
* [`1bb6027cc`](https://github.com/siderolabs/talos/commit/1bb6027ccd7c63ae3a012eb310d1e05027ec1f80) fix: fix nil panic on maintenance upgrade with partial config
* [`aa70bfb9d`](https://github.com/siderolabs/talos/commit/aa70bfb9dc4fc886a6c5b771947a146ee2f58ef7) docs: add Redpill Linpro to adopters list
* [`f02aeec92`](https://github.com/siderolabs/talos/commit/f02aeec922b6327dad6d4fee917987b147abbf2a) fix: do not fail cluster create when input dir does not contain talosconfig
* [`1ec6683e0`](https://github.com/siderolabs/talos/commit/1ec6683e0c1d60b55a25e495c2dfc18f5bbf05b0) chore: use go-copy
* [`3c8f51d70`](https://github.com/siderolabs/talos/commit/3c8f51d707b897fb34ed3a9f7c32b7cd3e5ee5b0) chore: move cli formatters and version modules to machinery
* [`8152a6dd6`](https://github.com/siderolabs/talos/commit/8152a6dd6b7484e3f313b7cc9dd84fefba84d106) feat: update Go to 1.22.1
* [`8c7953991`](https://github.com/siderolabs/talos/commit/8c79539914324eee64dbdaf1f535fc4e20da55e8) docs: update replicated-local-storage-with-openebs-jiva.md
* [`f23bd8144`](https://github.com/siderolabs/talos/commit/f23bd81448b640b37006d6bfffa9315f84cad492) fix: syslog parser
* [`bbed07e03`](https://github.com/siderolabs/talos/commit/bbed07e03a815869cbae5aaa2667864697fd5d65) feat: update Linux to 6.6.18
* [`8125e754b`](https://github.com/siderolabs/talos/commit/8125e754b8a4c8db891dcd2dbd6ee3702daa2393) feat: imager overlay
* [`0b9b4da12`](https://github.com/siderolabs/talos/commit/0b9b4da12abe6bf19d9eaaa48b42cd1a794ca8fa) feat: update Kubernetes to 1.30.0-alpha.3
* [`3a764029e`](https://github.com/siderolabs/talos/commit/3a764029ea2d3f888c2d4d83ebffd6f97a46e3a9) docs: fix typo in word governor
* [`d81d49000`](https://github.com/siderolabs/talos/commit/d81d4900030e93cacda34646732f24816dd3d85f) chore: update CoreDNS renovate source
* [`b2ad5dc5f`](https://github.com/siderolabs/talos/commit/b2ad5dc5f809da9665b41c25d9ab6359a87ec942) fix: workaround a race in CNI setup (talosctl cluster create)
* [`457507803`](https://github.com/siderolabs/talos/commit/457507803d302a31b47f5e386ce1e398861550bd) fix: provide auth when pulling images in the imager
* [`e707175ab`](https://github.com/siderolabs/talos/commit/e707175ab5bdeb0f79ad242e2c81f36eec928342) docs: update config patch in cilium docs
* [`f8c556a1c`](https://github.com/siderolabs/talos/commit/f8c556a1ce9aa49c1af1bfe97c3694c00fcc67bc) chore: listen for dns requests on 127.0.0.53
* [`8872a7a21`](https://github.com/siderolabs/talos/commit/8872a7a2105034d8d6550e628355fe5f09131691) fix: ignore 'no such device' in addition to 'no such file'
* [`1cb544353`](https://github.com/siderolabs/talos/commit/1cb5443530abc2f6333566ec8e8429b2a784f791) chore: uki der certs in iso
* [`67ac6933d`](https://github.com/siderolabs/talos/commit/67ac6933d3c23b8ea31f01bd45d0192573e64ef3) fix: handle errors to watch apid/trustd certs
* [`c79d69c2e`](https://github.com/siderolabs/talos/commit/c79d69c2e25ee588f45a8978117300c31871f749) fix: only set gateway if set in context (opennebula)
* [`4575dd8e7`](https://github.com/siderolabs/talos/commit/4575dd8e741e99ab92ac63afdf48d816562f744c) chore: allow not preallocated disks for QEMU cluster
* [`0bddfea81`](https://github.com/siderolabs/talos/commit/0bddfea818994288285f442c27a339e6d1dc6cf0) chore: add oceanbox.io to adopters
* [`136427592`](https://github.com/siderolabs/talos/commit/1364275926df312204e006751dacc7af8e7d6726) chore: use proper `talos_version_contract` for TF tests
* [`6bf50fdc1`](https://github.com/siderolabs/talos/commit/6bf50fdc14ad97d97fd8fcec3132f0b183c93e5a) chore: disable x/net/trace in gRPC to enable dead code elimination
* [`815a8e9cc`](https://github.com/siderolabs/talos/commit/815a8e9cc5ad2c22acf11f223d8a64abbbf4b3cb) feat: add partial config support to `talosctl cluster create`
* [`64e9703f8`](https://github.com/siderolabs/talos/commit/64e9703f8648f997ff2e2e0fff932f74fd52d585) chore: add tests for the Kata Containers extension
* [`9b6291925`](https://github.com/siderolabs/talos/commit/9b62919253f16cbbfec999da26f11e8751fbb345) feat: update pkgs
* [`66f3ffdd4`](https://github.com/siderolabs/talos/commit/66f3ffdd4ad69ec690c680868cc95697eb1fba48) fix: ensure that Talos runs in a pod (container)
* [`9dbc33972`](https://github.com/siderolabs/talos/commit/9dbc33972a2ded3818fabd9b157604d26926e3c9) feat: add basic syslog implementation
* [`0b7a27e6a`](https://github.com/siderolabs/talos/commit/0b7a27e6a122e7cacb5ff82a7f6cae005435ae54) feat: allow access to all resources over siderolink in maintenance mode
* [`53721883d`](https://github.com/siderolabs/talos/commit/53721883d50bd9979edeb4f94a0f1cfcf74d4d80) feat: support AWS KMS for the SecureBoot signing
* [`7ee999f8a`](https://github.com/siderolabs/talos/commit/7ee999f8a3906eda23b7657da4c4212886a81626) fix: disable KubeSpan endpoint harvesting by default
* [`7b87c7fe9`](https://github.com/siderolabs/talos/commit/7b87c7fe97d01f33eb621bb631d482f975da3feb) chore: bump Go dependencies
* [`8e9596d3c`](https://github.com/siderolabs/talos/commit/8e9596d3c65246824e921f6cb9dfcda96b5ff52c) docs: rpi talosctl install update
* [`493bb60f8`](https://github.com/siderolabs/talos/commit/493bb60f81075181c4f71af546674871f4616067) fix: correctly handle partial configs in `DNSUpstreamController`
* [`6deb10ae2`](https://github.com/siderolabs/talos/commit/6deb10ae25efa1d96dd7416045c99b178b04e020) chore: deprecate `environmentFile` for extensions
* [`f8b4ee82a`](https://github.com/siderolabs/talos/commit/f8b4ee82aeba990d8e34b7c95debf30c4a626298) chore: update extensions test
* [`1366ce14a`](https://github.com/siderolabs/talos/commit/1366ce14a8b0bf72ac884147497e354fb33ef3fa) feat: update Kubernetes to v1.30.0-alpha.2
* [`559308ef7`](https://github.com/siderolabs/talos/commit/559308ef7e482786cc3554002bcd9fb05e0459c8) fix: use MachineStatus resource to check for boot done
* [`15e8bca2b`](https://github.com/siderolabs/talos/commit/15e8bca2b2f839ee138faa14cb3931af173d258f) feat: support environment in `ExtensionServicesConfig`
* [`3fe82ec46`](https://github.com/siderolabs/talos/commit/3fe82ec461995b680ecf060af75b47cd175a6342) feat: custom image settings for k8s upgrade
* [`fa3b93370`](https://github.com/siderolabs/talos/commit/fa3b93370501009283e110b74876b18ce6bad4f9) chore: replace fmt.Errorf with errors.New where possible
* [`d4521ee9c`](https://github.com/siderolabs/talos/commit/d4521ee9c472622fb2ef3c8570c1fa1c46332c16) feat: update kernel with sfc driver and LSM updates
* [`2f0421b40`](https://github.com/siderolabs/talos/commit/2f0421b406ee252e9197c0b4589c0b33662bef34) fix: run xfs_repair on invalid argument error
* [`f868fb8e8`](https://github.com/siderolabs/talos/commit/f868fb8e8f50e1acaa1743001d5b4f702bf29294) docs: update vmware tools url
* [`fa2d34dd8`](https://github.com/siderolabs/talos/commit/fa2d34dd8875e6a09c257acfb9321c1230658b87) chore: enable v6 support on the same port
* [`83e0b0c19`](https://github.com/siderolabs/talos/commit/83e0b0c19aaca7d413483b3a908c9dc3b4289203) chore: adjust dns sockets settings
* [`a1ec1705b`](https://github.com/siderolabs/talos/commit/a1ec1705bc5d1f7c66dbb8549af42fc3b4778400) chore: update Go to 1.22.0
* [`76b50fcd4`](https://github.com/siderolabs/talos/commit/76b50fcd4ae2a5d602997cc360c9dcb45e4243e8) chore: add Ænix to the Adopters list
* [`5324d3916`](https://github.com/siderolabs/talos/commit/5324d391671dfbf918aee1bd6b095adffadecf8e) chore: bump stuff
* [`087b50f42`](https://github.com/siderolabs/talos/commit/087b50f42932e4da883de254984bce4ad7858b90) feat: support systemd-boot ISO enroll keys option
* [`afa71d6b0`](https://github.com/siderolabs/talos/commit/afa71d6b028c33333db51495a3db41b758f38435) chore: use "handle-like" resource in `DNSResolveCacheController`
* [`013e13070`](https://github.com/siderolabs/talos/commit/013e130702758dcd8f44c84de8090d624aa5c7b9) fix: error with decoding config document with wrong apiVersion
* [`1e77bb1c3`](https://github.com/siderolabs/talos/commit/1e77bb1c3dde3c6a54bc4174eafc09846ff59e62) chore: allow custom pkgs to build talos
* [`3f8a85f1b`](https://github.com/siderolabs/talos/commit/3f8a85f1b390936cf7d76a146f6b76973be1e474) fix: unlock the upgrade mutex properly
* [`61c3331b1`](https://github.com/siderolabs/talos/commit/61c3331b148901a3137de6a087d561a6db8f4dfc) docs: update indentation in vip.md
* [`383e528df`](https://github.com/siderolabs/talos/commit/383e528df8c52ad44402c830fb3611b66c71fc7a) chore: allow uuid-based hostnames in talosctl cluster create
* [`1e6c8c4de`](https://github.com/siderolabs/talos/commit/1e6c8c4dec1e71f0d83914c3a0d7b907b21dc3b0) feat: extensions services config
* [`989ca3ade`](https://github.com/siderolabs/talos/commit/989ca3ade194bb0cd5c162d5d8973c133e381501) feat: add OpenNebula platform support
* [`914f88778`](https://github.com/siderolabs/talos/commit/914f88778838abe51f24ec3a9574e91836561e9e) docs: update nocloud.md Proxmox information
* [`a04cc8015`](https://github.com/siderolabs/talos/commit/a04cc80154ed94e970615714fd8dff9cd8cf8ca9) fix: pass TTL when generating client certificate
* [`3fe8c12ca`](https://github.com/siderolabs/talos/commit/3fe8c12ca654790695417b3d4f6bb5517e5902b5) fix: add log line about controller runtime failing
* [`ddbabc7e5`](https://github.com/siderolabs/talos/commit/ddbabc7e58e476c95d7bb15f325f612a3d8fc86c) fix: use a separate cgroup for each extension service
* [`6ccdd2c09`](https://github.com/siderolabs/talos/commit/6ccdd2c09c88eb2fe8b5b382dbd94816865381d3) chore: fix markdown-lint call
* [`4184e617a`](https://github.com/siderolabs/talos/commit/4184e617ab92b8f41c2540bf55aa4d502778dcad) chore: add test for wasmedge runtime extension
* [`95ea3a6c6`](https://github.com/siderolabs/talos/commit/95ea3a6c65a952fef533016b7116212c21609aac) chore: bump timeout in acquire tests
* [`c19a505d8`](https://github.com/siderolabs/talos/commit/c19a505d8cde234e12f729183e8c7272ac049159) chore: bump docker dind image
* [`d7d4154d5`](https://github.com/siderolabs/talos/commit/d7d4154d5dc817f91771b25b358825dae803de7f) chore: remove channel blocking in qemu launch
* [`029d7f7b9`](https://github.com/siderolabs/talos/commit/029d7f7b9b2ba610b9bd68dd00a9d8a060bfd280) release(v1.7.0-alpha.0): prepare release
* [`2ff81c06b`](https://github.com/siderolabs/talos/commit/2ff81c06bc1123af2fa7286fff15d9de0b8a868a) feat: update runc 1.1.12, containerd 1.7.13
* [`9d8cd4d05`](https://github.com/siderolabs/talos/commit/9d8cd4d058e73d30e4864e67377cf55390467725) chore: drop deprecated method EtcdRemoveMember
* [`17567f19b`](https://github.com/siderolabs/talos/commit/17567f19be39eeaf0d9a9aa3cd773b73d537814a) fix: take into account the moment seen when cleaning up CRI images
* [`aa03204b8`](https://github.com/siderolabs/talos/commit/aa03204b864d8d8ac5a7ee4986a06230863043fb) docs: document the process of building custom kernel packages
* [`7af48bd55`](https://github.com/siderolabs/talos/commit/7af48bd5598e61357cdb9b31dd57de6479b1ce7c) feat: use RSA key for kube-apiserver service account key
* [`a5e13c696`](https://github.com/siderolabs/talos/commit/a5e13c696d1e1cb8e894a4133791c74470687553) fix: retry blockdevice open in the installer
* [`593afeea3`](https://github.com/siderolabs/talos/commit/593afeea38a75de01041e3126cb0ad3443f6e1a1) fix: run the interactive installer loop to report errors
* [`87be76b87`](https://github.com/siderolabs/talos/commit/87be76b8788d179058be14c53e1092054b08c5dd) fix: be more tolerant to error handling in Mounts API
* [`03add7503`](https://github.com/siderolabs/talos/commit/03add750309dcdeb7c2b87cd72da29a3e228e56e) docs: add section on using imager with extensions from tarball
* [`ee0fb5eff`](https://github.com/siderolabs/talos/commit/ee0fb5effce82fec99860b5910e0fb6e5147b49b) docs: consolidate certificate management articles
* [`9c14dea20`](https://github.com/siderolabs/talos/commit/9c14dea209bba69b471fd43eb2e8ba05de3ff549) chore: bump coredns
* [`ebeef2852`](https://github.com/siderolabs/talos/commit/ebeef28525f71189727200115d62fe8d713d1d07) feat: implement local caching dns server
* [`4a3691a27`](https://github.com/siderolabs/talos/commit/4a3691a2739871be5eff4b313c30d454a143fbc4) docs: fix broken links in metal-network-configuration.md
* [`c4ed189a6`](https://github.com/siderolabs/talos/commit/c4ed189a6912238350efd5f0181a6ef45728fc63) docs: provide sane defaults for each release series in vmware script
* [`8138d54c6`](https://github.com/siderolabs/talos/commit/8138d54c6c9bae4255216007595fa302bc418c1a) docs: clarify node taints/labels for worker nodes
* [`b44551ccd`](https://github.com/siderolabs/talos/commit/b44551ccdb0dd0ceaffd2e484c86ce91b25fe841) feat: update Linux to 6.6.13
* [`385707c5f`](https://github.com/siderolabs/talos/commit/385707c5f39e733c8f27532435cd14f5f2ff067d) docs: update vmware.sh
* [`d1a79b845`](https://github.com/siderolabs/talos/commit/d1a79b845f025defafb468fb6b5e86957cfad4fc) docs: fix small typo in etcd maintenance guide
* [`cf0603330`](https://github.com/siderolabs/talos/commit/cf0603330a5c852163642a6b3844d1dcc3892cf6) docs: copy generated JSON schema to host
* [`f11139c22`](https://github.com/siderolabs/talos/commit/f11139c229765cf82cadc84e6fa81d860005100b) docs: document local path provisioner install
</p>
</details>

### Dependency Changes

* **github.com/google/go-containerregistry**     v0.18.0 -> v0.19.1
* **github.com/prometheus/client_golang**        v1.18.0 -> v1.19.0
* **github.com/siderolabs/gen**                  v0.4.7 -> v0.4.8
* **github.com/siderolabs/go-debug**             v0.2.3 -> v0.3.0
* **github.com/siderolabs/talos**                e0dfbb8fba3c -> v1.7.0-beta.0
* **github.com/siderolabs/talos/pkg/machinery**  e0dfbb8fba3c -> v1.7.0-beta.0
* **github.com/sigstore/cosign/v2**              v2.2.2 -> v2.2.3
* **github.com/sigstore/sigstore**               v1.8.1 -> v1.8.3
* **github.com/stretchr/testify**                v1.8.4 -> v1.9.0
* **github.com/u-root/u-root**                   v0.12.0 -> v0.14.0
* **github.com/ulikunitz/xz**                    v0.5.11 -> v0.5.12
* **go.uber.org/zap**                            v1.26.0 -> v1.27.0
* **golang.org/x/net**                           v0.20.0 -> v0.23.0
* **golang.org/x/sys**                           v0.16.0 -> v0.18.0

Previous release can be found at [v0.2.2](https://github.com/siderolabs/image-factory/releases/tag/v0.2.2)

## [image-factory 0.2.3](https://github.com/siderolabs/image-factory/releases/tag/v0.2.3) (2024-03-14)

Welcome to the v0.2.3 release of image-factory!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/image-factory/issues.

### Contributors

* Andrey Smirnov
* Dmitriy Matrenichev
* Spencer Smith
* Christian Mohn
* Noel Georgi
* Steve Francis
* Utku Ozdemir
* edwinavalos
* stereobutter

### Changes
<details><summary>4 commits</summary>
<p>

* [`4ccf0e5`](https://github.com/siderolabs/image-factory/commit/4ccf0e5d7ed44e39d97ab45040cca6665618f4fa) fix: ignore missing DTB and other SBC artifacts
* [`c7dba02`](https://github.com/siderolabs/image-factory/commit/c7dba02d17b068e576de7c155d5a5e58fa156a76) chore: run tailwindcss before creating image
* [`81f2cb4`](https://github.com/siderolabs/image-factory/commit/81f2cb437f71e4cb2d92db71a6f2a2b7becb8b56) chore: bump dependencies, rekres
* [`07095cd`](https://github.com/siderolabs/image-factory/commit/07095cd4966ab8943d93490bd5a9bc5085bec2f8) chore: re-enable govulncheck
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>1 commit</summary>
<p>

* [`0c2be80`](https://github.com/siderolabs/go-debug/commit/0c2be80d9d60034f3352a34841b615ef7bb0a62c) chore: run rekres (update to Go 1.22)
</p>
</details>

### Changes from siderolabs/talos
<details><summary>21 commits</summary>
<p>

* [`029d7f7b9`](https://github.com/siderolabs/talos/commit/029d7f7b9b2ba610b9bd68dd00a9d8a060bfd280) release(v1.7.0-alpha.0): prepare release
* [`2ff81c06b`](https://github.com/siderolabs/talos/commit/2ff81c06bc1123af2fa7286fff15d9de0b8a868a) feat: update runc 1.1.12, containerd 1.7.13
* [`9d8cd4d05`](https://github.com/siderolabs/talos/commit/9d8cd4d058e73d30e4864e67377cf55390467725) chore: drop deprecated method EtcdRemoveMember
* [`17567f19b`](https://github.com/siderolabs/talos/commit/17567f19be39eeaf0d9a9aa3cd773b73d537814a) fix: take into account the moment seen when cleaning up CRI images
* [`aa03204b8`](https://github.com/siderolabs/talos/commit/aa03204b864d8d8ac5a7ee4986a06230863043fb) docs: document the process of building custom kernel packages
* [`7af48bd55`](https://github.com/siderolabs/talos/commit/7af48bd5598e61357cdb9b31dd57de6479b1ce7c) feat: use RSA key for kube-apiserver service account key
* [`a5e13c696`](https://github.com/siderolabs/talos/commit/a5e13c696d1e1cb8e894a4133791c74470687553) fix: retry blockdevice open in the installer
* [`593afeea3`](https://github.com/siderolabs/talos/commit/593afeea38a75de01041e3126cb0ad3443f6e1a1) fix: run the interactive installer loop to report errors
* [`87be76b87`](https://github.com/siderolabs/talos/commit/87be76b8788d179058be14c53e1092054b08c5dd) fix: be more tolerant to error handling in Mounts API
* [`03add7503`](https://github.com/siderolabs/talos/commit/03add750309dcdeb7c2b87cd72da29a3e228e56e) docs: add section on using imager with extensions from tarball
* [`ee0fb5eff`](https://github.com/siderolabs/talos/commit/ee0fb5effce82fec99860b5910e0fb6e5147b49b) docs: consolidate certificate management articles
* [`9c14dea20`](https://github.com/siderolabs/talos/commit/9c14dea209bba69b471fd43eb2e8ba05de3ff549) chore: bump coredns
* [`ebeef2852`](https://github.com/siderolabs/talos/commit/ebeef28525f71189727200115d62fe8d713d1d07) feat: implement local caching dns server
* [`4a3691a27`](https://github.com/siderolabs/talos/commit/4a3691a2739871be5eff4b313c30d454a143fbc4) docs: fix broken links in metal-network-configuration.md
* [`c4ed189a6`](https://github.com/siderolabs/talos/commit/c4ed189a6912238350efd5f0181a6ef45728fc63) docs: provide sane defaults for each release series in vmware script
* [`8138d54c6`](https://github.com/siderolabs/talos/commit/8138d54c6c9bae4255216007595fa302bc418c1a) docs: clarify node taints/labels for worker nodes
* [`b44551ccd`](https://github.com/siderolabs/talos/commit/b44551ccdb0dd0ceaffd2e484c86ce91b25fe841) feat: update Linux to 6.6.13
* [`385707c5f`](https://github.com/siderolabs/talos/commit/385707c5f39e733c8f27532435cd14f5f2ff067d) docs: update vmware.sh
* [`d1a79b845`](https://github.com/siderolabs/talos/commit/d1a79b845f025defafb468fb6b5e86957cfad4fc) docs: fix small typo in etcd maintenance guide
* [`cf0603330`](https://github.com/siderolabs/talos/commit/cf0603330a5c852163642a6b3844d1dcc3892cf6) docs: copy generated JSON schema to host
* [`f11139c22`](https://github.com/siderolabs/talos/commit/f11139c229765cf82cadc84e6fa81d860005100b) docs: document local path provisioner install
</p>
</details>

### Dependency Changes

* **github.com/google/go-containerregistry**     v0.18.0 -> v0.19.0
* **github.com/siderolabs/go-debug**             v0.2.3 -> v0.3.0
* **github.com/siderolabs/talos**                e0dfbb8fba3c -> v1.7.0-alpha.0
* **github.com/siderolabs/talos/pkg/machinery**  e0dfbb8fba3c -> v1.7.0-alpha.0
* **github.com/sigstore/cosign/v2**              v2.2.2 -> v2.2.3
* **github.com/u-root/u-root**                   v0.12.0 -> v0.13.1
* **go.uber.org/zap**                            v1.26.0 -> v1.27.0
* **golang.org/x/net**                           v0.20.0 -> v0.21.0
* **golang.org/x/sys**                           v0.16.0 -> v0.17.0

Previous release can be found at [v0.2.2](https://github.com/siderolabs/image-factory/releases/tag/v0.2.2)

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

