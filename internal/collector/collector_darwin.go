//go:build darwin

package collector

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type DarwinCollector struct{}

func NewPlatformCollector() Collector {
	return &DarwinCollector{}
}

func (d *DarwinCollector) Collect() (*SystemSpecs, error) {
	specs := &SystemSpecs{}

	// Host & Identification
	model, _ := syscall.Sysctl("hw.model")
	specs.General.Model = strings.TrimSpace(model)
	specs.General.Manufacturer = "Apple Inc."
	specs.CPU.Architecture = runtime.GOARCH

	hInfo, err := host.Info()
	if err == nil {
		specs.General.HostName = hInfo.Hostname
		specs.General.OS = "macOS " + hInfo.PlatformVersion
	}

	// Serial Number via ioreg
	if out, err := exec.Command("ioreg", "-l").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "IOPlatformSerialNumber") {
				parts := strings.Split(line, "=")
				if len(parts) > 1 {
					specs.General.SerialNumber = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					break
				}
			}
		}
	}
	if specs.General.SerialNumber == "" {
		specs.General.SerialNumber = "Unavailable"
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

	// Battery diagnostics via system_profiler
	cmd := exec.Command("system_profiler", "SPPowerDataType", "-json")
	if out, err := cmd.Output(); err == nil {
		var powerData struct {
			SPPowerDataType []struct {
				BatteryHealthInfo struct {
					CycleCount      int    `json:"sppower_battery_cycle_count"`
					HealthCondition string `json:"sppower_battery_health"`
				} `json:"sppower_battery_health_info"`
				BatteryChargeInfo struct {
					CurrentPercent int  `json:"sppower_battery_state_of_charge"`
					IsCharging     bool `json:"sppower_battery_is_charging"`
				} `json:"sppower_battery_charge_info"`
			} `json:"SPPowerDataType"`
		}
		if json.Unmarshal(out, &powerData) == nil && len(powerData.SPPowerDataType) > 0 {
			info := powerData.SPPowerDataType[0]
			specs.Battery.Present = true
			specs.Battery.CycleCount = info.BatteryHealthInfo.CycleCount
			specs.Battery.State = info.BatteryHealthInfo.HealthCondition
			specs.Battery.CurrentPercent = float64(info.BatteryChargeInfo.CurrentPercent)
			if info.BatteryHealthInfo.HealthCondition == "Good" || info.BatteryHealthInfo.HealthCondition == "Normal" {
				specs.Battery.HealthPercent = 100.0
			}
		}
	}

	// Wi-Fi Interfaces
	wifiCmd := exec.Command("system_profiler", "SPAirPortDataType", "-json")
	if out, err := wifiCmd.Output(); err == nil {
		var airportData struct {
			SPAirPortDataType []struct {
				Interfaces []struct {
					BSDName    string `json:"spairport_bsd_name"`
					CardType   string `json:"spairport_card_type"`
					MACAddress string `json:"spairport_mac_address"`
				} `json:"spairport_airport_interfaces"`
			} `json:"SPAirPortDataType"`
		}
		if json.Unmarshal(out, &airportData) == nil && len(airportData.SPAirPortDataType) > 0 {
			for _, item := range airportData.SPAirPortDataType[0].Interfaces {
				specs.NetworkAdapters = append(specs.NetworkAdapters, NetworkAdapter{
					Name:        item.BSDName,
					DisplayName: item.CardType,
					MACAddress:  item.MACAddress,
					IsWireless:  true,
				})
			}
		}
	}

	// Storage Volumes
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
