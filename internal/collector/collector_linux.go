//go:build linux

package collector

import (
	"os"
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

	// DMI System Strings
	specs.General.Manufacturer = readSysFile("/sys/class/dmi/id/sys_vendor")
	specs.General.Model = readSysFile("/sys/class/dmi/id/product_name")
	specs.General.SerialNumber = readSysFile("/sys/class/dmi/id/product_serial")
	specs.CPU.Architecture = runtime.GOARCH

	hInfo, err := host.Info()
	if err == nil {
		specs.General.HostName = hInfo.Hostname
		specs.General.OS = hInfo.Platform + " " + hInfo.PlatformVersion
	}

	// CPU & Memory
	c, _ := cpu.Info()
	if len(c) > 0 {
		specs.CPU.ModelName = c[0].ModelName
		specs.CPU.LogicalCores = len(c)
	}
	vMem, _ := mem.VirtualMemory()
	if vMem != nil {
		specs.Memory.TotalBytes = vMem.Total
		specs.Memory.AvailableBytes = vMem.Available
		specs.Memory.UsedPercentage = vMem.UsedPercent
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

		if full > 0 {
			specs.Battery.CurrentPercent = (now / full) * 100
		}
		if design > 0 {
			specs.Battery.HealthPercent = (full / design) * 100
		}
		specs.Battery.CycleCount = cycles
	}

	// Network Interfaces
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if len(iface.HardwareAddr) == 0 || iface.Name == "lo" {
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
