import json
import subprocess
import socket
import psutil

def _ps_json(command: str):
    """Runs a PowerShell command returning JSON and parses it safely."""
    try:
        out = subprocess.check_output(
            ["powershell", "-NoProfile", "-Command", f"{command} | ConvertTo-Json -Compress"],
            text=True,
            creationflags=subprocess.CREATE_NO_WINDOW
        )
        if not out.strip():
            return []
        data = json.loads(out)
        return data if isinstance(data, list) else [data]
    except Exception:
        return []

def get_motherboard() -> dict:
    try:
        import wmi
        c = wmi.WMI()
        for board in c.Win32_BaseBoard():
            return {
                "manufacturer": (board.Manufacturer or "Unknown").strip(),
                "model": (board.Product or "Unknown").strip(),
                "version": (board.Version or "N/A").strip(),
                "serial": (board.SerialNumber or "N/A").strip(),
            }
    except Exception:
        pass
    return {"manufacturer": "Unknown", "model": "Unknown", "version": "N/A", "serial": "N/A"}

def get_gpus() -> list[dict]:
    gpus = []
    # Read 64-bit VRAM from registry to bypass WMI's 4GB 32-bit cap
    vram_map = {}
    try:
        import winreg
        reg_path = r"SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}"
        with winreg.OpenKey(winreg.HKEY_LOCAL_MACHINE, reg_path) as key:
            for i in range(16):
                try:
                    subkey_name = winreg.EnumKey(key, i)
                    if not subkey_name.isdigit():
                        continue
                    with winreg.OpenKey(key, subkey_name) as sub:
                        desc, _ = winreg.QueryValueEx(sub, "DriverDesc")
                        qw_mem, _ = winreg.QueryValueEx(sub, "HardwareInformation.qwMemorySize")
                        if isinstance(qw_mem, bytes):
                            qw_mem = int.from_bytes(qw_mem, "little")
                        vram_map[desc] = round(qw_mem / (1024**3), 2)
                except OSError:
                    continue
    except Exception:
        pass

    try:
        import wmi
        c = wmi.WMI()
        for gpu in c.Win32_VideoController():
            name = (gpu.Name or "Unknown GPU").strip()
            vram_gb = vram_map.get(name)
            if vram_gb is None and gpu.AdapterRAM:
                vram_gb = round(int(gpu.AdapterRAM) / (1024**3), 2)
            gpus.append({
                "name": name,
                "vram": f"{vram_gb} GB" if vram_gb else "Shared / N/A",
                "driver": (gpu.DriverVersion or "N/A").strip(),
            })
    except Exception:
        pass
    return gpus

def get_storage() -> list[dict]:
    disks = []
    # Get-PhysicalDisk accurately distinguishes NVMe, SSD, and HDD
    phys = _ps_json("Get-PhysicalDisk | Select-Object FriendlyName, MediaType, BusType, Size")
    if phys:
        for d in phys:
            size_gb = round(int(d.get("Size", 0)) / (1024**3), 1)
            disks.append({
                "model": (d.get("FriendlyName") or "Unknown Drive").strip(),
                "type": d.get("MediaType") or "Unknown",
                "interface": d.get("BusType") or "Unknown",
                "size": f"{size_gb} GB",
            })
        return disks

    # Fallback to WMI Win32_DiskDrive
    try:
        import wmi
        c = wmi.WMI()
        for d in c.Win32_DiskDrive():
            size_gb = round(int(d.Size or 0) / (1024**3), 1)
            disks.append({
                "model": (d.Model or "Unknown Drive").strip(),
                "type": (d.MediaType or "Fixed Disk").strip(),
                "interface": (d.InterfaceType or "Unknown").strip(),
                "size": f"{size_gb} GB",
            })
    except Exception:
        pass
    return disks

def get_network() -> list[dict]:
    interfaces = []
    addrs = psutil.net_if_addrs()
    stats = psutil.net_if_stats()

    adapter_types = {}
    try:
        import wmi
        c = wmi.WMI()
        for nic in c.Win32_NetworkAdapter(PhysicalAdapter=True):
            if nic.NetConnectionID:
                adapter_types[nic.NetConnectionID] = nic.Name
    except Exception:
        pass

    for iface, snics in addrs.items():
        # Filter to physical adapters if WMI succeeded; otherwise skip loopback
        if adapter_types and iface not in adapter_types:
            continue
        if iface.lower().startswith("loopback"):
            continue

        mac, ipv4 = "N/A", "Disconnected"
        for s in snics:
            if s.family == psutil.AF_LINK:
                mac = s.address
            elif s.family == socket.AF_INET:
                ipv4 = s.address

        st = stats.get(iface)
        speed = f"{st.speed} Mbps" if st and st.speed > 0 else "N/A"
        interfaces.append({
            "interface": iface,
            "hardware": adapter_types.get(iface, iface),
            "mac": mac,
            "ipv4": ipv4,
            "status": "Up" if (st and st.isup) else "Down",
            "speed": speed,
        })
    return interfaces