package main

import (
	"sysprofiler/internal/collector"
	"sysprofiler/internal/ui"
)

func main() {
	col := collector.NewPlatformCollector()
	ui.RunApp(col)
}
