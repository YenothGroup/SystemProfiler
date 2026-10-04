//go:build windows

package collector

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

type WindowsCollector struct{}

func NewPlatformCollector() Collector {
	return &WindowsCollector{}
}

// WMI structs
type win32_ComputerSystem struct {
	Manufacturer string
	Model        string
}

type win32_BIOS struct {
	SerialNumber string
}

type win32_Processor struct {
	Name                      string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
}

type win32_OperatingSystem struct {
	Caption string
	Version string
}

type win32_NetworkAdapter struct {
	Name            string
	AdapterType     string
	MACAddress      string
	PhysicalAdapter bool
	NetConnectionID string
}

type win32_DiskDrive struct {
	Model         string
	InterfaceType string
	MediaType     string
	Size          uint64
	DeviceID      string
}

// Battery WMI structs from root\WMI
type batteryFullChargedCapacity struct {
	FullChargedCapacity uint32
}

type batteryStaticData struct {
	DesignedCapacity uint32
}

type batteryStatus struct {
	RemainingCapacity uint32
	Charging          bool
	Discharging       bool
}

type batteryCycleCount struct {
	CycleCount uint32
}

type win32_PhysicalMemory struct {
	Capacity uint64
}

func (w *WindowsCollector) Collect() (*SystemSpecs, error) {
	specs := &SystemSpecs{}
	specs.CPU.Architecture = runtime.GOARCH

	// 1. Manufacturer & Model (Win32_ComputerSystem)
	var cs []win32_ComputerSystem
	if err := wmi.Query("SELECT Manufacturer, Model FROM Win32_ComputerSystem", &cs); err == nil && len(cs) > 0 {
		specs.General.Manufacturer = strings.TrimSpace(cs[0].Manufacturer)
		specs.General.Model = strings.TrimSpace(cs[0].Model)
	} else {
		specs.General.Manufacturer = "Unknown"
		specs.General.Model = "PC System"
	}

	// 2. Serial Number (Win32_BIOS)
	var bios []win32_BIOS
	if err := wmi.Query("SELECT SerialNumber FROM Win32_BIOS", &bios); err == nil && len(bios) > 0 {
		specs.General.SerialNumber = strings.TrimSpace(bios[0].SerialNumber)
	}

	// 3. Operating System (Win32_OperatingSystem)
	var osInfo []win32_OperatingSystem
	if err := wmi.Query("SELECT Caption, Version FROM Win32_OperatingSystem", &osInfo); err == nil && len(osInfo) > 0 {
		specs.General.OS = fmt.Sprintf("%s (Build %s)", strings.TrimSpace(osInfo[0].Caption), strings.TrimSpace(osInfo[0].Version))
	} else {
		specs.General.OS = "Windows"
	}

	// 4. Processor (Win32_Processor)
	var procs []win32_Processor
	if err := wmi.Query("SELECT Name, NumberOfCores, NumberOfLogicalProcessors FROM Win32_Processor", &procs); err == nil && len(procs) > 0 {
		specs.CPU.ModelName = strings.TrimSpace(procs[0].Name)
		specs.CPU.PhysicalCores = int(procs[0].NumberOfCores)
		specs.CPU.LogicalCores = int(procs[0].NumberOfLogicalProcessors)
	}
	if specs.CPU.LogicalCores == 0 {
		specs.CPU.LogicalCores = runtime.NumCPU()
	}

	// Sum installed hardware RAM modules from Win32_PhysicalMemory (ignores iGPU/hardware reservations)
	var physicalRAM []win32_PhysicalMemory
	if err := wmi.Query("SELECT Capacity FROM Win32_PhysicalMemory", &physicalRAM); err == nil && len(physicalRAM) > 0 {
		var totalInstalled uint64
		for _, stick := range physicalRAM {
			totalInstalled += stick.Capacity
		}
		specs.Memory.TotalBytes = totalInstalled
	} else {
		// Fallback to usable memory if physical stick query fails
		var csMem []struct{ TotalPhysicalMemory uint64 }
		if err := wmi.Query("SELECT TotalPhysicalMemory FROM Win32_ComputerSystem", &csMem); err == nil && len(csMem) > 0 {
			specs.Memory.TotalBytes = csMem[0].TotalPhysicalMemory
		}
	}

	// 5. Battery (root\WMI)
	var bStatus []batteryStatus
	var bStatic []batteryStaticData
	var bFull []batteryFullChargedCapacity
	var bCycles []batteryCycleCount

	_ = wmi.QueryNamespace("SELECT RemainingCapacity, Charging, Discharging FROM BatteryStatus", &bStatus, `root\WMI`)
	_ = wmi.QueryNamespace("SELECT DesignedCapacity FROM BatteryStaticData", &bStatic, `root\WMI`)
	_ = wmi.QueryNamespace("SELECT FullChargedCapacity FROM BatteryFullChargedCapacity", &bFull, `root\WMI`)
	_ = wmi.QueryNamespace("SELECT CycleCount FROM BatteryCycleCount", &bCycles, `root\WMI`)

	if len(bStatic) > 0 && bStatic[0].DesignedCapacity > 0 {
		specs.Battery.Present = true
		specs.Battery.DesignCapacity = float64(bStatic[0].DesignedCapacity)

		if len(bFull) > 0 && bFull[0].FullChargedCapacity > 0 {
			specs.Battery.FullCapacity = float64(bFull[0].FullChargedCapacity)
			specs.Battery.HealthPercent = (specs.Battery.FullCapacity / specs.Battery.DesignCapacity) * 100.0
		}

		if len(bStatus) > 0 {
			remaining := float64(bStatus[0].RemainingCapacity)
			if specs.Battery.FullCapacity > 0 {
				specs.Battery.CurrentPercent = (remaining / specs.Battery.FullCapacity) * 100.0
			}
			if bStatus[0].Charging {
				specs.Battery.State = "Charging"
			} else if bStatus[0].Discharging {
				specs.Battery.State = "Discharging"
			} else {
				specs.Battery.State = "Full / AC Connected"
			}
		}

		if len(bCycles) > 0 {
			specs.Battery.CycleCount = int(bCycles[0].CycleCount)
		}
	}

	// 6. Network Adapters: Only Physical Hardware Adapters
	var adapters []win32_NetworkAdapter
	query := "SELECT Name, AdapterType, MACAddress, PhysicalAdapter, NetConnectionID FROM Win32_NetworkAdapter WHERE PhysicalAdapter = True"
	if err := wmi.Query(query, &adapters); err == nil {
		for _, nic := range adapters {
			if nic.MACAddress == "" {
				continue
			}
			isWireless := strings.Contains(strings.ToLower(nic.Name), "wi-fi") ||
				strings.Contains(strings.ToLower(nic.Name), "wireless") ||
				strings.Contains(strings.ToLower(nic.Name), "802.11") ||
				strings.Contains(strings.ToLower(nic.AdapterType), "wireless")

			displayName := nic.Name
			if nic.NetConnectionID != "" {
				displayName = fmt.Sprintf("%s (%s)", nic.Name, nic.NetConnectionID)
			}

			specs.NetworkAdapters = append(specs.NetworkAdapters, NetworkAdapter{
				Name:        nic.Name,
				DisplayName: displayName,
				MACAddress:  nic.MACAddress,
				IsWireless:  isWireless,
			})
		}
	}

	// 7. Physical Storage Drives (Win32_DiskDrive) - Excludes network/mapped shares
	var disks []win32_DiskDrive
	if err := wmi.Query("SELECT Model, InterfaceType, MediaType, Size, DeviceID FROM Win32_DiskDrive", &disks); err == nil {
		for _, d := range disks {
			bus := d.InterfaceType
			if bus == "" {
				bus = "Fixed Disk"
			}
			specs.StorageDevices = append(specs.StorageDevices, StorageDevice{
				DeviceName: d.DeviceID,
				Model:      fmt.Sprintf("%s [%s]", strings.TrimSpace(d.Model), bus),
				TotalBytes: d.Size,
			})
		}
	}

	return specs, nil
}
