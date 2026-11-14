package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"sysprofiler/internal/collector"
	"sysprofiler/report"
)

func fieldRow(label, value string) fyne.CanvasObject {
	lbl := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	val := widget.NewLabel(value)
	val.Wrapping = fyne.TextWrapWord
	return container.NewGridWithColumns(2, lbl, val)
}

func RunApp(col collector.Collector) {
	myApp := app.NewWithID("com.portable.sysprofiler")
	win := myApp.NewWindow("Hardware Diagnostic & System Profiler")
	win.Resize(fyne.NewSize(860, 740))

	var currentSpecs *collector.SystemSpecs

	content := container.NewVBox(
		widget.NewLabel("Scanning hardware components..."),
	)
	scrollable := container.NewVScroll(container.NewPadded(content))

	exportBtn := widget.NewButtonWithIcon("Export PDF Hardware Report...", theme.DocumentSaveIcon(), func() {
		if currentSpecs == nil {
			return
		}
		folderDialog := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if lu == nil {
				return
			}

			targetDir := lu.Path()
			expectedName := report.GenerateFilename(currentSpecs.General.Model, currentSpecs.General.SerialNumber, time.Now())

			savedFile, genErr := report.GeneratePDF(currentSpecs, targetDir)
			if genErr != nil {
				dialog.ShowError(genErr, win)
				return
			}

			dialog.ShowInformation(
				"Report Export Complete",
				fmt.Sprintf("Report saved successfully:\n\nFile: %s\nFolder: %s", expectedName, filepath.Dir(savedFile)),
				win,
			)
		}, win)

		if docURI, err := storage.ParseURI("file://"); err == nil {
			if listable, err := storage.ListerForURI(docURI); err == nil {
				folderDialog.SetLocation(listable)
			}
		}

		folderDialog.Resize(fyne.NewSize(600, 400))
		folderDialog.Show()
	})
	exportBtn.Importance = widget.HighImportance
	exportBtn.Disable()

	var runScan func()
	rescanBtn := widget.NewButtonWithIcon("Rescan", theme.ViewRefreshIcon(), func() {
		runScan()
	})

	runScan = func() {
		rescanBtn.Disable()
		exportBtn.Disable()
		content.Objects = []fyne.CanvasObject{
			widget.NewLabel("Interrogating hardware buses..."),
		}
		content.Refresh()

		go func() {
			specs, err := col.Collect()
			if err != nil {
				dialog.ShowError(err, win)
				rescanBtn.Enable()
				return
			}

			currentSpecs = specs
			cards := buildSpecCards(specs)

			content.Objects = cards
			content.Refresh()
			scrollable.ScrollToTop()
			rescanBtn.Enable()
			exportBtn.Enable()
		}()
	}

	bottomBar := container.NewPadded(
		container.NewBorder(nil, nil, nil, rescanBtn, exportBtn),
	)

	layout := container.NewBorder(
		nil,
		bottomBar,
		nil,
		nil,
		scrollable,
	)

	win.SetContent(layout)
	runScan()
	win.ShowAndRun()
}

func buildSpecCards(specs *collector.SystemSpecs) []fyne.CanvasObject {
	var objs []fyne.CanvasObject

	// --- Header Title Bar ---
	title := canvas.NewText("System Hardware Profiler", theme.ForegroundColor())
	title.TextSize = 20
	title.TextStyle = fyne.TextStyle{Bold: true}
	sub := canvas.NewText(fmt.Sprintf("Host: %s • OS: %s", specs.General.HostName, specs.General.OS), theme.PlaceHolderColor())
	sub.TextSize = 11

	objs = append(objs, container.NewVBox(title, sub), widget.NewSeparator())

	// --- Section 1: System & Motherboard Identification ---
	moboText := strings.TrimSpace(specs.Motherboard.Manufacturer + " " + specs.Motherboard.Model)
	if moboText == "" {
		moboText = "Unknown"
	}
	sysDetails := container.NewVBox(
		fieldRow("Manufacturer", specs.General.Manufacturer),
		fieldRow("Product Model", specs.General.Model),
		fieldRow("Motherboard", moboText),
		fieldRow("Serial Number", specs.General.SerialNumber),
		fieldRow("Architecture", specs.CPU.Architecture),
	)
	objs = append(objs, widget.NewCard("System Identification", "Hardware Model & Board Details", sysDetails))

	// --- Section 2: Processor & Memory ---
	ramGB := float64(specs.Memory.TotalBytes) / (1024 * 1024 * 1024)
	cpuDetails := container.NewVBox(
		fieldRow("Processor", specs.CPU.ModelName),
		fieldRow("Core Topology", fmt.Sprintf("%d Physical Cores / %d Logical Threads", specs.CPU.PhysicalCores, specs.CPU.LogicalCores)),
		fieldRow("Clock Speeds", specs.FormatClockSummary()),
		fieldRow("Physical Memory", fmt.Sprintf("%.0f GB Installed RAM (%.2f GB)", ramGB, ramGB)),
	)
	objs = append(objs, widget.NewCard("Processor & Memory", "CPU Clocks, Topology, and Physical RAM", cpuDetails))

	// --- Section 3: Graphics Controllers (GPU) ---
	gpuBox := container.NewVBox()
	if len(specs.GPUs) == 0 {
		gpuBox.Add(widget.NewLabel("No dedicated or integrated GPU detected."))
	} else {
		for i, g := range specs.GPUs {
			gpuBox.Add(
				container.NewVBox(
					widget.NewLabelWithStyle(g.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel(fmt.Sprintf("VRAM: %s   •   Driver / API: %s", g.VRAM, g.DriverVersion)),
				),
			)
			if i < len(specs.GPUs)-1 {
				gpuBox.Add(widget.NewSeparator())
			}
		}
	}
	objs = append(objs, widget.NewCard("Graphics Controllers (GPU)", "Installed Display & Compute Adapters", gpuBox))

	// --- Section 4: Battery Diagnostics ---
	if specs.Battery.Present {
		batDetails := container.NewVBox(
			fieldRow("Power Status", specs.Battery.State),
			fieldRow("Current Charge", fmt.Sprintf("%.1f%%", specs.Battery.CurrentPercent)),
			fieldRow("Battery Health", fmt.Sprintf("%.1f%% of original factory rating", specs.Battery.HealthPercent)),
			fieldRow("Full Charge Capacity", fmt.Sprintf("%.0f mWh", specs.Battery.FullCapacity)),
			fieldRow("Design Capacity", fmt.Sprintf("%.0f mWh", specs.Battery.DesignCapacity)),
			fieldRow("Cumulative Cycle Count", fmt.Sprintf("%d cycles", specs.Battery.CycleCount)),
		)
		objs = append(objs, widget.NewCard("Battery Diagnostics", "Power Cells & Wear Statistics", batDetails))
	} else {
		objs = append(objs, widget.NewCard("Battery Diagnostics", "Power Cells", widget.NewLabel("No battery detected (Desktop or AC Direct).")))
	}

	// --- Section 5: Physical Network Adapters ---
	netBox := container.NewVBox()
	if len(specs.NetworkAdapters) == 0 {
		netBox.Add(widget.NewLabel("No physical network adapters detected."))
	} else {
		for i, nic := range specs.NetworkAdapters {
			nicType := "Ethernet (LAN)"
			if nic.IsWireless {
				nicType = "Wireless (Wi-Fi)"
			}
			rowItem := container.NewVBox(
				container.NewHBox(
					widget.NewLabelWithStyle(nic.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel(fmt.Sprintf("[%s]", nicType)),
				),
				widget.NewLabel(fmt.Sprintf("Physical Address (MAC): %s", nic.MACAddress)),
			)
			netBox.Add(rowItem)
			if i < len(specs.NetworkAdapters)-1 {
				netBox.Add(widget.NewSeparator())
			}
		}
	}
	objs = append(objs, widget.NewCard("Network Controllers", "Installed Physical Network Interfaces", netBox))

	// --- Section 6: Physical Storage Drives ---
	diskBox := container.NewVBox()
	if len(specs.StorageDevices) == 0 {
		diskBox.Add(widget.NewLabel("No physical storage drives detected."))
	} else {
		for _, d := range specs.StorageDevices {
			rowItem := container.NewHBox(
				widget.NewLabelWithStyle("• "+d.Model, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(fmt.Sprintf("(%.2f GB)", float64(d.TotalBytes)/1e9)),
			)
			diskBox.Add(rowItem)
		}
	}
	objs = append(objs, widget.NewCard("Storage Drives", "Internal Solid State & Hard Drives", diskBox))

	return objs
}
