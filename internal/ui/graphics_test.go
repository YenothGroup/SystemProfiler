package ui

import "testing"

func TestGraphicsDisplay(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{name: "no adapters", want: noGraphicsDetected},
		{name: "no usable adapter", models: []string{"", "No Graphics Detected"}, want: noGraphicsDetected},
		{name: "one adapter", models: []string{" Intel Iris Xe Graphics "}, want: "Intel Iris Xe Graphics"},
		{name: "multiple adapters", models: []string{"Intel Iris Xe Graphics", "NVIDIA GeForce RTX 4060"}, want: "Intel Iris Xe Graphics\nNVIDIA GeForce RTX 4060"},
		{name: "case insensitive duplicates", models: []string{"Intel Iris Xe Graphics", "intel iris xe graphics"}, want: "Intel Iris Xe Graphics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := graphicsDisplay(tt.models); got != tt.want {
				t.Errorf("graphicsDisplay(%v) = %q, want %q", tt.models, got, tt.want)
			}
		})
	}
}

func TestGraphicsNameIsGeneric(t *testing.T) {
	for _, name := range []string{"", "Microsoft Basic Display Adapter", "Unknown Device"} {
		if !graphicsNameIsGeneric(name) {
			t.Errorf("graphicsNameIsGeneric(%q) = false, want true", name)
		}
	}
	if graphicsNameIsGeneric("Intel Iris Xe Graphics") {
		t.Fatal("graphicsNameIsGeneric returned true for a named GPU")
	}
}
