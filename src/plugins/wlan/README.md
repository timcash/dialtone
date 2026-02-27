# wlan Plugin

`src/plugins/wlan` tracks the USB Wi-Fi dongle driver workflow used on rover.

Scope:
- Keep instructions and fixes for compiling/installing the USB dongle driver.
- Keep patch files we applied.
- Do not add a long-running `wlan` service.

## Rover Current Facts

- Rover kernel: `6.12.25+rpt-rpi-v8`
- Dongle initially appears as storage/CDROM mode:
  - `0bda:1a2b` (`Realtek DISK`)
- Desired Wi-Fi mode ID is typically:
  - `35bc:0102` (or another runtime Wi-Fi VID:PID depending on adapter firmware)

## Why This Is Needed

Some Realtek multi-state USB dongles boot as storage mode. Driver compile alone is not enough unless the device mode-switches to Wi-Fi mode.

## New Upstream Driver Repo

Use this repo for current out-of-tree driver work:

- `https://github.com/morrownr/rtl8852bu-20250826`

The older `rtl8852bu-20240418` repo has a retirement notice.

Migration status:
- Documentation updated to the new repo.
- Rover migrated in-place to new repo build (`rtl8852bu/1.19.21-86`) and verified on 2026-02-26.

## Prebuilt Release Artifact (Outside Git)

Published release (kernel-specific):
- https://github.com/timcash/dialtone/releases/tag/wlan-rover-8852bu-6.12.25-rpt-rpi-v8-20260226

Assets:
- `8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz`
- `8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz.sha256`

Important:
- This module only matches `aarch64` + kernel `6.12.25+rpt-rpi-v8`.
- Rebuild is required when kernel/ABI changes.

## 1) Check Adapter State

```bash
lsusb
```

If you see `0bda:1a2b`, adapter is still in CDROM/storage mode.

## 2) Ensure Required Tools

```bash
sudo apt-get update
sudo apt-get install -y usb-modeswitch usb-modeswitch-data dkms build-essential git
```

## 3) Test Live Mode Switch

```bash
sudo usb_modeswitch -W -v 0x0bda -p 0x1a2b -J
sleep 2
lsusb
```

If mode switch succeeds, `lsusb` should stop showing only `0bda:1a2b` and show runtime Wi-Fi device ID.

## 4) Persistent Fix on Raspberry Pi OS

If device keeps returning to storage mode after reboot, add USB quirk to cmdline:

```bash
sudo nano /boot/firmware/cmdline.txt
```

Append to the single existing line:

```text
usb-storage.quirks=0bda:1a2b:i
```

Then reboot:

```bash
sudo reboot
```

## 5) Build/Install Driver (New Repo)

```bash
cd ~
git clone https://github.com/morrownr/rtl8852bu-20250826.git
cd rtl8852bu-20250826
sudo ./install-driver.sh NoPrompt
```

If your runtime Wi-Fi ID is missing in upstream source, apply patch from this plugin first (see `src_v1/patches`), then run install script.

## 6) Verify Driver

```bash
lsusb
nmcli device status
ip -4 addr
modinfo 8852bu | head
```

Expected:
- dongle no longer stuck in `0bda:1a2b` storage mode
- Wi-Fi interface appears and can connect

## 7) Rollback

From driver repo:

```bash
sudo ./uninstall-driver.sh
sudo reboot
```

## Optional: Install Prebuilt Module From Release

Only for exact kernel match:

```bash
uname -r
# must be 6.12.25+rpt-rpi-v8
```

```bash
cd /tmp
curl -L -O https://github.com/timcash/dialtone/releases/download/wlan-rover-8852bu-6.12.25-rpt-rpi-v8-20260226/8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz
curl -L -O https://github.com/timcash/dialtone/releases/download/wlan-rover-8852bu-6.12.25-rpt-rpi-v8-20260226/8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz.sha256
sha256sum -c 8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz.sha256
```

```bash
sudo install -D -m 0644 8852bu-6.12.25+rpt-rpi-v8-arm64.ko.xz /lib/modules/$(uname -r)/updates/dkms/8852bu.ko.xz
sudo depmod -a
sudo modprobe -r 8852bu || true
sudo modprobe 8852bu
```
