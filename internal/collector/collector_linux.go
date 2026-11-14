//go:build linux

package collector

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type LinuxCollector struct{}

func NewPlatformCollector() Collector {
	return &LinuxCollector{}
}

func (l *LinuxCollector) Collect() (*SystemSpecs, error) {
	specs := &SystemSpecs{}

	// DMI System & Motherboard Strings
	specs.General.Manufacturer = readSysFile("/sys/class/dmi/id/sys_vendor")
	specs.General.Model = readSysFile("/sys/class/dmi/id/product_name")
	specs.General.SerialNumber = readSysFile("/sys/class/dmi/id/product_serial")

	specs.Motherboard.Manufacturer = readSysFile("/sys/class/dmi/id/board_vendor")
	specs.Motherboard.Model = readSysFile("/sys/class/dmi/id/board_name")
	specs.Motherboard.SerialNumber = readSysFile("/sys/class/dmi/id/board_serial")

	specs.CPU.Architecture = runtime.GOARCH

	hInfo, err := host.Info()
	if err == nil {
		specs.General.HostName = hInfo.Hostname
		specs.General.OS = hInfo.Platform + " " + hInfo.PlatformVersion
	}

	// CPU Cores, Model & Clock Speeds
	physCores, _ := cpu.Counts(false)
	logCores, _ := cpu.Counts(true)
	specs.CPU.PhysicalCores = physCores
	specs.CPU.LogicalCores = logCores

	if c, err := cpu.Info(); err == nil && len(c) > 0 {
		specs.CPU.ModelName = c[0].ModelName
		if specs.CPU.LogicalCores == 0 {
			specs.CPU.LogicalCores = len(c)
		}
		var sumMhz float64
		for _, core := range c {
			sumMhz += core.Mhz
		}
		specs.CPU.CurrentClockGHz = (sumMhz / float64(len(c))) / 1000.0
	}

	if baseKhz, err := strconv.ParseFloat(readSysFile("/sys/devices/system/cpu/cpu0/cpufreq/base_frequency"), 64); err == nil && baseKhz > 0 {
		specs.CPU.BaseClockGHz = baseKhz / 1_000_000.0
	}
	if maxKhz, err := strconv.ParseFloat(readSysFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"), 64); err == nil && maxKhz > 0 {
		specs.CPU.MaxClockGHz = maxKhz / 1_000_000.0
	}

	// Memory
	vMem, _ := mem.VirtualMemory()
	if vMem != nil {
		specs.Memory.TotalBytes = vMem.Total
		specs.Memory.AvailableBytes = vMem.Available
		specs.Memory.UsedPercentage = vMem.UsedPercent
	}

	// GPUs via lspci -vmm
	if out, err := exec.Command("lspci", "-vmm").Output(); err == nil {
		for _, block := range strings.Split(strings.TrimSpace(string(out)), "\n\n") {
			fields := make(map[string]string)
			for _, line := range strings.Split(block, "\n") {
				if parts := strings.SplitN(line, ":\t", 2); len(parts) == 2 {
					fields[parts[0]] = strings.TrimSpace(parts[1])
				}
			}
			cls := fields["Class"]
			if strings.Contains(cls, "VGA compatible controller") ||
				strings.Contains(cls, "3D controller") ||
				strings.Contains(cls, "Display controller") {
				driver := fields["Driver"]
				if driver == "" {
					driver = "N/A"
				}
				specs.GPUs = append(specs.GPUs, GPUDevice{
					Name:          strings.TrimSpace(fields["Vendor"] + " " + fields["Device"]),
					VRAM:          "See DRM / Driver",
					DriverVersion: driver,
				})
			}
		}
	}

	// Battery Inspection via sysfs
	batDir := "/sys/class/power_supply/BAT0"
	if _, err := os.Stat(batDir); err == nil {
		specs.Battery.Present = true
		specs.Battery.State = readSysFile(filepath.Join(batDir, "status"))

		now, _ := strconv.ParseFloat(readSysFile(filepath.Join(batDir, "charge_now")), 64)
		if now == 0 {
			now, _ = strconv.ParseFloat(readSysFile(filepath.Join(batDir, "energy_now")), 64)
		}

		full, _ := strconv.ParseFloat(readSysFile(filepath.Join(batDir, "charge_full")), 64)
		if full == 0 {
			full, _ = strconv.ParseFloat(readSysFile(filepath.Join(batDir, "energy_full")), 64)
		}

		design, _ := strconv.ParseFloat(readSysFile(filepath.Join(batDir, "charge_full_design")), 64)
		if design == 0 {
			design, _ = strconv.ParseFloat(readSysFile(filepath.Join(batDir, "energy_full_design")), 64)
		}

		cycles, _ := strconv.Atoi(readSysFile(filepath.Join(batDir, "cycle_count")))

		specs.Battery.FullCapacity = full / 1000.0
		specs.Battery.DesignCapacity = design / 1000.0
		if full > 0 {
			specs.Battery.CurrentPercent = (now / full) * 100
		}
		if design > 0 {
			specs.Battery.HealthPercent = (full / design) * 100
		}
		specs.Battery.CycleCount = cycles
	}

	// Physical Network Interfaces (filters out docker0, veth, tailscale0, lo)
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if len(iface.HardwareAddr) == 0 || iface.Name == "lo" {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", iface.Name, "device")); err != nil {
			continue
		}

		isWireless := false
		wirelessPath := filepath.Join("/sys/class/net", iface.Name, "wireless")
		phyPath := filepath.Join("/sys/class/net", iface.Name, "phy80211")
		if _, err := os.Stat(wirelessPath); err == nil {
			isWireless = true
		} else if _, err := os.Stat(phyPath); err == nil {
			isWireless = true
		}

		specs.NetworkAdapters = append(specs.NetworkAdapters, NetworkAdapter{
			Name:        iface.Name,
			DisplayName: iface.Name,
			MACAddress:  iface.HardwareAddr,
			IsWireless:  isWireless,
		})
	}

	// Disks
	partitions, _ := disk.Partitions(false)
	for _, p := range partitions {
		u, err := disk.Usage(p.Mountpoint)
		if err == nil && u.Total > 0 {
			specs.StorageDevices = append(specs.StorageDevices, StorageDevice{
				DeviceName: p.Device,
				Model:      p.Fstype,
				TotalBytes: u.Total,
				FreeBytes:  u.Free,
			})
		}
	}

	return specs, nil
}

func readSysFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(content))
}
