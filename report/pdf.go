package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"sysprofiler/internal/collector"
)

var (
	// Brand Color Palette (Corporate Slate & Deep Blue)
	bannerColor  = &props.Color{Red: 30, Green: 41, Blue: 59}    // Slate 800
	accentColor  = &props.Color{Red: 15, Green: 118, Blue: 110}  // Teal 700
	sectionColor = &props.Color{Red: 241, Green: 245, Blue: 249} // Slate 100
	zebraColor   = &props.Color{Red: 248, Green: 250, Blue: 252} // Slate 50
	labelColor   = &props.Color{Red: 71, Green: 85, Blue: 105}   // Slate 600
	valueColor   = &props.Color{Red: 15, Green: 23, Blue: 42}    // Slate 900
	whiteColor   = &props.Color{Red: 255, Green: 255, Blue: 255}
)

func GeneratePDF(specs *collector.SystemSpecs, outputDir string) (string, error) {
	now := time.Now()
	filename := GenerateFilename(specs.General.Model, specs.General.SerialNumber, now)
	targetPath := filepath.Join(outputDir, filename)

	cfg := config.NewBuilder().
		WithMargins(12, 12, 12).
		Build()

	m := maroto.New(cfg)

	// --- 1. Executive Top Banner ---
	m.AddRows(
		row.New(16).Add(
			col.New(12).Add(
				text.New("HARDWARE AUDIT & SYSTEM SPECIFICATION REPORT", props.Text{
					Style: fontstyle.Bold,
					Size:  14,
					Align: align.Left,
					Color: whiteColor,
				}),
			),
		).WithStyle(&props.Cell{BackgroundColor: bannerColor}),
		row.New(6).Add(
			col.New(6).Add(text.New("Certified Offline Hardware Diagnostic Extraction", props.Text{
				Size:  8,
				Color: &props.Color{Red: 148, Green: 163, Blue: 184},
			})),
			col.New(6).Add(text.New(fmt.Sprintf("Timestamp: %s", now.Format("2006-01-02 15:04:05 MST")), props.Text{
				Size:  8,
				Align: align.Right,
				Color: &props.Color{Red: 148, Green: 163, Blue: 184},
			})),
		).WithStyle(&props.Cell{BackgroundColor: bannerColor}),
		row.New(6), // Spacer
	)

	// Helper function for formatted section headers
	addSectionHeader := func(title string) {
		m.AddRows(
			row.New(7).Add(
				col.New(12).Add(
					text.New(title, props.Text{
						Style: fontstyle.Bold,
						Size:  10,
						Color: bannerColor,
					}),
				),
			).WithStyle(&props.Cell{BackgroundColor: sectionColor}),
			row.New(2),
		)
	}

	// Helper function for key-value row pairs
	addSpecRow := func(label, val string, isZebra bool) {
		r := row.New(5).Add(
			col.New(4).Add(text.New(label, props.Text{Style: fontstyle.Bold, Size: 8, Color: labelColor})),
			col.New(8).Add(text.New(val, props.Text{Size: 8, Color: valueColor})),
		)
		if isZebra {
			r.WithStyle(&props.Cell{BackgroundColor: zebraColor})
		}
		m.AddRows(r)
	}

	// --- 2. System Identification ---
	addSectionHeader("1. SYSTEM IDENTIFICATION")
	addSpecRow("Manufacturer / Vendor", specs.General.Manufacturer, false)
	addSpecRow("Product Model", specs.General.Model, true)
	addSpecRow("Serial Number (Service Tag)", specs.General.SerialNumber, false)
	addSpecRow("Operating System", specs.General.OS, true)
	addSpecRow("System Architecture", specs.CPU.Architecture, false)
	m.AddRows(row.New(4))

	// --- 3. Processor & Memory ---
	ramGB := float64(specs.Memory.TotalBytes) / (1024 * 1024 * 1024)
	addSectionHeader("2. PROCESSOR & MEMORY")
	addSpecRow("Processor Model", specs.CPU.ModelName, false)
	addSpecRow("Core Configuration", fmt.Sprintf("%d Physical Cores / %d Threads", specs.CPU.PhysicalCores, specs.CPU.LogicalCores), true)
	addSpecRow("Physical Installed RAM", fmt.Sprintf("%.0f GB (%.2f GB)", ramGB, ramGB), false)
	m.AddRows(row.New(4))

	// --- 4. Battery Diagnostics ---
	addSectionHeader("3. POWER & BATTERY DIAGNOSTICS")
	if specs.Battery.Present {
		addSpecRow("Power State", specs.Battery.State, false)
		addSpecRow("Current Charge Level", fmt.Sprintf("%.1f%%", specs.Battery.CurrentPercent), true)
		addSpecRow("State of Health (Wear)", fmt.Sprintf("%.1f%% of design rating", specs.Battery.HealthPercent), false)
		addSpecRow("Full Charge Capacity", fmt.Sprintf("%.0f mWh", specs.Battery.FullCapacity), true)
		addSpecRow("Factory Design Capacity", fmt.Sprintf("%.0f mWh", specs.Battery.DesignCapacity), false)
		addSpecRow("Cumulative Cycle Count", fmt.Sprintf("%d cycles", specs.Battery.CycleCount), true)
	} else {
		addSpecRow("Battery Detection", "No battery detected (Direct AC / Desktop System)", false)
	}
	m.AddRows(row.New(4))

	// --- 5. Network Hardware ---
	addSectionHeader("4. PHYSICAL NETWORK CONTROLLERS")
	if len(specs.NetworkAdapters) == 0 {
		addSpecRow("Network Hardware", "No physical network adapters detected", false)
	} else {
		for i, nic := range specs.NetworkAdapters {
			nicType := "LAN (Ethernet)"
			if nic.IsWireless {
				nicType = "WLAN (Wi-Fi)"
			}
			isZebra := (i % 2) != 0
			r := row.New(5).Add(
				col.New(3).Add(text.New(fmt.Sprintf("[%s]", nicType), props.Text{Style: fontstyle.Bold, Size: 8, Color: accentColor})),
				col.New(5).Add(text.New(nic.DisplayName, props.Text{Size: 8, Color: valueColor})),
				col.New(4).Add(text.New(nic.MACAddress, props.Text{Size: 8, Align: align.Right, Color: labelColor})),
			)
			if isZebra {
				r.WithStyle(&props.Cell{BackgroundColor: zebraColor})
			}
			m.AddRows(r)
		}
	}
	m.AddRows(row.New(4))

	// --- 6. Storage Hardware ---
	addSectionHeader("5. INSTALLED PHYSICAL STORAGE DRIVES")
	if len(specs.StorageDevices) == 0 {
		addSpecRow("Storage Hardware", "No physical storage drives detected", false)
	} else {
		for i, diskItem := range specs.StorageDevices {
			sizeGB := float64(diskItem.TotalBytes) / 1e9
			isZebra := (i % 2) != 0
			r := row.New(5).Add(
				col.New(8).Add(text.New(diskItem.Model, props.Text{Style: fontstyle.Bold, Size: 8, Color: valueColor})),
				col.New(4).Add(text.New(fmt.Sprintf("%.1f GB", sizeGB), props.Text{Size: 8, Align: align.Right, Color: accentColor})),
			)
			if isZebra {
				r.WithStyle(&props.Cell{BackgroundColor: zebraColor})
			}
			m.AddRows(r)
		}
	}

	// --- Footer ---
	m.AddRows(
		row.New(10), // Bottom spacer
		row.New(5).Add(
			col.New(12).Add(
				text.New("Confidential • Extracted locally with zero network sockets or external telemetry.", props.Text{
					Size:  7,
					Align: align.Center,
					Color: labelColor,
				}),
			),
		),
	)

	doc, err := m.Generate()
	if err != nil {
		return "", fmt.Errorf("failed generating PDF: %w", err)
	}

	if err := os.WriteFile(targetPath, doc.GetBytes(), 0644); err != nil {
		return "", fmt.Errorf("failed saving PDF to %s: %w", targetPath, err)
	}

	return targetPath, nil
}
