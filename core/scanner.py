import platform
import subprocess
import json
import os
import psutil
import cpuinfo

def get_battery_details():
    """Fetches present batteries and their design vs full charge capacities."""
    system = platform.system()
    batteries = []

    if system == "Linux":
        ps_path = "/sys/class/power_supply"
        if os.path.exists(ps_path):
            for entry in os.listdir(ps_path):
                if entry.startswith("BAT"):
                    base = os.path.join(ps_path, entry)
                    def read_val(name):
                        p = os.path.join(base, name)
                        return open(p).read().strip() if os.path.exists(p) else None

                    design = read_val("energy_full_design") or read_val("charge_full_design")
                    full = read_val("energy_full") or read_val("charge_full")
                    now = read_val("energy_now") or read_val("charge_now")
                    status = read_val("status") or "Unknown"

                    design_mwh = int(design) // 1000 if design else 0
                    full_mwh = int(full) // 1000 if full else 0
                    health = round((full_mwh / design_mwh) * 100, 1) if design_mwh else 0

                    batteries.append({
                        "name": entry,
                        "status": status,
                        "design_capacity": f"{design_mwh} mWh",
                        "full_capacity": f"{full_mwh} mWh",
                        "health": f"{health}%"
                    })

    elif system == "Darwin":  # macOS
        try:
            out = subprocess.check_output(["ioreg", "-rn", "AppleSmartBattery"], text=True)
            data = {}
            for line in out.splitlines():
                if "=" in line:
                    k, v = [x.strip().strip('"') for x in line.split("=", 1)]
                    data[k] = v
            if data:
                design = int(data.get("DesignCapacity", 0))
                max_cap = int(data.get("MaxCapacity", 0))
                health = round((max_cap / design) * 100, 1) if design else 0
                batteries.append({
                    "name": "Internal Battery",
                    "status": "Charging" if data.get("IsCharging") == "Yes" else "Discharging/AC",
                    "design_capacity": f"{design} mAh",
                    "full_capacity": f"{max_cap} mAh",
                    "health": f"{health}% (Cycles: {data.get('CycleCount', 'N/A')})"
                })
        except Exception:
            pass

    elif system == "Windows":
        try:
            import wmi
            c = wmi.WMI(namespace="root\\wmi")
            full_caps = {b.InstanceName: b.FullChargedCapacity for b in c.BatteryFullChargedCapacity()}
            for b in c.BatteryStaticData():
                design = b.DesignedCapacity or 0
                full = full_caps.get(b.InstanceName, 0)
                health = round((full / design) * 100, 1) if design else 0
                batteries.append({
                    "name": b.DeviceName or "Primary Battery",
                    "status": "Present",
                    "design_capacity": f"{design} mWh",
                    "full_capacity": f"{full} mWh",
                    "health": f"{health}%"
                })
        except Exception:
            # Fallback to psutil if WMI root\wmi is restricted
            bat = psutil.sensors_battery()
            if bat:
                batteries.append({
                    "name": "System Battery",
                    "status": "Plugged In" if bat.power_plugged else "On Battery",
                    "design_capacity": "N/A",
                    "full_capacity": f"{bat.percent}% remaining",
                    "health": "N/A"
                })

    return batteries

# Import get_battery_details from your existing battery probe
from .battery import get_battery_details

def _get_windows_live_clock(base_mhz: float) -> tuple[str, str]:
    """Calculates real Turbo-aware clock speed on Windows using Performance Counters."""
    try:
        # Task Manager formula: BaseSpeed * (% Processor Performance / 100)
        cmd = (
            "(Get-Counter '\\Processor Information(_Total)\\% Processor Performance' "
            "-ErrorAction Stop).CounterSamples[0].CookedValue"
        )
        out = subprocess.check_output(
            ["powershell", "-NoProfile", "-Command", cmd],
            text=True,
            creationflags=subprocess.CREATE_NO_WINDOW,
        ).strip()
        perf_pct = float(out)
        live_ghz = round((base_mhz * (perf_pct / 100.0)) / 1000.0, 2)
        return f"{round(base_mhz / 1000.0, 2)} GHz", f"{live_ghz} GHz"
    except Exception:
        return f"{round(base_mhz / 1000.0, 2)} GHz", "N/A"


def _get_cpu_speeds(info: dict) -> dict:
    system = platform.system()
    base_clock = "N/A"
    max_clock = "N/A"
    current_clock = "N/A"

    try:
        freq = psutil.cpu_freq()
    except Exception:
        freq = None

    if system == "Windows":
        # On Windows, freq.max is actually the Nominal Base Clock (e.g., 2100 MHz)
        base_mhz = freq.max if (freq and freq.max) else 0
        if base_mhz > 0:
            base_clock, current_clock = _get_windows_live_clock(base_mhz)
        # Windows WMI does not expose Intel/AMD max single-core Turbo Boost ceiling natively
        max_clock = "Dynamic (Turbo Boost)"

    elif system == "Linux":
        if freq:
            if freq.min and freq.min > 0:
                base_clock = f"{round(freq.min / 1000, 2)} GHz"
            if freq.current and freq.current > 0:
                current_clock = f"{round(freq.current / 1000, 2)} GHz"
            if freq.max and freq.max > 0:
                max_clock = f"{round(freq.max / 1000, 2)} GHz"

        # Prefer sysfs cpuinfo_max_freq if available for true hardware boost ceiling
        sys_max = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"
        if os.path.exists(sys_max):
            try:
                khz = int(open(sys_max, encoding="utf-8").read().strip())
                max_clock = f"{round(khz / 1_000_000, 2)} GHz"
            except Exception:
                pass

    elif system == "Darwin":
        base_clock = info.get("hz_advertised_friendly") or info.get("hz_actual_friendly") or "N/A"
        if freq and freq.max and freq.max > 0:
            max_clock = f"{round(freq.max / 1000, 2)} GHz"
        if freq and freq.current and freq.current > 0:
            current_clock = f"{round(freq.current / 1000, 2)} GHz"

    return {
        "base_clock": base_clock,
        "max_clock": max_clock,
        "current_clock": current_clock,
    }

def scan_all() -> dict:
    system = platform.system()
    info = cpuinfo.get_cpu_info()
    vm = psutil.virtual_memory()
    speeds = _get_cpu_speeds(info)

    if system == "Windows":
        from . import probe_windows as probe
        mobo = probe.get_motherboard()
        gpus = probe.get_gpus()
        storage = probe.get_storage()
        network = probe.get_network()
    elif system == "Linux":
        from . import probe_linux as probe
        mobo = probe.get_motherboard()
        gpus = probe.get_gpus()
        storage = probe.get_storage()
        network = probe.get_network()
    elif system == "Darwin":
        from .probe_macos import get_all_macos_specs
        mac_data = get_all_macos_specs()
        mobo = mac_data["motherboard"]
        gpus = mac_data["gpus"]
        storage = mac_data["storage"]
        network = mac_data["network"]
    else:
        mobo, gpus, storage, network = {}, [], [], []

    return {
        "OS": f"{system} {platform.release()} ({platform.machine()})",
        "CPU": {
            "model": info.get("brand_raw", "Unknown"),
            "cores": f"{psutil.cpu_count(logical=False)} Physical / {psutil.cpu_count(logical=True)} Logical",
            "arch": info.get("arch", platform.machine()),
            "base_clock": speeds["base_clock"],
            "max_clock": speeds["max_clock"],
            "current_clock": speeds["current_clock"],
        },
        "RAM": {
            "total": f"{round(vm.total / (1024**3), 2)} GB",
            "available": f"{round(vm.available / (1024**3), 2)} GB",
            "used_percent": f"{vm.percent}%",
        },
        "Motherboard": mobo,
        "GPU": gpus,
        "Storage": storage,
        "Network": network,
        "Batteries": get_battery_details(),
    }