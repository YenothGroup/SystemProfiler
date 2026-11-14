package collector

import (
	"fmt"
	"strings"
)

// SystemSpecs is the unified offline model populated across all OS targets.
type SystemSpecs struct {
	General struct {
		Manufacturer string `json:"manufacturer"`
		Model        string `json:"model"`
		SerialNumber string `json:"serial_number"`
		HostName     string `json:"host_name"`
		OS           string `json:"os"`
	} `json:"general"`

	Motherboard struct {
		Manufacturer string `json:"manufacturer"`
		Model        string `json:"model"`
		SerialNumber string `json:"serial_number"`
	} `json:"motherboard"`

	CPU struct {
		ModelName       string  `json:"model_name"`
		PhysicalCores   int     `json:"physical_cores"`
		LogicalCores    int     `json:"logical_cores"`
		BaseClockGHz    float64 `json:"base_clock_ghz"`
		MaxClockGHz     float64 `json:"max_clock_ghz"`
		CurrentClockGHz float64 `json:"current_clock_ghz"`
		Architecture    string  `json:"architecture"`
	} `json:"cpu"`

	Memory struct {
		TotalBytes     uint64  `json:"total_bytes"`
		AvailableBytes uint64  `json:"available_bytes"`
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"memory"`

	GPUs []GPUDevice `json:"gpus"`

	Battery struct {
		Present        bool    `json:"present"`
		State          string  `json:"state"`
		CurrentPercent float64 `json:"current_percent"`
		DesignCapacity float64 `json:"design_capacity_mwh"`
		FullCapacity   float64 `json:"full_capacity_mwh"`
		HealthPercent  float64 `json:"health_percent"`
		CycleCount     int     `json:"cycle_count"`
	} `json:"battery"`

	NetworkAdapters []NetworkAdapter `json:"network_adapters"`
	StorageDevices  []StorageDevice  `json:"storage_devices"`
}

// FormatClockSummary builds a clean human-readable string of available clock speeds.
func (s *SystemSpecs) FormatClockSummary() string {
	var parts []string
	if s.CPU.BaseClockGHz > 0 {
		parts = append(parts, fmt.Sprintf("Base: %.2f GHz", s.CPU.BaseClockGHz))
	}
	if s.CPU.MaxClockGHz > 0 {
		parts = append(parts, fmt.Sprintf("Max: %.2f GHz", s.CPU.MaxClockGHz))
	}
	if s.CPU.CurrentClockGHz > 0 {
		parts = append(parts, fmt.Sprintf("Live: %.2f GHz", s.CPU.CurrentClockGHz))
	}
	if len(parts) == 0 {
		return "Unavailable"
	}
	return strings.Join(parts, "  •  ")
}

type GPUDevice struct {
	Name          string `json:"name"`
	VRAM          string `json:"vram"`
	DriverVersion string `json:"driver_version"`
}

type NetworkAdapter struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	MACAddress  string `json:"mac_address"`
	IsWireless  bool   `json:"is_wireless"`
}

type StorageDevice struct {
	DeviceName string `json:"device_name"`
	Model      string `json:"model"`
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
}

type Collector interface {
	Collect() (*SystemSpecs, error)
}
