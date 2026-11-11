package config

import (
	"slices"
	"testing"
)

func TestOriginsAreTrimmed(t *testing.T) {
	got := ParseOrigins(" http://localhost:5173/ ,https://rackup.app,, ")
	want := []string{"http://localhost:5173", "https://rackup.app"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
