package collector

// SystemSpecs is the unified offline model populated across all OS targets.
type SystemSpecs struct {
	General struct {
		Manufacturer string `json:"manufacturer"`
		Model        string `json:"model"`
		SerialNumber string `json:"serial_number"`
		HostName     string `json:"host_name"`
		OS           string `json:"os"`
	} `json:"general"`

	CPU struct {
		ModelName     string `json:"model_name"`
		PhysicalCores int    `json:"physical_cores"`
		LogicalCores  int    `json:"logical_cores"`
		Architecture  string `json:"architecture"`
	} `json:"cpu"`

	Memory struct {
		TotalBytes     uint64  `json:"total_bytes"`
		AvailableBytes uint64  `json:"available_bytes"`
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"memory"`

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
