use std::collections::{BTreeMap, VecDeque};
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct NetStats {
    pub name: String,
    pub up: bool,
    pub rx_bytes_per_sec: u64,
    pub tx_bytes_per_sec: u64,
    pub rx_total_bytes: u64,
    pub tx_total_bytes: u64,
    pub link_speed_mbps: Option<u32>,
}

#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
pub struct MetricVariabilityStats {
    pub min: f64,
    pub max: f64,
    pub mean: f64,
    pub stddev: f64,
    pub delta_range: f64,
    pub jitter_mean_abs: f64,
}

#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
pub struct StatsSummary {
    pub window_secs: u64,
    pub sample_count: usize,
    pub published_at_unix: u64,
    pub cpu_pct: MetricVariabilityStats,
    pub mem_pct: MetricVariabilityStats,
    pub net_combined_kbps: MetricVariabilityStats,
    pub rtt_ms: MetricVariabilityStats,
    pub process_restarts_delta: u32,
    pub thrashing_detected: bool,
    pub thrashing_score: f64,
    pub thrashing_reasons: Vec<String>,
}

fn round_tenth(v: f64) -> f64 {
    (v * 10.0).round() / 10.0
}

fn round_hundredth(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}

pub fn compute_metric_variability(values: &[f64]) -> MetricVariabilityStats {
    if values.is_empty() {
        return MetricVariabilityStats::default();
    }
    let mut min_v = f64::INFINITY;
    let mut max_v = f64::NEG_INFINITY;
    let mut sum = 0.0;
    for &v in values {
        if v < min_v {
            min_v = v;
        }
        if v > max_v {
            max_v = v;
        }
        sum += v;
    }
    let n = values.len() as f64;
    let mean = sum / n;
    let var_sum: f64 = values
        .iter()
        .map(|&v| {
            let d = v - mean;
            d * d
        })
        .sum();
    let stddev = (var_sum / n).sqrt();
    let jitter_mean_abs = if values.len() > 1 {
        let step_sum: f64 = values.windows(2).map(|w| (w[1] - w[0]).abs()).sum();
        step_sum / ((values.len() - 1) as f64)
    } else {
        0.0
    };

    MetricVariabilityStats {
        min: round_hundredth(min_v),
        max: round_hundredth(max_v),
        mean: round_hundredth(mean),
        stddev: round_hundredth(stddev),
        delta_range: round_hundredth((max_v - min_v).max(0.0)),
        jitter_mean_abs: round_hundredth(jitter_mean_abs),
    }
}

#[derive(Clone, Debug)]
struct WindowSamplePoint {
    timestamp_unix: u64,
    cpu_percent: f64,
    mem_percent: f64,
    net_combined_kbps: f64,
    avg_rtt_ms: Option<f64>,
    total_process_restarts: u32,
}

#[derive(Clone, Debug)]
pub struct StatsWindow {
    window_secs: u64,
    samples: VecDeque<WindowSamplePoint>,
}

impl Default for StatsWindow {
    fn default() -> Self {
        Self::new(30)
    }
}

impl StatsWindow {
    pub fn new(window_secs: u64) -> Self {
        Self {
            window_secs: window_secs.max(5),
            samples: VecDeque::with_capacity(64),
        }
    }

    pub fn record_sample(
        &mut self,
        now_unix: u64,
        host: &Stats,
        peer_rtts_ms: &BTreeMap<String, f64>,
        total_process_restarts: u32,
    ) {
        let net_bytes_sec: u64 = host
            .interfaces
            .iter()
            .map(|nic| nic.rx_bytes_per_sec.saturating_add(nic.tx_bytes_per_sec))
            .sum();
        let net_combined_kbps = (net_bytes_sec as f64) / 1024.0;
        let avg_rtt_ms = if peer_rtts_ms.is_empty() {
            None
        } else {
            let sum: f64 = peer_rtts_ms.values().copied().sum();
            Some(sum / (peer_rtts_ms.len() as f64))
        };

        self.samples.push_back(WindowSamplePoint {
            timestamp_unix: now_unix,
            cpu_percent: host.cpu_percent,
            mem_percent: host.mem_percent,
            net_combined_kbps,
            avg_rtt_ms,
            total_process_restarts,
        });

        while self.samples.len() > 6
            && self
                .samples
                .front()
                .map(|s| now_unix.saturating_sub(s.timestamp_unix) > self.window_secs + 5)
                .unwrap_or(false)
        {
            self.samples.pop_front();
        }
        while self.samples.len() > 120 {
            self.samples.pop_front();
        }
    }

    pub fn summarize(&self, now_unix: u64) -> StatsSummary {
        let cpu_vals: Vec<f64> = self.samples.iter().map(|s| s.cpu_percent).collect();
        let mem_vals: Vec<f64> = self.samples.iter().map(|s| s.mem_percent).collect();
        let net_vals: Vec<f64> = self.samples.iter().map(|s| s.net_combined_kbps).collect();
        let rtt_vals: Vec<f64> = self.samples.iter().filter_map(|s| s.avg_rtt_ms).collect();

        let cpu_pct = compute_metric_variability(&cpu_vals);
        let mem_pct = compute_metric_variability(&mem_vals);
        let net_combined_kbps = compute_metric_variability(&net_vals);
        let rtt_ms = compute_metric_variability(&rtt_vals);

        let process_restarts_delta = match (self.samples.front(), self.samples.back()) {
            (Some(first), Some(last)) => last
                .total_process_restarts
                .saturating_sub(first.total_process_restarts),
            _ => 0,
        };

        let mut reasons = Vec::new();
        let mut score: f64 = 0.0;

        // CPU variability / saturation thrashing detection
        if cpu_pct.stddev >= 20.0 || cpu_pct.jitter_mean_abs >= 25.0 || cpu_pct.delta_range >= 55.0
        {
            reasons.push(format!(
                "cpu_volatility(stddev={:.1}%,range={:.1}%,jitter={:.1}%)",
                cpu_pct.stddev, cpu_pct.delta_range, cpu_pct.jitter_mean_abs
            ));
            score += 35.0;
        }
        if cpu_pct.mean >= 85.0 {
            reasons.push(format!("cpu_saturation(mean={:.1}%)", cpu_pct.mean));
            score += 30.0;
        }

        // Memory pressure / oscillation thrashing detection
        if mem_pct.stddev >= 8.0 || mem_pct.delta_range >= 18.0 || mem_pct.jitter_mean_abs >= 10.0 {
            reasons.push(format!(
                "mem_volatility(stddev={:.1}%,range={:.1}%)",
                mem_pct.stddev, mem_pct.delta_range
            ));
            score += 35.0;
        }
        if mem_pct.mean >= 90.0 {
            reasons.push(format!("mem_pressure(mean={:.1}%)", mem_pct.mean));
            score += 25.0;
        }

        // Network burst / oscillation thrashing detection
        if net_combined_kbps.stddev >= 2048.0
            && net_combined_kbps.jitter_mean_abs >= net_combined_kbps.mean.max(512.0) * 1.25
        {
            reasons.push(format!(
                "net_burst_volatility(stddev={:.0}KB/s,jitter={:.0}KB/s)",
                net_combined_kbps.stddev, net_combined_kbps.jitter_mean_abs
            ));
            score += 15.0;
        }

        // Inter-host RTT jitter detection
        if rtt_ms.stddev >= 45.0 || rtt_ms.delta_range >= 120.0 {
            reasons.push(format!(
                "rtt_jitter(stddev={:.1}ms,range={:.1}ms)",
                rtt_ms.stddev, rtt_ms.delta_range
            ));
            score += if rtt_ms.stddev >= 65.0 { 25.0 } else { 15.0 };
        }

        // Supervised process crash-loop thrashing detection
        if process_restarts_delta >= 2 {
            reasons.push(format!(
                "process_crash_loop(restarts_in_window={process_restarts_delta})"
            ));
            score += 50.0;
        }

        let thrashing_score = round_tenth(score.min(100.0));
        let thrashing_detected = thrashing_score >= 25.0;

        StatsSummary {
            window_secs: self.window_secs,
            sample_count: self.samples.len(),
            published_at_unix: now_unix,
            cpu_pct,
            mem_pct,
            net_combined_kbps,
            rtt_ms,
            process_restarts_delta,
            thrashing_detected,
            thrashing_score,
            thrashing_reasons: reasons,
        }
    }
}

fn default_cpu_cores() -> u32 {
    std::thread::available_parallelism()
        .map(|n| n.get() as u32)
        .unwrap_or(8)
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Stats {
    #[serde(default = "default_cpu_cores")]
    pub cpu_cores: u32,
    pub cpu_percent: f64,
    pub mem_used_bytes: u64,
    pub mem_total_bytes: u64,
    pub mem_percent: f64,
    pub disk_used_bytes: u64,
    pub disk_total_bytes: u64,
    pub disk_percent: f64,
    pub interfaces: Vec<NetStats>,
    #[serde(alias = "telemetry_summary")] // compat
    pub stats_line: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary_stats: Option<StatsSummary>,
}

impl Default for Stats {
    fn default() -> Self {
        sample_stats("local", None, 0)
    }
}

struct StatsSampler {
    #[cfg(target_os = "macos")]
    prev_macos_cpu_ticks: Option<[u32; 4]>,
    #[cfg(target_os = "windows")]
    prev_win_cpu_ticks: Option<(u64, u64, u64)>,
    #[cfg(target_os = "linux")]
    prev_linux_cpu_ticks: Option<(u64, u64)>,
    prev_net_sample: Option<(u64, u64, Instant)>,
    #[cfg(target_os = "windows")]
    win_net_accum: (Option<u64>, Option<u64>, u64, u64),
    last_cached: Option<(Instant, Stats)>,
}

fn sampler_state() -> &'static Mutex<StatsSampler> {
    static STATE: OnceLock<Mutex<StatsSampler>> = OnceLock::new();
    STATE.get_or_init(|| {
        Mutex::new(StatsSampler {
            #[cfg(target_os = "macos")]
            prev_macos_cpu_ticks: None,
            #[cfg(target_os = "windows")]
            prev_win_cpu_ticks: None,
            #[cfg(target_os = "linux")]
            prev_linux_cpu_ticks: None,
            prev_net_sample: None,
            #[cfg(target_os = "windows")]
            win_net_accum: (None, None, 0, 0),
            last_cached: None,
        })
    })
}

#[cfg(target_os = "macos")]
fn read_macos_cpu_ticks() -> Option<[u32; 4]> {
    const HOST_CPU_LOAD_INFO: i32 = 3;
    const HOST_CPU_LOAD_INFO_COUNT: u32 = 4;
    extern "C" {
        fn mach_host_self() -> u32;
        fn host_statistics(
            host_priv: u32,
            flavor: i32,
            host_info_out: *mut u32,
            host_info_out_cnt: *mut u32,
        ) -> i32;
    }
    let mut ticks = [0u32; 4];
    let mut count = HOST_CPU_LOAD_INFO_COUNT;
    let kr = unsafe {
        host_statistics(
            mach_host_self(),
            HOST_CPU_LOAD_INFO,
            ticks.as_mut_ptr(),
            &mut count,
        )
    };
    if kr == 0 {
        Some(ticks)
    } else {
        None
    }
}

#[cfg(target_os = "macos")]
fn sample_macos_cpu_percent(state: &mut StatsSampler) -> f64 {
    let first = match read_macos_cpu_ticks() {
        Some(t) => t,
        None => return 0.0,
    };
    let prev = match state.prev_macos_cpu_ticks {
        Some(p) => p,
        None => {
            std::thread::sleep(Duration::from_millis(18));
            let second = read_macos_cpu_ticks().unwrap_or(first);
            state.prev_macos_cpu_ticks = Some(second);
            let d_user = second[0].wrapping_sub(first[0]) as u64;
            let d_sys = second[1].wrapping_sub(first[1]) as u64;
            let d_idle = second[2].wrapping_sub(first[2]) as u64;
            let d_nice = second[3].wrapping_sub(first[3]) as u64;
            let busy = d_user + d_sys + d_nice;
            let total = busy + d_idle;
            if total == 0 {
                return 0.0;
            }
            return round_tenth(((busy as f64) / (total as f64)) * 100.0);
        }
    };
    let d_user = first[0].wrapping_sub(prev[0]) as u64;
    let d_sys = first[1].wrapping_sub(prev[1]) as u64;
    let d_idle = first[2].wrapping_sub(prev[2]) as u64;
    let d_nice = first[3].wrapping_sub(prev[3]) as u64;
    let busy = d_user + d_sys + d_nice;
    let total = busy + d_idle;
    if total == 0 {
        if let Some((_, ref cached)) = state.last_cached {
            return cached.cpu_percent;
        }
        return 0.0;
    }
    state.prev_macos_cpu_ticks = Some(first);
    round_tenth(((busy as f64) / (total as f64)) * 100.0)
}

#[cfg(target_os = "macos")]
fn sample_macos_memory() -> (u64, u64, f64) {
    static TOTAL_MEM: OnceLock<u64> = OnceLock::new();
    let total_bytes = *TOTAL_MEM.get_or_init(|| {
        crate::run_cmd("sysctl", &["-n", "hw.memsize"])
            .and_then(|s| s.trim().parse::<u64>().ok())
            .unwrap_or(16 * 1024 * 1024 * 1024)
    });

    if let Some(vm_out) = crate::run_cmd("vm_stat", &[]) {
        let mut page_size = 16384u64;
        let mut active = 0u64;
        let mut wired = 0u64;
        let mut compressed = 0u64;
        for line in vm_out.lines() {
            if let Some(idx) = line.find("page size of ") {
                let rest = &line[idx + "page size of ".len()..];
                if let Some(num_str) = rest.split_whitespace().next() {
                    if let Ok(ps) = num_str.parse::<u64>() {
                        page_size = ps;
                    }
                }
            } else if let Some((label, val_part)) = line.split_once(':') {
                let num = val_part
                    .trim()
                    .trim_end_matches('.')
                    .parse::<u64>()
                    .unwrap_or(0);
                match label.trim() {
                    "Pages active" => active = num,
                    "Pages wired down" => wired = num,
                    "Pages occupied by compressor" => compressed = num,
                    _ => {}
                }
            }
        }
        let used_bytes = active
            .saturating_add(wired)
            .saturating_add(compressed)
            .saturating_mul(page_size)
            .min(total_bytes);
        if used_bytes > 0 && total_bytes > 0 {
            let pct = round_tenth(((used_bytes as f64) / (total_bytes as f64)) * 100.0);
            return (used_bytes, total_bytes, pct);
        }
    }
    (0, total_bytes, 0.0)
}

#[cfg(unix)]
fn sample_unix_disk() -> (u64, u64, f64) {
    let target_dir = std::env::var("DIALTONE_STATE_DIR")
        .or_else(|_| std::env::var("HOME"))
        .unwrap_or_else(|_| ".".to_string());
    let path_arg = if std::path::Path::new(&target_dir).exists() {
        target_dir.as_str()
    } else {
        "."
    };
    if let Some(df_out) = crate::run_cmd("df", &["-Pk", path_arg]) {
        if let Some(last_line) = df_out.lines().skip(1).last() {
            let cols: Vec<&str> = last_line.split_whitespace().collect();
            if cols.len() >= 4 {
                let total_kb = cols[1].parse::<u64>().unwrap_or(0);
                let used_kb = cols[2].parse::<u64>().unwrap_or(0);
                let avail_kb = cols[3].parse::<u64>().unwrap_or(0);
                if total_kb > 0 {
                    let total_bytes = total_kb.saturating_mul(1024);
                    let effective_used_kb = if avail_kb <= total_kb {
                        total_kb.saturating_sub(avail_kb)
                    } else {
                        used_kb
                    };
                    let used_bytes = effective_used_kb.saturating_mul(1024).min(total_bytes);
                    let pct = round_tenth(((used_bytes as f64) / (total_bytes as f64)) * 100.0);
                    return (used_bytes, total_bytes, pct);
                }
            }
        }
    }
    (0, 1, 0.0)
}

#[cfg(target_os = "macos")]
fn sample_macos_network(state: &mut StatsSampler, bound_ip: Option<&str>) -> NetStats {
    let mut best_name = "en0".to_string();
    let mut best_rx = 0u64;
    let mut best_tx = 0u64;

    if let Some(out) = crate::run_cmd("netstat", &["-ibn"]) {
        for line in out.lines().skip(1) {
            if !line.contains("<Link#") {
                continue;
            }
            let cols: Vec<&str> = line.split_whitespace().collect();
            if cols.len() < 10 {
                continue;
            }
            let if_name = cols[0].trim_end_matches('*');
            if if_name.starts_with("lo")
                || if_name.starts_with("utun")
                || if_name.starts_with("gif")
                || if_name.starts_with("stf")
                || if_name.starts_with("anpi")
                || if_name.starts_with("awdl")
                || if_name.starts_with("llw")
                || if_name.starts_with("bridge")
            {
                continue;
            }
            let (rx_idx, tx_idx) = if cols.len() >= 11 { (6, 9) } else { (5, 8) };
            let rx = cols
                .get(rx_idx)
                .and_then(|s| s.parse::<u64>().ok())
                .unwrap_or(0);
            let tx = cols
                .get(tx_idx)
                .and_then(|s| s.parse::<u64>().ok())
                .unwrap_or(0);
            if rx + tx > best_rx + best_tx {
                best_name = if_name.to_string();
                best_rx = rx;
                best_tx = tx;
            }
        }
    }

    let now = Instant::now();
    let (rx_rate, tx_rate) = if let Some((prev_rx, prev_tx, prev_t)) = state.prev_net_sample {
        let dt = now.duration_since(prev_t).as_secs_f64();
        if dt >= 0.2 {
            let drx = best_rx.saturating_sub(prev_rx);
            let dtx = best_tx.saturating_sub(prev_tx);
            state.prev_net_sample = Some((best_rx, best_tx, now));
            ((drx as f64 / dt) as u64, (dtx as f64 / dt) as u64)
        } else if let Some((_, ref cached)) = state.last_cached {
            let nic = cached.interfaces.first();
            (
                nic.map(|n| n.rx_bytes_per_sec).unwrap_or(0),
                nic.map(|n| n.tx_bytes_per_sec).unwrap_or(0),
            )
        } else {
            (0, 0)
        }
    } else {
        state.prev_net_sample = Some((best_rx, best_tx, now));
        (0, 0)
    };

    let ip_label = bound_ip.unwrap_or("physical-lan");
    NetStats {
        name: format!("{best_name} ({ip_label})"),
        up: best_rx + best_tx > 0,
        rx_bytes_per_sec: rx_rate,
        tx_bytes_per_sec: tx_rate,
        rx_total_bytes: best_rx,
        tx_total_bytes: best_tx,
        link_speed_mbps: Some(1000),
    }
}

#[cfg(target_os = "windows")]
mod win_os {
    #[repr(C)]
    #[derive(Copy, Clone, Default)]
    pub struct FILETIME {
        pub dw_low_date_time: u32,
        pub dw_high_date_time: u32,
    }

    impl FILETIME {
        pub fn to_u64(self) -> u64 {
            ((self.dw_high_date_time as u64) << 32) | (self.dw_low_date_time as u64)
        }
    }

    #[repr(C)]
    pub struct MEMORYSTATUSEX {
        pub dw_length: u32,
        pub dw_memory_load: u32,
        pub ull_total_phys: u64,
        pub ull_avail_phys: u64,
        pub ull_total_page_file: u64,
        pub ull_avail_page_file: u64,
        pub ull_total_virtual: u64,
        pub ull_avail_virtual: u64,
        pub ull_avail_extended_virtual: u64,
    }

    #[link(name = "kernel32")]
    extern "system" {
        pub fn GetSystemTimes(
            lp_idle_time: *mut FILETIME,
            lp_kernel_time: *mut FILETIME,
            lp_user_time: *mut FILETIME,
        ) -> i32;
        pub fn GlobalMemoryStatusEx(lp_buffer: *mut MEMORYSTATUSEX) -> i32;
        pub fn GetDiskFreeSpaceExW(
            lp_directory_name: *const u16,
            lp_free_bytes_available_to_caller: *mut u64,
            lp_total_number_of_bytes: *mut u64,
            lp_total_number_of_free_bytes: *mut u64,
        ) -> i32;
    }
}

#[cfg(target_os = "windows")]
fn read_windows_cpu_ticks() -> Option<(u64, u64, u64)> {
    let mut idle = win_os::FILETIME::default();
    let mut kernel = win_os::FILETIME::default();
    let mut user = win_os::FILETIME::default();
    let ok = unsafe { win_os::GetSystemTimes(&mut idle, &mut kernel, &mut user) };
    if ok != 0 {
        Some((idle.to_u64(), kernel.to_u64(), user.to_u64()))
    } else {
        None
    }
}

#[cfg(target_os = "windows")]
fn sample_windows_cpu_percent(state: &mut StatsSampler) -> f64 {
    let first = match read_windows_cpu_ticks() {
        Some(t) => t,
        None => return 0.0,
    };
    let prev = match state.prev_win_cpu_ticks {
        Some(p) => p,
        None => {
            std::thread::sleep(Duration::from_millis(18));
            let second = read_windows_cpu_ticks().unwrap_or(first);
            state.prev_win_cpu_ticks = Some(second);
            let d_idle = second.0.saturating_sub(first.0);
            let d_kernel = second.1.saturating_sub(first.1);
            let d_user = second.2.saturating_sub(first.2);
            let total = d_kernel.saturating_add(d_user);
            let busy = total.saturating_sub(d_idle);
            if total == 0 {
                return 0.0;
            }
            return round_tenth(((busy as f64) / (total as f64)) * 100.0);
        }
    };
    let d_idle = first.0.saturating_sub(prev.0);
    let d_kernel = first.1.saturating_sub(prev.1);
    let d_user = first.2.saturating_sub(prev.2);
    let total = d_kernel.saturating_add(d_user);
    if total == 0 {
        if let Some((_, ref cached)) = state.last_cached {
            return cached.cpu_percent;
        }
        return 0.0;
    }
    state.prev_win_cpu_ticks = Some(first);
    let busy = total.saturating_sub(d_idle);
    round_tenth(((busy as f64) / (total as f64)) * 100.0)
}

#[cfg(target_os = "windows")]
fn sample_windows_memory() -> (u64, u64, f64) {
    let mut mem = win_os::MEMORYSTATUSEX {
        dw_length: std::mem::size_of::<win_os::MEMORYSTATUSEX>() as u32,
        dw_memory_load: 0,
        ull_total_phys: 0,
        ull_avail_phys: 0,
        ull_total_page_file: 0,
        ull_avail_page_file: 0,
        ull_total_virtual: 0,
        ull_avail_virtual: 0,
        ull_avail_extended_virtual: 0,
    };
    let ok = unsafe { win_os::GlobalMemoryStatusEx(&mut mem) };
    if ok != 0 && mem.ull_total_phys > 0 {
        let total = mem.ull_total_phys;
        let used = total.saturating_sub(mem.ull_avail_phys);
        let pct = round_tenth(((used as f64) / (total as f64)) * 100.0);
        (used, total, pct)
    } else {
        (0, 1, 0.0)
    }
}

#[cfg(target_os = "windows")]
fn sample_windows_disk() -> (u64, u64, f64) {
    use std::os::windows::ffi::OsStrExt;
    let root_str =
        std::env::var_os("USERPROFILE").unwrap_or_else(|| std::ffi::OsString::from(r"C:\"));
    let mut wide: Vec<u16> = root_str.encode_wide().collect();
    wide.push(0);
    let mut free_avail = 0u64;
    let mut total_bytes = 0u64;
    let mut total_free = 0u64;
    let ok = unsafe {
        win_os::GetDiskFreeSpaceExW(
            wide.as_ptr(),
            &mut free_avail,
            &mut total_bytes,
            &mut total_free,
        )
    };
    if ok != 0 && total_bytes > 0 {
        let used = total_bytes.saturating_sub(total_free);
        let pct = round_tenth(((used as f64) / (total_bytes as f64)) * 100.0);
        (used, total_bytes, pct)
    } else {
        (0, 1, 0.0)
    }
}

#[cfg(target_os = "windows")]
fn sample_windows_network(state: &mut StatsSampler, bound_ip: Option<&str>) -> NetStats {
    let mut raw_rx = 0u64;
    let mut raw_tx = 0u64;
    if let Some(out) = crate::run_cmd("netstat", &["-e"]) {
        for line in out.lines() {
            let trimmed = line.trim();
            if trimmed.starts_with("Bytes") {
                let cols: Vec<&str> = trimmed.split_whitespace().collect();
                if cols.len() >= 3 {
                    raw_rx = cols[1].parse::<u64>().unwrap_or(0);
                    raw_tx = cols[2].parse::<u64>().unwrap_or(0);
                }
                break;
            }
        }
    }

    let (ref mut prev_raw_rx, ref mut prev_raw_tx, ref mut accum_rx, ref mut accum_tx) =
        state.win_net_accum;
    match (*prev_raw_rx, *prev_raw_tx) {
        (Some(prx), Some(ptx)) => {
            let drx = if raw_rx >= prx {
                raw_rx - prx
            } else {
                (raw_rx as u32).wrapping_sub(prx as u32) as u64
            };
            let dtx = if raw_tx >= ptx {
                raw_tx - ptx
            } else {
                (raw_tx as u32).wrapping_sub(ptx as u32) as u64
            };
            *accum_rx = accum_rx.saturating_add(drx);
            *accum_tx = accum_tx.saturating_add(dtx);
        }
        _ => {
            *accum_rx = raw_rx;
            *accum_tx = raw_tx;
        }
    }
    *prev_raw_rx = Some(raw_rx);
    *prev_raw_tx = Some(raw_tx);

    let total_rx = *accum_rx;
    let total_tx = *accum_tx;
    let now = Instant::now();
    let (rx_rate, tx_rate) = if let Some((prev_rx, prev_tx, prev_t)) = state.prev_net_sample {
        let dt = now.duration_since(prev_t).as_secs_f64();
        if dt >= 0.2 {
            let drx = total_rx.saturating_sub(prev_rx);
            let dtx = total_tx.saturating_sub(prev_tx);
            state.prev_net_sample = Some((total_rx, total_tx, now));
            ((drx as f64 / dt) as u64, (dtx as f64 / dt) as u64)
        } else if let Some((_, ref cached)) = state.last_cached {
            let nic = cached.interfaces.first();
            (
                nic.map(|n| n.rx_bytes_per_sec).unwrap_or(0),
                nic.map(|n| n.tx_bytes_per_sec).unwrap_or(0),
            )
        } else {
            (0, 0)
        }
    } else {
        state.prev_net_sample = Some((total_rx, total_tx, now));
        (0, 0)
    };

    let ip_label = bound_ip.unwrap_or("physical-lan");
    NetStats {
        name: format!("nic0 ({ip_label})"),
        up: total_rx + total_tx > 0,
        rx_bytes_per_sec: rx_rate,
        tx_bytes_per_sec: tx_rate,
        rx_total_bytes: total_rx,
        tx_total_bytes: total_tx,
        link_speed_mbps: Some(1000),
    }
}

#[cfg(target_os = "linux")]
fn read_linux_cpu_ticks() -> Option<(u64, u64)> {
    let raw = std::fs::read_to_string("/proc/stat").ok()?;
    let line = raw.lines().next()?;
    let mut parts = line.split_whitespace();
    if parts.next()? != "cpu" {
        return None;
    }
    let vals: Vec<u64> = parts.filter_map(|s| s.parse::<u64>().ok()).collect();
    if vals.len() < 4 {
        return None;
    }
    let user = vals[0];
    let nice = vals[1];
    let system = vals[2];
    let idle = vals[3];
    let iowait = vals.get(4).copied().unwrap_or(0);
    let irq = vals.get(5).copied().unwrap_or(0);
    let softirq = vals.get(6).copied().unwrap_or(0);
    let steal = vals.get(7).copied().unwrap_or(0);
    let busy = user + nice + system + irq + softirq + steal;
    let total = busy + idle + iowait;
    Some((busy, total))
}

#[cfg(target_os = "linux")]
fn sample_linux_cpu_percent(state: &mut StatsSampler) -> f64 {
    let first = match read_linux_cpu_ticks() {
        Some(t) => t,
        None => return 0.0,
    };
    let prev = match state.prev_linux_cpu_ticks {
        Some(p) => p,
        None => {
            std::thread::sleep(Duration::from_millis(18));
            let second = read_linux_cpu_ticks().unwrap_or(first);
            state.prev_linux_cpu_ticks = Some(second);
            let d_busy = second.0.saturating_sub(first.0);
            let d_total = second.1.saturating_sub(first.1);
            if d_total == 0 {
                return 0.0;
            }
            return round_tenth(((d_busy as f64) / (d_total as f64)) * 100.0);
        }
    };
    let d_busy = first.0.saturating_sub(prev.0);
    let d_total = first.1.saturating_sub(prev.1);
    if d_total == 0 {
        if let Some((_, ref cached)) = state.last_cached {
            return cached.cpu_percent;
        }
        return 0.0;
    }
    state.prev_linux_cpu_ticks = Some(first);
    round_tenth(((d_busy as f64) / (d_total as f64)) * 100.0)
}

#[cfg(target_os = "linux")]
fn sample_linux_memory() -> (u64, u64, f64) {
    if let Ok(raw) = std::fs::read_to_string("/proc/meminfo") {
        let mut total_kb = 0u64;
        let mut avail_kb = 0u64;
        for line in raw.lines() {
            if let Some((k, v)) = line.split_once(':') {
                let num = v
                    .split_whitespace()
                    .next()
                    .and_then(|s| s.parse::<u64>().ok())
                    .unwrap_or(0);
                match k.trim() {
                    "MemTotal" => total_kb = num,
                    "MemAvailable" => avail_kb = num,
                    _ => {}
                }
            }
        }
        if total_kb > 0 {
            let total_bytes = total_kb * 1024;
            let used_bytes = total_kb.saturating_sub(avail_kb) * 1024;
            let pct = round_tenth(((used_bytes as f64) / (total_bytes as f64)) * 100.0);
            return (used_bytes, total_bytes, pct);
        }
    }
    (0, 1, 0.0)
}

#[cfg(target_os = "linux")]
fn sample_linux_network(state: &mut StatsSampler, bound_ip: Option<&str>) -> NetStats {
    let mut best_name = "eth0".to_string();
    let mut best_rx = 0u64;
    let mut best_tx = 0u64;
    if let Ok(raw) = std::fs::read_to_string("/proc/net/dev") {
        for line in raw.lines().skip(2) {
            if let Some((iface, stats)) = line.split_once(':') {
                let name = iface.trim();
                if name == "lo" || name.starts_with("tailscale") || name.starts_with("docker") {
                    continue;
                }
                let cols: Vec<u64> = stats
                    .split_whitespace()
                    .filter_map(|s| s.parse::<u64>().ok())
                    .collect();
                if cols.len() >= 9 {
                    let rx = cols[0];
                    let tx = cols[8];
                    if rx + tx > best_rx + best_tx {
                        best_name = name.to_string();
                        best_rx = rx;
                        best_tx = tx;
                    }
                }
            }
        }
    }
    let now = Instant::now();
    let (rx_rate, tx_rate) = if let Some((prev_rx, prev_tx, prev_t)) = state.prev_net_sample {
        let dt = now.duration_since(prev_t).as_secs_f64();
        if dt >= 0.2 {
            let drx = best_rx.saturating_sub(prev_rx);
            let dtx = best_tx.saturating_sub(prev_tx);
            state.prev_net_sample = Some((best_rx, best_tx, now));
            ((drx as f64 / dt) as u64, (dtx as f64 / dt) as u64)
        } else if let Some((_, ref cached)) = state.last_cached {
            let nic = cached.interfaces.first();
            (
                nic.map(|n| n.rx_bytes_per_sec).unwrap_or(0),
                nic.map(|n| n.tx_bytes_per_sec).unwrap_or(0),
            )
        } else {
            (0, 0)
        }
    } else {
        state.prev_net_sample = Some((best_rx, best_tx, now));
        (0, 0)
    };
    let ip_label = bound_ip.unwrap_or("physical-lan");
    NetStats {
        name: format!("{best_name} ({ip_label})"),
        up: best_rx + best_tx > 0,
        rx_bytes_per_sec: rx_rate,
        tx_bytes_per_sec: tx_rate,
        rx_total_bytes: best_rx,
        tx_total_bytes: best_tx,
        link_speed_mbps: Some(1000),
    }
}

pub fn sample_stats(node: &str, bound_ip: Option<&str>, _counter: u64) -> Stats {
    let cores = default_cpu_cores();
    let ip_label = bound_ip.unwrap_or("physical-lan");

    if let Ok(mut guard) = sampler_state().lock() {
        if let Some((last_t, ref cached)) = guard.last_cached {
            if last_t.elapsed() < Duration::from_millis(450) {
                let mut out = cached.clone();
                if let Some(nic) = out.interfaces.first_mut() {
                    let base_nic = nic.name.split_whitespace().next().unwrap_or("nic0");
                    nic.name = format!("{base_nic} ({ip_label})");
                }
                out.stats_line = format!(
                    "node={node} cores={cores} cpu={:.1}% mem={:.1}% disk={:.1}% ip={ip_label} tailnet=off",
                    out.cpu_percent, out.mem_percent, out.disk_percent
                );
                return out;
            }
        }

        #[cfg(target_os = "macos")]
        let (
            cpu_percent,
            (mem_used_bytes, mem_total_bytes, mem_percent),
            (disk_used_bytes, disk_total_bytes, disk_percent),
            nic,
        ) = {
            let cpu = sample_macos_cpu_percent(&mut guard);
            let mem = sample_macos_memory();
            let disk = sample_unix_disk();
            let net = sample_macos_network(&mut guard, bound_ip);
            (cpu, mem, disk, net)
        };

        #[cfg(target_os = "windows")]
        let (
            cpu_percent,
            (mem_used_bytes, mem_total_bytes, mem_percent),
            (disk_used_bytes, disk_total_bytes, disk_percent),
            nic,
        ) = {
            let cpu = sample_windows_cpu_percent(&mut guard);
            let mem = sample_windows_memory();
            let disk = sample_windows_disk();
            let net = sample_windows_network(&mut guard, bound_ip);
            (cpu, mem, disk, net)
        };

        #[cfg(target_os = "linux")]
        let (
            cpu_percent,
            (mem_used_bytes, mem_total_bytes, mem_percent),
            (disk_used_bytes, disk_total_bytes, disk_percent),
            nic,
        ) = {
            let cpu = sample_linux_cpu_percent(&mut guard);
            let mem = sample_linux_memory();
            let disk = sample_unix_disk();
            let net = sample_linux_network(&mut guard, bound_ip);
            (cpu, mem, disk, net)
        };

        let sample = Stats {
            cpu_cores: cores,
            cpu_percent,
            mem_used_bytes,
            mem_total_bytes,
            mem_percent,
            disk_used_bytes,
            disk_total_bytes,
            disk_percent,
            interfaces: vec![nic],
            stats_line: format!(
                "node={node} cores={cores} cpu={cpu_percent:.1}% mem={mem_percent:.1}% disk={disk_percent:.1}% ip={ip_label} tailnet=off"
            ),
            summary_stats: None,
        };
        guard.last_cached = Some((Instant::now(), sample.clone()));
        return sample;
    }

    Stats {
        cpu_cores: cores,
        cpu_percent: 0.0,
        mem_used_bytes: 0,
        mem_total_bytes: 1,
        mem_percent: 0.0,
        disk_used_bytes: 0,
        disk_total_bytes: 1,
        disk_percent: 0.0,
        interfaces: Vec::new(),
        stats_line: format!("node={node} cores={cores} ip={ip_label} tailnet=off"),
        summary_stats: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn live_stats_reports_real_os_metrics_not_synthetic_defaults() {
        let res = sample_stats("node-alpha", Some("127.0.0.1"), 1);
        assert!(res.cpu_cores >= 1);
        assert!((0.0..=100.0).contains(&res.cpu_percent));
        assert!(res.mem_total_bytes > 1024 * 1024 * 100);
        assert!(res.mem_used_bytes > 0 && res.mem_used_bytes <= res.mem_total_bytes);
        assert!(res.disk_total_bytes > 1024 * 1024 * 100);
        assert!(res.disk_used_bytes > 0 && res.disk_used_bytes <= res.disk_total_bytes);
        // Must not be the old hardcoded 120 GiB / 512 GiB (23.4%) fake disk numbers
        assert_ne!(
            (res.disk_used_bytes, res.disk_total_bytes),
            (120 * 1024 * 1024 * 1024, 512 * 1024 * 1024 * 1024)
        );
    }

    #[test]
    fn rolling_window_computes_variability_and_detects_thrashing() {
        let mut win = StatsWindow::new(30);
        let mut rtts = BTreeMap::new();
        rtts.insert("node-beta".to_string(), 12.0);

        for (idx, (cpu, mem)) in [(10.0, 40.0), (92.0, 42.0), (12.0, 41.0), (95.0, 43.0)]
            .into_iter()
            .enumerate()
        {
            let host = Stats {
                cpu_cores: 8,
                cpu_percent: cpu,
                mem_used_bytes: 4_000_000_000,
                mem_total_bytes: 10_000_000_000,
                mem_percent: mem,
                disk_used_bytes: 50_000_000_000,
                disk_total_bytes: 100_000_000_000,
                disk_percent: 50.0,
                interfaces: vec![NetStats {
                    name: "en0".to_string(),
                    up: true,
                    rx_bytes_per_sec: 100_000,
                    tx_bytes_per_sec: 50_000,
                    rx_total_bytes: 1_000_000,
                    tx_total_bytes: 500_000,
                    link_speed_mbps: Some(1000),
                }],
                stats_line: String::new(),
                summary_stats: None,
            };
            win.record_sample(1000 + (idx as u64) * 5, &host, &rtts, 0);
        }

        let summary = win.summarize(1015);
        assert_eq!(summary.sample_count, 4);
        assert!(summary.cpu_pct.stddev > 35.0);
        assert!(summary.cpu_pct.delta_range >= 80.0);
        assert!(summary.cpu_pct.jitter_mean_abs >= 75.0);
        assert!(summary.thrashing_detected);
        assert!(summary.thrashing_score > 0.0);
        assert!(summary
            .thrashing_reasons
            .iter()
            .any(|r| r.contains("cpu_volatility")));
    }
}
