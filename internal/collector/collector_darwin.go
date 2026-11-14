//go:build darwin

package collector

import (
	"encoding/binary"
	"encoding/json"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type DarwinCollector struct{}

func NewPlatformCollector() Collector {
	return &DarwinCollector{}
}

func (d *DarwinCollector) Collect() (*SystemSpecs, error) {
	specs := &SystemSpecs{}

	// 1. Host & Board Identification
	model, _ := syscall.Sysctl("hw.model")
	specs.General.Model = strings.TrimSpace(model)
	specs.General.Manufacturer = "Apple Inc."
	specs.Motherboard.Manufacturer = "Apple Inc."
	specs.Motherboard.Model = specs.General.Model
	specs.CPU.Architecture = runtime.GOARCH

	if hInfo, err := host.Info(); err == nil {
		specs.General.HostName = hInfo.Hostname
		specs.General.OS = "macOS " + hInfo.PlatformVersion
	}

	// 2. Serial Number & Raw Battery Capacities via ioreg
	if out, err := exec.Command("ioreg", "-l").Output(); err == nil {
		var designCap, maxCap float64
		for _, line := range strings.Split(string(out), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "IOPlatformSerialNumber") {
				if parts := strings.Split(trimmed, "="); len(parts) > 1 {
					specs.General.SerialNumber = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					specs.Motherboard.SerialNumber = specs.General.SerialNumber
				}
			} else if strings.HasPrefix(trimmed, "\"DesignCapacity\" =") {
				if parts := strings.Split(trimmed, "="); len(parts) > 1 {
					designCap, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				}
			} else if strings.HasPrefix(trimmed, "\"MaxCapacity\" =") {
				if parts := strings.Split(trimmed, "="); len(parts) > 1 {
					maxCap, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				}
			}
		}
		if designCap > 0 && maxCap > 0 {
			specs.Battery.DesignCapacity = designCap
			specs.Battery.FullCapacity = maxCap
			specs.Battery.HealthPercent = (maxCap / designCap) * 100.0
		}
	}
	if specs.General.SerialNumber == "" {
		specs.General.SerialNumber = "Unavailable"
	}

	// 3. CPU Cores, Model & Clock Speeds
	physCores, _ := cpu.Counts(false)
	logCores, _ := cpu.Counts(true)
	specs.CPU.PhysicalCores = physCores
	specs.CPU.LogicalCores = logCores

	if c, err := cpu.Info(); err == nil && len(c) > 0 {
		specs.CPU.ModelName = c[0].ModelName
		if specs.CPU.LogicalCores == 0 {
			specs.CPU.LogicalCores = len(c)
		}
		if c[0].Mhz > 0 {
			specs.CPU.BaseClockGHz = c[0].Mhz / 1000.0
		}
	}

	// Max CPU frequency via sysctl (available on Intel Macs; safely skipped on Apple Silicon)
	if rawFreq, err := syscall.Sysctl("hw.cpufrequency_max"); err == nil && len(rawFreq) > 0 {
		var buf [8]byte
		copy(buf[:], rawFreq)
		if hz := binary.LittleEndian.Uint64(buf[:]); hz > 0 {
			specs.CPU.MaxClockGHz = float64(hz) / 1e9
		}
	}

	// 4. Memory
	if vMem, err := mem.VirtualMemory(); err == nil && vMem != nil {
		specs.Memory.TotalBytes = vMem.Total
		specs.Memory.AvailableBytes = vMem.Available
		specs.Memory.UsedPercentage = vMem.UsedPercent
	}

	// 5. Single Batched system_profiler Call (Hardware, GPUs, Battery, Wi-Fi)
	wifiInterfaces := make(map[string]bool)
	cmd := exec.Command("system_profiler", "SPHardwareDataType", "SPDisplaysDataType", "SPPowerDataType", "SPAirPortDataType", "-json")
	if out, err := cmd.Output(); err == nil {
		var spData struct {
			SPHardwareDataType []struct {
				ChipType              string `json:"chip_type"`
				CPUType               string `json:"cpu_type"`
				CurrentProcessorSpeed string `json:"current_processor_speed"`
			} `json:"SPHardwareDataType"`
			SPDisplaysDataType []struct {
				Model      string `json:"sppci_model"`
				VRAM       string `json:"spdisplays_vram"`
				VRAMShared string `json:"spdisplays_vram_shared"`
				Metal      string `json:"spdisplays_mtl_version"`
			} `json:"SPDisplaysDataType"`
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
			SPAirPortDataType []struct {
				Interfaces []struct {
					BSDName    string `json:"spairport_bsd_name"`
					CardType   string `json:"spairport_card_type"`
					MACAddress string `json:"spairport_mac_address"`
				} `json:"spairport_airport_interfaces"`
			} `json:"SPAirPortDataType"`
		}

		if json.Unmarshal(out, &spData) == nil {
			// Hardware fallback for Apple Silicon (chip_type) & Intel Macs (cpu_type + current_processor_speed)
			if len(spData.SPHardwareDataType) > 0 {
				hw := spData.SPHardwareDataType[0]
				if specs.CPU.ModelName == "" {
					if hw.ChipType != "" {
						specs.CPU.ModelName = hw.ChipType
					} else if hw.CPUType != "" {
						specs.CPU.ModelName = hw.CPUType
					}
				}
				if specs.CPU.BaseClockGHz == 0 && hw.CurrentProcessorSpeed != "" {
					parts := strings.Fields(hw.CurrentProcessorSpeed)
					if len(parts) >= 1 {
						if ghz, err := strconv.ParseFloat(parts[0], 64); err == nil {
							specs.CPU.BaseClockGHz = ghz
						}
					}
				}
			}

			// GPUs
			for _, g := range spData.SPDisplaysDataType {
				vram := g.VRAM
				if vram == "" {
					vram = g.VRAMShared
				}
				if vram == "" {
					vram = "Unified Memory"
				}
				metal := g.Metal
				if metal == "" {
					metal = "Metal"
				}
				specs.GPUs = append(specs.GPUs, GPUDevice{
					Name:          g.Model,
					VRAM:          vram,
					DriverVersion: metal,
				})
			}

			// Battery status & cycle count
			if len(spData.SPPowerDataType) > 0 {
				info := spData.SPPowerDataType[0]
				if info.BatteryHealthInfo.CycleCount > 0 || info.BatteryChargeInfo.CurrentPercent > 0 {
					specs.Battery.Present = true
					specs.Battery.CycleCount = info.BatteryHealthInfo.CycleCount
					specs.Battery.State = info.BatteryHealthInfo.HealthCondition
					specs.Battery.CurrentPercent = float64(info.BatteryChargeInfo.CurrentPercent)
					if specs.Battery.HealthPercent == 0 && (info.BatteryHealthInfo.HealthCondition == "Good" || info.BatteryHealthInfo.HealthCondition == "Normal") {
						specs.Battery.HealthPercent = 100.0
					}
				}
			}

			// Wi-Fi Interfaces
			if len(spData.SPAirPortDataType) > 0 {
				for _, item := range spData.SPAirPortDataType[0].Interfaces {
					wifiInterfaces[item.BSDName] = true
					specs.NetworkAdapters = append(specs.NetworkAdapters, NetworkAdapter{
						Name:        item.BSDName,
						DisplayName: item.CardType,
						MACAddress:  item.MACAddress,
						IsWireless:  true,
					})
				}
			}
		}
	}

	// 6. Physical Wired Ethernet Interfaces (en0, en1, etc. excluding Wi-Fi & virtual tunnels)
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if !strings.HasPrefix(iface.Name, "en") || wifiInterfaces[iface.Name] || len(iface.HardwareAddr) == 0 {
				continue
			}
			specs.NetworkAdapters = append(specs.NetworkAdapters, NetworkAdapter{
				Name:        iface.Name,
				DisplayName: "Ethernet / Thunderbolt (" + iface.Name + ")",
				MACAddress:  iface.HardwareAddr,
				IsWireless:  false,
			})
		}
	}

	// 7. Storage Volumes
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
