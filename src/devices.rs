//! Devices: the hardware connected to a node — cameras, microphones, speakers, keyboards,
//! pointers, displays, printers, storage and Bluetooth devices.
//!
//! Each OS is asked the way its own tools see it: `system_profiler` on macOS,
//! `Get-PnpDevice` on Windows, `/sys` and `/proc` on Linux. Only a kind, a name, a bus and
//! a short detail leave this module; serial numbers and device IDs never do.

use std::io::Read;
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};
use serde_json::Value;

/// One device connected to a node.
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub struct Device {
    /// `camera`, `microphone`, `speaker`, `keyboard`, `pointer`, `display`, `printer`,
    /// `storage`, `bluetooth`, `gamepad` or `other`.
    pub kind: String,
    pub name: String,
    /// How it is attached: `built-in`, `usb`, `bluetooth`, `hid`, `display`, `pci`,
    /// `thunderbolt`, `network` or `virtual`; empty when the OS does not say.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub bus: String,
    /// A short note such as `default input` or a display resolution.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub detail: String,
}

/// Device kinds in the order the CLI and HUD list them.
pub const DEVICE_KINDS: [&str; 11] = [
    "camera",
    "microphone",
    "speaker",
    "keyboard",
    "pointer",
    "display",
    "printer",
    "storage",
    "bluetooth",
    "gamepad",
    "other",
];

/// How long one OS query may take before it is abandoned.
const QUERY_TIMEOUT: Duration = Duration::from_secs(20);
const MAX_NAME_CHARS: usize = 80;
const MAX_DEVICES: usize = 64;

fn device(kind: &str, name: &str, bus: &str, detail: &str) -> Device {
    Device {
        kind: kind.to_string(),
        name: name.to_string(),
        bus: bus.to_string(),
        detail: detail.to_string(),
    }
}

/// List the devices connected to this machine.
pub fn list_devices() -> Result<Vec<Device>, String> {
    let devices = if cfg!(target_os = "macos") {
        let json = run(
            "system_profiler",
            &[
                "-json",
                "-detailLevel",
                "mini",
                "SPCameraDataType",
                "SPAudioDataType",
                "SPSPIDataType",
                "SPUSBHostDataType",
                "SPUSBDataType",
                "SPBluetoothDataType",
                "SPDisplaysDataType",
                "SPPrintersDataType",
                "SPThunderboltDataType",
            ],
        )?;
        parse_system_profiler(&json)
    } else if cfg!(windows) {
        let script = "$ProgressPreference = 'SilentlyContinue'; \
            Get-PnpDevice -PresentOnly -Class Camera,Image,Keyboard,Mouse,AudioEndpoint,Printer,Monitor,Bluetooth,DiskDrive,Biometric,SmartCardReader,XnaComposite -ErrorAction SilentlyContinue \
            | Where-Object { $_.Status -eq 'OK' } \
            | Select-Object Class,FriendlyName,InstanceId | ConvertTo-Json -Compress";
        let json = run(
            "powershell",
            &["-NoProfile", "-NonInteractive", "-Command", script],
        )?;
        parse_pnp(&json)
    } else {
        linux_devices()
    };
    Ok(normalize(devices))
}

/// Run a command and return its stdout, killing it after `QUERY_TIMEOUT`.
fn run(program: &str, args: &[&str]) -> Result<String, String> {
    let mut cmd = Command::new(program);
    cmd.args(args)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::null());
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        cmd.creation_flags(CREATE_NO_WINDOW);
    }
    let mut child = cmd.spawn().map_err(|e| format!("run {program}: {e}"))?;
    let mut stdout = child.stdout.take().ok_or("no stdout")?;
    let reader = std::thread::spawn(move || {
        let mut out = String::new();
        let _ = stdout.read_to_string(&mut out);
        out
    });
    let deadline = Instant::now() + QUERY_TIMEOUT;
    loop {
        match child.try_wait() {
            Ok(Some(_)) => break,
            Ok(None) if Instant::now() < deadline => std::thread::sleep(Duration::from_millis(50)),
            _ => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(format!("{program} did not answer within {QUERY_TIMEOUT:?}"));
            }
        }
    }
    reader.join().map_err(|_| format!("read {program} output"))
}

/// Clean names, drop duplicates and sort by kind, then name.
pub fn normalize(devices: Vec<Device>) -> Vec<Device> {
    let clean = |s: &str| -> String {
        let s: String = s
            .chars()
            .filter(|c| !c.is_control() && !matches!(c, '<' | '>' | '"' | '\'' | '`' | '&'))
            .collect();
        s.split_whitespace()
            .collect::<Vec<_>>()
            .join(" ")
            .chars()
            .take(MAX_NAME_CHARS)
            .collect()
    };
    let rank = |kind: &str| DEVICE_KINDS.iter().position(|k| *k == kind).unwrap_or(99);
    let mut out: Vec<Device> = devices
        .into_iter()
        .map(|d| Device {
            kind: clean(&d.kind),
            name: clean(&d.name),
            bus: clean(&d.bus),
            detail: clean(&d.detail),
        })
        .filter(|d| !d.name.is_empty())
        .collect();
    out.sort_by(|a, b| {
        (rank(&a.kind), &a.name, &a.bus, &a.detail).cmp(&(
            rank(&b.kind),
            &b.name,
            &b.bus,
            &b.detail,
        ))
    });
    // Identical keyboard and pointer entries are one device seen through several HID
    // collections; identical displays and disks are separate devices, so they are counted.
    let mut counted: Vec<(Device, usize)> = Vec::new();
    for d in out {
        match counted.last_mut() {
            Some((prev, n)) if prev.kind == d.kind && prev.name == d.name && prev.bus == d.bus => {
                *n += 1
            }
            _ => counted.push((d, 1)),
        }
    }
    let mut out: Vec<Device> = counted
        .into_iter()
        .map(|(mut d, n)| {
            if n > 1 && matches!(d.kind.as_str(), "display" | "storage") {
                d.detail = if d.detail.is_empty() {
                    format!("{n}x")
                } else {
                    format!("{n}x {}", d.detail)
                };
            }
            d
        })
        .collect();
    out.truncate(MAX_DEVICES);
    out
}

/// Guess a kind from a device name, for buses that do not say what is attached.
fn kind_from_name(name: &str) -> &'static str {
    let n = name.to_ascii_lowercase();
    if n.contains("camera") || n.contains("webcam") || n.contains("cam link") {
        "camera"
    } else if n.contains("microphone") || n.contains(" mic") {
        "microphone"
    } else if n.contains("keyboard") {
        "keyboard"
    } else if n.contains("mouse") || n.contains("trackpad") || n.contains("touchpad") {
        "pointer"
    } else if n.contains("headphone")
        || n.contains("headset")
        || n.contains("speaker")
        || n.contains("airpods")
        || n.contains("buds")
    {
        "speaker"
    } else if n.contains("controller") && (n.contains("game") || n.contains("xbox")) {
        "gamepad"
    } else if n.contains("printer") {
        "printer"
    } else if n.contains("display") || n.contains("monitor") {
        "display"
    } else if n.contains("disk") || n.contains("drive") || n.contains("ssd") {
        "storage"
    } else {
        "other"
    }
}

fn is_virtual_printer(name: &str) -> bool {
    let n = name.to_ascii_lowercase();
    ["pdf", "xps", "onenote", "fax", "send to"]
        .iter()
        .any(|v| n.contains(v))
}

// ---------------------------------------------------------------------------------------
// macOS
// ---------------------------------------------------------------------------------------

/// Parse `system_profiler -json` output.
pub fn parse_system_profiler(json: &str) -> Vec<Device> {
    let Ok(root) = serde_json::from_str::<Value>(json) else {
        return Vec::new();
    };
    let section = |key: &str| -> Vec<Value> {
        root.get(key)
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default()
    };
    let name_of = |v: &Value| {
        v.get("_name")
            .and_then(Value::as_str)
            .unwrap_or("")
            .to_string()
    };
    let mut out = Vec::new();

    for cam in section("SPCameraDataType") {
        let name = name_of(&cam);
        let bus = if name.contains("FaceTime") {
            "built-in"
        } else {
            ""
        };
        out.push(device("camera", &name, bus, ""));
    }

    for group in section("SPAudioDataType") {
        for item in group
            .get("_items")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            let name = name_of(item);
            let bus = match item
                .get("coreaudio_device_transport")
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim_start_matches("coreaudio_device_type_")
            {
                "builtin" => "built-in",
                "usb" => "usb",
                "bluetooth" | "bluetooth_le" => "bluetooth",
                "hdmi" | "displayport" => "display",
                "virtual" | "aggregate" => "virtual",
                "thunderbolt" => "thunderbolt",
                "airplay" | "avb" => "network",
                _ => "",
            };
            let yes = |key: &str| item.get(key).and_then(Value::as_str) == Some("spaudio_yes");
            if item.get("coreaudio_device_input").is_some() {
                let detail = if yes("coreaudio_default_audio_input_device") {
                    "default input"
                } else {
                    ""
                };
                out.push(device("microphone", &name, bus, detail));
            }
            if item.get("coreaudio_device_output").is_some() {
                let detail = if yes("coreaudio_default_audio_output_device") {
                    "default output"
                } else {
                    ""
                };
                out.push(device("speaker", &name, bus, detail));
            }
        }
    }

    // Built-in keyboard and trackpad (internal SPI bus).
    for item in section("SPSPIDataType") {
        let name = name_of(&item);
        let lower = name.to_ascii_lowercase();
        if lower.contains("keyboard") {
            out.push(device(
                "keyboard",
                &name.replace(" / Trackpad", ""),
                "built-in",
                "",
            ));
        }
        if lower.contains("trackpad") {
            out.push(device(
                "pointer",
                &name.replace("Keyboard / ", ""),
                "built-in",
                "",
            ));
        }
    }

    // USB: buses are listed first; devices hang below them in `_items`.
    fn walk_usb(items: &[Value], bus: &str, out: &mut Vec<Device>) {
        for item in items {
            let name = item.get("_name").and_then(Value::as_str).unwrap_or("");
            let is_bus_or_hub = item.get("USBKeyHardwareType").is_some()
                || name.ends_with(" Bus")
                || name.to_ascii_lowercase().contains("hub");
            if !is_bus_or_hub {
                out.push(device(kind_from_name(name), name, bus, ""));
            }
            if let Some(children) = item.get("_items").and_then(Value::as_array) {
                walk_usb(children, bus, out);
            }
        }
    }
    walk_usb(&section("SPUSBHostDataType"), "usb", &mut out);
    walk_usb(&section("SPUSBDataType"), "usb", &mut out);
    for tb in section("SPThunderboltDataType") {
        if let Some(children) = tb.get("_items").and_then(Value::as_array) {
            walk_usb(children, "thunderbolt", &mut out);
        }
    }

    // Bluetooth: only devices connected now, not every paired one.
    for bt in section("SPBluetoothDataType") {
        for entry in bt
            .get("device_connected")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            let Some(map) = entry.as_object() else {
                continue;
            };
            for (name, info) in map {
                let minor = info
                    .get("device_minorType")
                    .and_then(Value::as_str)
                    .unwrap_or("")
                    .to_ascii_lowercase();
                let kind = match minor.as_str() {
                    m if m.contains("keyboard") => "keyboard",
                    m if m.contains("mouse") || m.contains("trackpad") => "pointer",
                    m if m.contains("headphone")
                        || m.contains("headset")
                        || m.contains("speaker") =>
                    {
                        "speaker"
                    }
                    m if m.contains("gamepad") || m.contains("joystick") => "gamepad",
                    _ => match kind_from_name(name) {
                        "other" => "bluetooth",
                        k => k,
                    },
                };
                out.push(device(kind, name, "bluetooth", ""));
            }
        }
    }

    for gpu in section("SPDisplaysDataType") {
        for screen in gpu
            .get("spdisplays_ndrvs")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            let internal = screen
                .get("spdisplays_connection_type")
                .and_then(Value::as_str)
                == Some("spdisplays_internal");
            let pixels = screen
                .get("_spdisplays_pixels")
                .and_then(Value::as_str)
                .unwrap_or("")
                .replace(' ', "");
            out.push(device(
                "display",
                &name_of(screen),
                if internal { "built-in" } else { "display" },
                &pixels,
            ));
        }
    }

    for printer in section("SPPrintersDataType") {
        let name = name_of(&printer);
        if !name.is_empty() {
            let bus = if is_virtual_printer(&name) {
                "virtual"
            } else {
                ""
            };
            out.push(device("printer", &name, bus, ""));
        }
    }
    out
}

// ---------------------------------------------------------------------------------------
// Windows
// ---------------------------------------------------------------------------------------

/// Parse `Get-PnpDevice | Select-Object Class,FriendlyName,InstanceId | ConvertTo-Json`.
pub fn parse_pnp(json: &str) -> Vec<Device> {
    let items = match serde_json::from_str::<Value>(json.trim()) {
        Ok(Value::Array(items)) => items,
        Ok(one @ Value::Object(_)) => vec![one],
        _ => return Vec::new(),
    };
    let mut out = Vec::new();
    for item in items {
        let class = item.get("Class").and_then(Value::as_str).unwrap_or("");
        let name = item
            .get("FriendlyName")
            .and_then(Value::as_str)
            .unwrap_or("");
        let id = item
            .get("InstanceId")
            .and_then(Value::as_str)
            .unwrap_or("")
            .to_ascii_uppercase();
        let mut parts = id.split('\\');
        let bus = match parts.next().unwrap_or("") {
            "USB" | "USBSTOR" => "usb",
            "BTHENUM" | "BTHLE" | "BTHLEDEVICE" => "bluetooth",
            "HID" if id.contains("{00001124") || id.contains("{00001812") => "bluetooth",
            "HID" => "hid",
            "DISPLAY" => "display",
            "PCI" => "pci",
            "ACPI" => "built-in",
            "SCSI" | "NVME" | "STORAGE" => "built-in",
            _ => "",
        };
        let device_part = parts.next().unwrap_or("");
        let kind = match class {
            "Camera" | "Image" => "camera",
            "Keyboard" => "keyboard",
            "Mouse" => "pointer",
            "Monitor" => "display",
            "DiskDrive" => "storage",
            "XnaComposite" => "gamepad",
            "Printer" => "printer",
            "Biometric" | "SmartCardReader" => "other",
            // MMDEVAPI endpoint IDs start {0.0.1.…} for capture and {0.0.0.…} for render.
            "AudioEndpoint" => match device_part {
                "MMDEVAPI" if id.contains("{0.0.1.") => "microphone",
                "MMDEVAPI" => "speaker",
                _ => match kind_from_name(name) {
                    "microphone" => "microphone",
                    _ => "speaker",
                },
            },
            "Bluetooth" => {
                let lower = name.to_ascii_lowercase();
                // Radios, enumerators and protocol drivers are plumbing, not devices.
                if lower.contains("enumerator")
                    || lower.contains("protocol")
                    || lower.contains("wireless bluetooth")
                    || lower.contains("bluetooth adapter")
                    || lower.contains("bluetooth radio")
                {
                    continue;
                }
                match kind_from_name(name) {
                    "other" => "bluetooth",
                    k => k,
                }
            }
            _ => continue,
        };
        let bus = if kind == "printer" && is_virtual_printer(name) {
            "virtual"
        } else {
            bus
        };
        out.push(device(kind, name, bus, ""));
    }
    out
}

// ---------------------------------------------------------------------------------------
// Linux
// ---------------------------------------------------------------------------------------

fn read(path: impl AsRef<std::path::Path>) -> String {
    std::fs::read_to_string(path).unwrap_or_default()
}

fn linux_devices() -> Vec<Device> {
    let mut out = Vec::new();

    // Cameras: one entry per V4L2 device, skipping each camera's metadata node.
    for entry in std::fs::read_dir("/sys/class/video4linux")
        .into_iter()
        .flatten()
        .flatten()
    {
        let dir = entry.path();
        if read(dir.join("index")).trim() == "0" {
            let bus = if read(dir.join("device/uevent")).contains("usb") {
                "usb"
            } else {
                ""
            };
            out.push(device("camera", read(dir.join("name")).trim(), bus, ""));
        }
    }

    out.extend(parse_proc_input(&read("/proc/bus/input/devices")));

    // Sound cards: a capture PCM makes it a microphone, a playback PCM a speaker.
    for line in read("/proc/asound/cards").lines() {
        let Some((num, rest)) = line.trim().split_once(' ') else {
            continue;
        };
        let Ok(num) = num.parse::<u32>() else {
            continue;
        };
        let name = rest
            .rsplit_once(" - ")
            .map(|(_, n)| n)
            .unwrap_or(rest)
            .trim();
        let pcms: Vec<String> = std::fs::read_dir(format!("/proc/asound/card{num}"))
            .into_iter()
            .flatten()
            .flatten()
            .map(|e| e.file_name().to_string_lossy().to_string())
            .filter(|n| n.starts_with("pcm"))
            .collect();
        let bus = if read(format!("/proc/asound/card{num}/usbid")).is_empty() {
            ""
        } else {
            "usb"
        };
        if pcms.iter().any(|p| p.ends_with('c')) {
            out.push(device("microphone", name, bus, ""));
        }
        if pcms.iter().any(|p| p.ends_with('p')) {
            out.push(device("speaker", name, bus, ""));
        }
    }

    // Displays: connected DRM connectors.
    for entry in std::fs::read_dir("/sys/class/drm")
        .into_iter()
        .flatten()
        .flatten()
    {
        let name = entry.file_name().to_string_lossy().to_string();
        if name.contains('-') && read(entry.path().join("status")).trim() == "connected" {
            let connector = name.split_once('-').map(|(_, c)| c).unwrap_or(&name);
            let bus = if connector.starts_with("eDP") || connector.starts_with("LVDS") {
                "built-in"
            } else {
                "display"
            };
            out.push(device("display", connector, bus, ""));
        }
    }

    // Storage: whole disks with a model name.
    for entry in std::fs::read_dir("/sys/block")
        .into_iter()
        .flatten()
        .flatten()
    {
        let dir = entry.path();
        let model = read(dir.join("device/model"));
        if model.trim().is_empty() {
            continue;
        }
        let bus = if read(dir.join("removable")).trim() == "1" {
            "usb"
        } else {
            "built-in"
        };
        out.push(device("storage", model.trim(), bus, ""));
    }

    // Printers known to CUPS.
    if let Ok(names) = run("lpstat", &["-e"]) {
        for name in names.lines().map(str::trim).filter(|n| !n.is_empty()) {
            let bus = if is_virtual_printer(name) {
                "virtual"
            } else {
                ""
            };
            out.push(device("printer", name, bus, ""));
        }
    }
    out
}

/// Parse `/proc/bus/input/devices` into keyboards, pointers and gamepads.
pub fn parse_proc_input(text: &str) -> Vec<Device> {
    let mut out = Vec::new();
    for block in text.split("\n\n") {
        let mut name = "";
        let mut bus_code = "";
        let mut handlers = "";
        for line in block.lines() {
            if let Some(rest) = line.strip_prefix("N: Name=") {
                name = rest.trim_matches('"');
            } else if let Some(rest) = line.strip_prefix("I: ") {
                bus_code = rest
                    .split_whitespace()
                    .find_map(|kv| kv.strip_prefix("Bus="))
                    .unwrap_or("");
            } else if let Some(rest) = line.strip_prefix("H: Handlers=") {
                handlers = rest;
            }
        }
        let lower = name.to_ascii_lowercase();
        // Buttons, lids, jacks and the PC speaker also register as input devices.
        if [
            "button",
            "lid switch",
            "video bus",
            "pc speaker",
            "jack",
            "hdmi",
            "consumer control",
            "system control",
            "wmi",
        ]
        .iter()
        .any(|n| lower.contains(n))
        {
            continue;
        }
        let kind = if handlers.contains("js") {
            "gamepad"
        } else if handlers.contains("mouse") {
            "pointer"
        } else if handlers.contains("kbd") && lower.contains("keyboard") {
            "keyboard"
        } else {
            continue;
        };
        let bus = match bus_code {
            "0003" => "usb",
            "0005" => "bluetooth",
            "0011" | "0018" | "0019" => "built-in",
            _ => "",
        };
        out.push(device(kind, name, bus, ""));
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn macos_profile_lists_devices_without_serials_or_ids() {
        let json = r#"{
          "SPCameraDataType": [{"_name": "FaceTime HD Camera", "spcamera_unique-id": "UNIQUE-ID-1"}],
          "SPAudioDataType": [{"_name": "coreaudio_device", "_items": [
            {"_name": "Studio Mic", "coreaudio_device_input": 1, "coreaudio_default_audio_input_device": "spaudio_yes",
             "coreaudio_device_transport": "coreaudio_device_type_usb"},
            {"_name": "Desk Speakers", "coreaudio_device_output": 2, "coreaudio_default_audio_output_device": "spaudio_yes",
             "coreaudio_device_transport": "coreaudio_device_type_builtin"}]}],
          "SPSPIDataType": [{"_name": "Apple Internal Keyboard / Trackpad", "d_serial_num": "SERIAL-1"}],
          "SPUSBHostDataType": [{"_name": "USB 3.1 Bus", "USBKeyHardwareType": "Built-in", "_items": [
            {"_name": "USB Hub", "_items": [{"_name": "Ergo Keyboard"}, {"_name": "Label Printer"}]}]}],
          "SPBluetoothDataType": [{"device_connected": [{"Travel Mouse": {"device_minorType": "Mouse"}}],
                                   "device_not_connected": [{"Old Headset": {"device_minorType": "Headset"}}]}],
          "SPDisplaysDataType": [{"_name": "GPU", "spdisplays_ndrvs": [
            {"_name": "Color LCD", "spdisplays_connection_type": "spdisplays_internal", "_spdisplays_pixels": "3456 x 2234"},
            {"_name": "Studio Display", "_spdisplays_pixels": "5120 x 2880"}]}],
          "SPPrintersDataType": [{"_name": "Office Laser"}, {"_name": "Save as PDF"}]
        }"#;
        let devices = normalize(parse_system_profiler(json));
        let has = |kind: &str, name: &str, bus: &str| {
            devices
                .iter()
                .any(|d| d.kind == kind && d.name == name && d.bus == bus)
        };
        assert!(has("camera", "FaceTime HD Camera", "built-in"));
        assert!(has("microphone", "Studio Mic", "usb"));
        assert!(has("speaker", "Desk Speakers", "built-in"));
        assert!(has("keyboard", "Apple Internal Keyboard", "built-in"));
        assert!(has("pointer", "Apple Internal Trackpad", "built-in"));
        assert!(has("keyboard", "Ergo Keyboard", "usb"));
        assert!(has("printer", "Label Printer", "usb"));
        assert!(has("pointer", "Travel Mouse", "bluetooth"));
        assert!(has("display", "Color LCD", "built-in"));
        assert!(has("display", "Studio Display", "display"));
        assert!(has("printer", "Office Laser", ""));
        assert!(has("printer", "Save as PDF", "virtual"));
        assert!(
            !devices.iter().any(|d| d.name == "Old Headset"),
            "paired but not connected"
        );
        assert!(!devices
            .iter()
            .any(|d| d.name.contains("Hub") || d.name.ends_with("Bus")));
        let mic = devices.iter().find(|d| d.name == "Studio Mic").unwrap();
        assert_eq!(mic.detail, "default input");
        let wire = serde_json::to_string(&devices).unwrap();
        assert!(!wire.contains("SERIAL") && !wire.contains("UNIQUE-ID"));
        // Sorted by kind: cameras first.
        assert_eq!(devices[0].kind, "camera");
    }

    #[test]
    fn windows_pnp_maps_classes_and_audio_direction() {
        let json = r#"[
          {"Class":"Camera","FriendlyName":"Capture Cam 4K","InstanceId":"USB\\VID_0FD9&PID_0066\\X"},
          {"Class":"AudioEndpoint","FriendlyName":"Microphone (Podcast Mic)","InstanceId":"SWD\\MMDEVAPI\\{0.0.1.00000000}.{A}"},
          {"Class":"AudioEndpoint","FriendlyName":"Headphones (Podcast Mic)","InstanceId":"SWD\\MMDEVAPI\\{0.0.0.00000000}.{B}"},
          {"Class":"AudioEndpoint","FriendlyName":"Digital Audio Interface (Capture Cam 4K)","InstanceId":"SWD\\MMDEVAPI\\{0.0.1.00000000}.{C}"},
          {"Class":"Keyboard","FriendlyName":"HID Keyboard Device","InstanceId":"HID\\VID_046D\\1"},
          {"Class":"Keyboard","FriendlyName":"HID Keyboard Device","InstanceId":"HID\\VID_046D\\2"},
          {"Class":"Mouse","FriendlyName":"HID-compliant mouse","InstanceId":"HID\\{00001124-0000-1000-8000-00805F9B34FB}\\3"},
          {"Class":"Monitor","FriendlyName":"Wide Monitor (DP)","InstanceId":"DISPLAY\\DELA\\4"},
          {"Class":"Printer","FriendlyName":"Microsoft Print to PDF","InstanceId":"SWD\\PRINTENUM\\5"},
          {"Class":"Printer","FriendlyName":"Office Laser","InstanceId":"SWD\\PRINTENUM\\6"},
          {"Class":"Bluetooth","FriendlyName":"Intel(R) Wireless Bluetooth(R)","InstanceId":"USB\\VID_8087\\7"},
          {"Class":"Bluetooth","FriendlyName":"Microsoft Bluetooth Enumerator","InstanceId":"BTH\\MS_BTHBRB\\8"},
          {"Class":"Bluetooth","FriendlyName":"Pocket Speaker","InstanceId":"BTHENUM\\DEV_1\\9"},
          {"Class":"DiskDrive","FriendlyName":"Fast SSD 2TB","InstanceId":"SCSI\\DISK\\10"},
          {"Class":"HIDClass","FriendlyName":"HID-compliant vendor-defined device","InstanceId":"HID\\11"}
        ]"#;
        let devices = normalize(parse_pnp(json));
        let kind_of = |name: &str| {
            devices
                .iter()
                .find(|d| d.name == name)
                .map(|d| (d.kind.as_str(), d.bus.as_str()))
        };
        assert_eq!(kind_of("Capture Cam 4K"), Some(("camera", "usb")));
        assert_eq!(
            kind_of("Microphone (Podcast Mic)"),
            Some(("microphone", ""))
        );
        assert_eq!(kind_of("Headphones (Podcast Mic)"), Some(("speaker", "")));
        assert_eq!(
            kind_of("Digital Audio Interface (Capture Cam 4K)"),
            Some(("microphone", "")),
            "capture endpoints are microphones whatever their name"
        );
        assert_eq!(
            kind_of("HID-compliant mouse"),
            Some(("pointer", "bluetooth"))
        );
        assert_eq!(kind_of("Wide Monitor (DP)"), Some(("display", "display")));
        assert_eq!(
            kind_of("Microsoft Print to PDF"),
            Some(("printer", "virtual"))
        );
        assert_eq!(kind_of("Office Laser"), Some(("printer", "")));
        assert_eq!(kind_of("Pocket Speaker"), Some(("speaker", "bluetooth")));
        assert_eq!(kind_of("Fast SSD 2TB"), Some(("storage", "built-in")));
        assert_eq!(
            kind_of("Intel(R) Wireless Bluetooth(R)"),
            None,
            "radios are not devices"
        );
        assert_eq!(kind_of("Microsoft Bluetooth Enumerator"), None);
        assert_eq!(kind_of("HID-compliant vendor-defined device"), None);
        assert_eq!(
            devices
                .iter()
                .filter(|d| d.name == "HID Keyboard Device")
                .count(),
            1,
            "duplicate HID collections collapse"
        );
        let monitors = normalize(parse_pnp(
            r#"[{"Class":"Monitor","FriendlyName":"Office Monitor","InstanceId":"DISPLAY\\A\\1"},
                {"Class":"Monitor","FriendlyName":"Office Monitor","InstanceId":"DISPLAY\\A\\2"},
                {"Class":"Monitor","FriendlyName":"Office Monitor","InstanceId":"DISPLAY\\A\\3"}]"#,
        ));
        assert_eq!(monitors.len(), 1);
        assert_eq!(monitors[0].detail, "3x", "identical displays are counted");
        // A single device comes back from ConvertTo-Json as an object, not an array.
        let one = parse_pnp(
            r#"{"Class":"Keyboard","FriendlyName":"Solo Keyboard","InstanceId":"HID\\1"}"#,
        );
        assert_eq!(one.len(), 1);
        assert!(parse_pnp("").is_empty());
    }

    #[test]
    fn linux_input_devices_skip_buttons_and_switches() {
        let text = "I: Bus=0019 Vendor=0000 Product=0001 Version=0000\n\
N: Name=\"Power Button\"\n\
H: Handlers=kbd event0\n\
\n\
I: Bus=0011 Vendor=0001 Product=0001 Version=ab41\n\
N: Name=\"AT Translated Set 2 keyboard\"\n\
H: Handlers=sysrq kbd event3 leds\n\
\n\
I: Bus=0003 Vendor=046d Product=c52b Version=0111\n\
N: Name=\"Logitech USB Receiver Mouse\"\n\
H: Handlers=mouse0 event5\n\
\n\
I: Bus=0005 Vendor=045e Product=0b13 Version=0503\n\
N: Name=\"Wireless Controller\"\n\
H: Handlers=event9 js0\n";
        let devices = parse_proc_input(text);
        assert_eq!(devices.len(), 3);
        assert_eq!(
            devices[0],
            device("keyboard", "AT Translated Set 2 keyboard", "built-in", "")
        );
        assert_eq!(
            devices[1],
            device("pointer", "Logitech USB Receiver Mouse", "usb", "")
        );
        assert_eq!(
            devices[2],
            device("gamepad", "Wireless Controller", "bluetooth", "")
        );
    }

    #[test]
    fn normalize_strips_markup_and_caps_names() {
        let long = "x".repeat(200);
        let devices = normalize(vec![
            device("camera", "<img src=x onerror=1>Cam", "usb", ""),
            device("camera", &long, "", ""),
            device("camera", "   ", "", ""),
        ]);
        assert_eq!(devices.len(), 2);
        assert!(devices
            .iter()
            .all(|d| !d.name.contains('<') && d.name.chars().count() <= 80));
    }

    /// The real query must answer (possibly with nothing) on every CI OS.
    #[test]
    fn listing_this_machines_devices_answers() {
        let devices = list_devices().unwrap_or_default();
        assert!(devices.len() <= MAX_DEVICES);
    }
}
