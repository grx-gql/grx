# Changelog

All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Published versions use section titles that match
[release-please](https://github.com/googleapis/release-please) (emoji-prefixed
headings such as **`### ✨ Added`**, **`### 🐛 Fixed`**, **`### 📚 Documentation`**, … —
see **`release-please-config.json`**). This file **`CHANGELOG.md`** is the changelog; browse it **[on GitHub](https://github.com/grx-gql/grx/blob/main/CHANGELOG.md)** (the docs site no longer duplicates it).

## [0.5.0](https://github.com/grx-gql/grx/compare/v0.4.2...v0.5.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* flatten module layout and migrate import paths under grx-gql/grx

### ✨ Added

* add configurable server HTTP options ([43c9335](https://github.com/grx-gql/grx/commit/43c9335df11625ba7372876243aae8d63e84a21f))
* add enum scalar interface union and default value support ([9c75e51](https://github.com/grx-gql/grx/commit/9c75e516246a4f9eda692eac8b12492756ec7906))
* add GraphQL fragments directives and SDL export ([26a5440](https://github.com/grx-gql/grx/commit/26a5440e2ecf75ebead896d31e7b62396ab2308e))
* add GraphQL HTTP client ([a6c22ac](https://github.com/grx-gql/grx/commit/a6c22ac82701c0b31cda60475573b1237ae884a0))
* add GraphQL HTTP client ([b227c4c](https://github.com/grx-gql/grx/commit/b227c4c4bcd0f8ce5d66b6f1413ea4d439a3a431))
* add GraphQL response locations extensions and ordered output ([6bfe5cd](https://github.com/grx-gql/grx/commit/6bfe5cdac57b5257aa25afb76394599b414d20eb))
* add GraphQL response locations extensions and ordered output ([9d3d6e9](https://github.com/grx-gql/grx/commit/9d3d6e9b51d11fbaf21d8371fc95d322c7bb74c4))
* add runtime observability options ([96f8ddb](https://github.com/grx-gql/grx/commit/96f8ddb65844abc750c634a823e334ceccace264))
* add schema coordinate resolution ([a63f909](https://github.com/grx-gql/grx/commit/a63f909f59ff712fbb1d710d5806deca75535e4a))
* clean up and more features implemented ([b7ba361](https://github.com/grx-gql/grx/commit/b7ba3615071665ae0ff5a679e3d496091fb4b7ce))
* complete GraphQL execution and introspection parity ([fd851bd](https://github.com/grx-gql/grx/commit/fd851bdd94ddde59577882d5f9a713e39d0d4e37)), closes [#8](https://github.com/grx-gql/grx/issues/8) [#9](https://github.com/grx-gql/grx/issues/9)
* complete GraphQL response format parity and sync roadmap ([6737b59](https://github.com/grx-gql/grx/commit/6737b599bfcae9ddb553a4c44fd3712d45bdd520)), closes [#11](https://github.com/grx-gql/grx/issues/11)
* complete GraphQL response format parity and sync roadmap ([558a887](https://github.com/grx-gql/grx/commit/558a8870cfc4f7d8cce6cc8f2f12ebb18747d0d7)), closes [#11](https://github.com/grx-gql/grx/issues/11)
* expand websocket transport controls ([5271f5a](https://github.com/grx-gql/grx/commit/5271f5a600ce6e949c3286b7c10c32d4eeb25d03))
* GraphQL execution and introspection parity ([2036cd0](https://github.com/grx-gql/grx/commit/2036cd0eee68e9bc6f3f9114bb11db5fec8179d8))
* GraphQL validation and response format parity ([aef5bd2](https://github.com/grx-gql/grx/commit/aef5bd2add8ffd1aa3dd482e1883fcf045d98747))
* implement GraphQL security controls ([e9d0f88](https://github.com/grx-gql/grx/commit/e9d0f883811df80d372fc6e9972e8bf911a2ee58))
* implement GraphQL security controls ([b9cd277](https://github.com/grx-gql/grx/commit/b9cd27749a9febf7ff9d884e16827a91503ac42f))
* implement SDL parser, type system metadata, and built-in directive parity ([7016c5a](https://github.com/grx-gql/grx/commit/7016c5a417c9222d28e9494891363167d3bd7157))
* split subscription example from basic server ([1c68e11](https://github.com/grx-gql/grx/commit/1c68e11c70cdaa4d65cf48b4d0fc42c3db047cc2))


### 🐛 Fixed

* align graphql execution with spec and preserve hot paths ([68e0e0d](https://github.com/grx-gql/grx/commit/68e0e0d1bb017a520d987ac9fe322de6455b899b))
* align graphql execution with spec and preserve hot paths ([a0f0385](https://github.com/grx-gql/grx/commit/a0f0385df31c950af8a6eec30b9a116e15d9515d))
* **ci:** gh workflows ([2d02fce](https://github.com/grx-gql/grx/commit/2d02fce887fff9fed7115217dd72cdd8b32644b2))
* **ci:** gh workflows ([5a5e2f1](https://github.com/grx-gql/grx/commit/5a5e2f1dc8b6be4a50d56d6c9bf8bfdcbd33e1c1))
* complete graphql validation parity ([d049abe](https://github.com/grx-gql/grx/commit/d049abe1dcdc1991f481ff7e370f055c00a149c1))
* create release tag during manual workflow dispatch ([3aefe6d](https://github.com/grx-gql/grx/commit/3aefe6dbb897bd00f9972bca46db5f858ca8c2bb))
* **docs:** drop dead docs/changelog link from changelog pages ([f4ab88a](https://github.com/grx-gql/grx/commit/f4ab88aa0f9277c306a5ca43ae823dc97d68f96e))
* format introspection defaultValue as SDL literal string ([205714e](https://github.com/grx-gql/grx/commit/205714e20ca8c7daf3cf506d3734eae06ec6bcee))
* format introspection defaultValue as SDL literal string ([7de42ba](https://github.com/grx-gql/grx/commit/7de42ba809a8cd4215a183f6f5ebd944231043d6))
* harden graphql validation and execution limit ([ccb4ff9](https://github.com/grx-gql/grx/commit/ccb4ff92ed9b8fe4a3bff339a6f2d7ba292b5bc8))
* prevent double-close race in Memory pubsub unsubscribe ([b1c48b0](https://github.com/grx-gql/grx/commit/b1c48b07d5513b921988197668b7545a75167d4e))
* release workflow ([324a339](https://github.com/grx-gql/grx/commit/324a339ca22bd06cedcb421b0ba2afcd3a7b4560))
* release workflow ([e21a354](https://github.com/grx-gql/grx/commit/e21a354b443effb287c5b92582644938bf9c865c))
* resolve env context unavailable in job-level if condition ([d53ee61](https://github.com/grx-gql/grx/commit/d53ee610e9a660179d7c80131e1ee662ece73393))
* test coverage ([4d47081](https://github.com/grx-gql/grx/commit/4d47081821e7bdb97508940b0873162fb6e66626))
* test coverage ([d86ebbe](https://github.com/grx-gql/grx/commit/d86ebbeacc44e5be1726c9dfa8305d5165616bf8))


### 📚 Documentation

* clean up ([6f81ace](https://github.com/grx-gql/grx/commit/6f81acea0d09050978ee45ab2339fa8acad7d232))
* clean up ([c2ba814](https://github.com/grx-gql/grx/commit/c2ba8144a5c80871581de92ba791ccfa1fe37635))
* collapse sidebar groups by default ([161e62c](https://github.com/grx-gql/grx/commit/161e62cd93f3703db4e3503ebefa846a828fe81b))
* collapse sidebar groups by default ([801da50](https://github.com/grx-gql/grx/commit/801da5047abdc8c9941d9c56a9ee39ef7b46a9f1))
* link changelog from GitHub, remove mirrored page ([233f28f](https://github.com/grx-gql/grx/commit/233f28fd7a05fbf00979f6ba953a12a3fccfd2c2))
* link changelog from GitHub, remove mirrored page ([7a01f2f](https://github.com/grx-gql/grx/commit/7a01f2f472181fb038376e0706a34d47a760c3af))
* migrate documentation site to VitePress ([8f5ec81](https://github.com/grx-gql/grx/commit/8f5ec81555da34fb16a977a7aebde43d3d3e3cd4))
* remove RELEASING.md ([af4bad1](https://github.com/grx-gql/grx/commit/af4bad10f3be4dc57b7e7fc24f4495d8a2f80eb8))
* remove RELEASING.md ([0a1d03f](https://github.com/grx-gql/grx/commit/0a1d03f95040f445b18103ddc8df870385b065ff))
* sync roadmap parity status ([b35d643](https://github.com/grx-gql/grx/commit/b35d643aa15242911c49f2f3e0db2e165353ca88))
* update roadmap ([890881e](https://github.com/grx-gql/grx/commit/890881e03a6cb93534c9f3534b4c6dd12e95ccab))


### 🧹 Chores

* add library coverage target ([4d73f36](https://github.com/grx-gql/grx/commit/4d73f36a852fa851b0e406db19e431fed7a62424))
* **changelog:** resync docs/changelog.md via sync-changelog script ([795c08a](https://github.com/grx-gql/grx/commit/795c08a08095345dc48d6f0438fc791c264e75c2))
* **ci:** clean up workflow ([895d7f5](https://github.com/grx-gql/grx/commit/895d7f5be22108c7c8d49d3f4bf0de6ccb09a309))
* **ci:** clean up workflow ([ac6bcac](https://github.com/grx-gql/grx/commit/ac6bcac630dedc4137b16f3df1d4733946796aca))
* **ci:** drop tag-verify job from Release workflow ([4762e51](https://github.com/grx-gql/grx/commit/4762e51d79765ececcdda511aed4ca30f2237e18))
* **ci:** drop tag-verify job from Release workflow ([bf64b11](https://github.com/grx-gql/grx/commit/bf64b110bdd684b3d870511b3ddcc955f6c21d68))
* **ci:** simplify release workflow ([1cdd693](https://github.com/grx-gql/grx/commit/1cdd69303503618480c5dced2da2566e41a7c16c))
* flatten module layout and migrate import paths under grx-gql/grx ([1726fe5](https://github.com/grx-gql/grx/commit/1726fe5ab51a1ae96ceb2d4ce5791b98e1ce9289))
* ignore basic example build output ([d30fa83](https://github.com/grx-gql/grx/commit/d30fa8335845cdd97b21a789800e55023a2f45f6))
* init project ([06567cd](https://github.com/grx-gql/grx/commit/06567cdef682f20934e1d8527e1a6f84de84ab45))
* **main:** release 0.4.0 ([8fba753](https://github.com/grx-gql/grx/commit/8fba753ecef0d892b826e917ee53720521b1bd90))
* **main:** release 0.4.0 ([946ef36](https://github.com/grx-gql/grx/commit/946ef3695e80e770436222890402802677b9c3b6))
* **main:** release 0.4.1 ([4682959](https://github.com/grx-gql/grx/commit/468295923babf2d2c071f174febe162668b727d5))
* **main:** release 0.4.1 ([db92248](https://github.com/grx-gql/grx/commit/db922487ce1f566f3019a24b92e2e792a122a28f))
* **main:** release 0.4.2 ([e626988](https://github.com/grx-gql/grx/commit/e62698835abab714023549fe2791d1608d76d97d))
* **main:** release 0.4.2 ([310ad92](https://github.com/grx-gql/grx/commit/310ad921cc9e82342464c46a3b532980293a9356))

## [0.4.2](https://github.com/grx-gql/grx/compare/v0.4.1...v0.4.2) (2026-05-24)


### 📚 Documentation

* collapse sidebar groups by default ([263b5b3](https://github.com/grx-gql/grx/commit/263b5b3b515030eaabc6e537322127ede2d56471))
* collapse sidebar groups by default ([812cea4](https://github.com/grx-gql/grx/commit/812cea447cc4b8cd7c53f51e6b17b9bcd60f6c14))
* remove RELEASING.md ([ceaaaec](https://github.com/grx-gql/grx/commit/ceaaaec34b0d172d4044ef77ef9b709f2211dd56))
* remove RELEASING.md ([dddf912](https://github.com/grx-gql/grx/commit/dddf912bc91ab106b03f08ee04333b61a7a3c7ac))

## [0.4.1](https://github.com/grx-gql/grx/compare/v0.4.0...v0.4.1) (2026-05-24)


### 🧹 Chores

* **ci:** drop tag-verify job from Release workflow ([c379db7](https://github.com/grx-gql/grx/commit/c379db78fdb111370d1d7890c6a5d12b97897420))
* **ci:** drop tag-verify job from Release workflow ([e733c7f](https://github.com/grx-gql/grx/commit/e733c7f62806b92447e239bbad075f032eecf857))

## [0.4.0](https://github.com/grx-gql/grx/compare/v0.3.0...v0.4.0) (2026-05-24)

### ⚠ BREAKING CHANGES

- flatten module layout and migrate import paths under grx-gql/grx

### ✨ Added

- add runtime observability options ([21729c3](https://github.com/grx-gql/grx/commit/21729c35221d37f19a54361c304bcb74def95a67))
- add schema coordinate resolution ([89c6f4c](https://github.com/grx-gql/grx/commit/89c6f4ced16f5a0bdeae4089b30cb5770aee0149))
- expand websocket transport controls ([9b19a21](https://github.com/grx-gql/grx/commit/9b19a21237d765ccd17cdfbc4a1962d2d2f5709a))

### 🐛 Fixed

- **ci:** gh workflows ([a87aa63](https://github.com/grx-gql/grx/commit/a87aa63b409662947e9014270a61829a7d7a9dfa))
- **ci:** gh workflows ([73b5701](https://github.com/grx-gql/grx/commit/73b5701be234fecdc9bfc6077682bb9cfe9ee6ed))
- complete graphql validation parity ([61cbcf2](https://github.com/grx-gql/grx/commit/61cbcf28e6e7befb50c000cb5734529afed6ae4a))
- **docs:** drop dead docs/changelog link from changelog pages ([08944bd](https://github.com/grx-gql/grx/commit/08944bd47a173d69ae9d0a7edafd8ba862f4fa20))
- release workflow ([b8745c7](https://github.com/grx-gql/grx/commit/b8745c7787e9898ee6c91eef510bff59323fb2a2))
- release workflow ([78818bb](https://github.com/grx-gql/grx/commit/78818bb7c8ad8b77e57560b75436fc848d804c1e))
- test coverage ([7c590d8](https://github.com/grx-gql/grx/commit/7c590d890e5229e0bc93df5fed46a8fedb9a8343))
- test coverage ([7e6fea1](https://github.com/grx-gql/grx/commit/7e6fea1f087e753b9ff1a62bce740e52af6798ae))

### 📚 Documentation

- link changelog from GitHub, remove mirrored page ([2641eeb](https://github.com/grx-gql/grx/commit/2641eeb17491efb7c877f3929bb6497d8c249cc1))
- link changelog from GitHub, remove mirrored page ([d89e1da](https://github.com/grx-gql/grx/commit/d89e1da8a4d290dcd4bd79d3ccc4ad2fd979be37))
- sync roadmap parity status ([aef2a37](https://github.com/grx-gql/grx/commit/aef2a37e0c3d822ee5f5913f53415a33e13652f9))

### 🧹 Chores

- add library coverage target ([fbbe1d8](https://github.com/grx-gql/grx/commit/fbbe1d8598817121dd8e93fd4b17d886a76615b4))
- **changelog:** resync docs/changelog.md via sync-changelog script ([ff53f58](https://github.com/grx-gql/grx/commit/ff53f581a1e2b932b31308b7a26f726e5608e93e))
- **ci:** clean up workflow ([b344f0c](https://github.com/grx-gql/grx/commit/b344f0c9d1ac04a509a057cbb9fbff21ea63bdb2))
- **ci:** clean up workflow ([37a2aed](https://github.com/grx-gql/grx/commit/37a2aedd8355da3f166c1bd6f747cb7a31f3cc21))
- **ci:** simplify release workflow ([56d479b](https://github.com/grx-gql/grx/commit/56d479bd79a443a820329001ecd3a6cd501f5b01))
- flatten module layout and migrate import paths under grx-gql/grx ([c1233c1](https://github.com/grx-gql/grx/commit/c1233c1357d39519a43ef2054027dce6d1cb8b17))
