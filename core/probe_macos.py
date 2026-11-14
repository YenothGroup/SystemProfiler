import json
import subprocess
import socket
import psutil

def _sp_json(*data_types: str) -> dict:
    """Runs system_profiler once for all requested data types in JSON format."""
    try:
        out = subprocess.check_output(
            ["system_profiler", *data_types, "-json"],
            text=True
        )
        return json.loads(out)
    except Exception:
        return {}

def get_all_macos_specs() -> dict:
    raw = _sp_json(
        "SPHardwareDataType",
        "SPDisplaysDataType",
        "SPStorageDataType",
        "SPNVMeDataType"
    )

    # 1. Motherboard / Logic Board
    hw_list = raw.get("SPHardwareDataType", [])
    hw = hw_list[0] if hw_list else {}
    motherboard = {
        "manufacturer": "Apple Inc.",
        "model": hw.get("machine_model", hw.get("machine_name", "Mac Logic Board")),
        "version": hw.get("chip_type", hw.get("cpu_type", "N/A")),
        "serial": hw.get("serial_number", "N/A"),
    }

    # 2. GPUs
    gpus = []
    for g in raw.get("SPDisplaysDataType", []):
        vram = g.get("spdisplays_vram") or g.get("spdisplays_vram_shared") or "Unified Memory"
        gpus.append({
            "name": g.get("sppci_model", "Apple GPU"),
            "vram": vram,
            "driver": g.get("spdisplays_mtl_version", "Metal"),
        })

    # 3. Storage
    disks = []
    seen_names = set()
    for s in raw.get("SPStorageDataType", []):
        phys = s.get("physical_drive", {})
        name = phys.get("device_name") or s.get("_name") or "Apple Storage"
        if name in seen_names:
            continue
        seen_names.add(name)
        size_bytes = s.get("size_in_bytes", 0)
        size_gb = round(size_bytes / (1024**3), 1) if size_bytes else 0
        disks.append({
            "model": name,
            "type": phys.get("medium_type", "SSD").upper(),
            "interface": phys.get("protocol", "Apple Fabric / NVMe"),
            "size": f"{size_gb} GB" if size_gb else "N/A",
        })

    # 4. Network Interfaces
    port_map = {}
    try:
        out = subprocess.check_output(["networksetup", "-listallhardwareports"], text=True)
        current_port = None
        for line in out.splitlines():
            if line.startswith("Hardware Port:"):
                current_port = line.split(":", 1)[1].strip()
            elif line.startswith("Device:") and current_port:
                dev = line.split(":", 1)[1].strip()
                port_map[dev] = current_port
                current_port = None
    except Exception:
        pass

    interfaces = []
    addrs = psutil.net_if_addrs()
    stats = psutil.net_if_stats()
    for dev, port_name in port_map.items():
        if dev not in addrs:
            continue
        mac, ipv4 = "N/A", "Disconnected"
        for s in addrs[dev]:
            if s.family == psutil.AF_LINK:
                mac = s.address
            elif s.family == socket.AF_INET:
                ipv4 = s.address
        st = stats.get(dev)
        speed = f"{st.speed} Mbps" if st and st.speed > 0 else "N/A"
        interfaces.append({
            "interface": dev,
            "hardware": port_name,
            "mac": mac,
            "ipv4": ipv4,
            "status": "Up" if (st and st.isup) else "Down",
            "speed": speed,
        })

    return {
        "motherboard": motherboard,
        "gpus": gpus,
        "storage": disks,
        "network": interfaces,
    }