import os
import platform
import subprocess
import psutil


def get_battery_details() -> list[dict]:
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
                        return open(p, encoding="utf-8").read().strip() if os.path.exists(p) else None

                    design = read_val("energy_full_design") or read_val("charge_full_design")
                    full = read_val("energy_full") or read_val("charge_full")
                    status = read_val("status") or "Unknown"

                    design_mwh = int(design) // 1000 if design else 0
                    full_mwh = int(full) // 1000 if full else 0
                    health = round((full_mwh / design_mwh) * 100, 1) if design_mwh else 0

                    batteries.append({
                        "name": entry,
                        "status": status,
                        "design_capacity": f"{design_mwh} mWh",
                        "full_capacity": f"{full_mwh} mWh",
                        "health": f"{health}%",
                    })

    elif system == "Darwin":
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
                    "health": f"{health}% (Cycles: {data.get('CycleCount', 'N/A')})",
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
                    "health": f"{health}%",
                })
        except Exception:
            bat = psutil.sensors_battery()
            if bat:
                batteries.append({
                    "name": "System Battery",
                    "status": "Plugged In" if bat.power_plugged else "On Battery",
                    "design_capacity": "N/A",
                    "full_capacity": f"{bat.percent}% remaining",
                    "health": "N/A",
                })

    return batteries