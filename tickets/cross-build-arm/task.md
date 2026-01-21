# Task: Cross-compile for Linux ARM using Podman

- [ ] Migrate `build` to plugin <!-- id: 12 -->
    - [ ] Move `src/build.go` logic to `src/plugins/build/cli/build.go` <!-- id: 13 -->
    - [ ] Update `src/dev.go` to use `build_cli.RunBuild` <!-- id: 14 -->
- [ ] Support cross-compilers and podman in `install` plugin <!-- id: 15 -->
    - [ ] Update `src/plugins/install/cli/install.go` to support `gcc-arm-linux-gnueabihf`, `gcc-aarch64-linux-gnu`, and `podman` <!-- id: 16 -->
- [/] Research existing build process <!-- id: 0 -->
    - [x] Examine `src/build.go` for Podman logic <!-- id: 1 -->
- [ ] Implement new build flags <!-- id: 2 -->
    - [ ] Add `--linux-arm` and `--linux-arm64` to `RunBuild` in `src/plugins/build/cli/build.go` <!-- id: 3 -->
    - [ ] Add `--podman` to `RunBuild` (explicitly) <!-- id: 4 -->
- [ ] Refactor `buildWithPodman` <!-- id: 5 -->
    - [ ] Parameterize architecture and compiler <!-- id: 6 -->
    - [ ] Support `armv7` (32-bit) with `gcc-arm-linux-gnueabihf` <!-- id: 7 -->
    - [ ] Support `aarch64` (64-bit) with `gcc-aarch64-linux-gnu` <!-- id: 8 -->
- [ ] Verification <!-- id: 9 -->
    - [ ] Create integration test to verify command construction <!-- id: 10 -->
    - [ ] Verify binary output format <!-- id: 11 -->
