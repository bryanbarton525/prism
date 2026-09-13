package importconfig

import "testing"

func TestParseConfig(t *testing.T) {
	cfg, err := Parse([]byte("version: 1\nmap:\n  name: id\nomit:\n  - tools"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Map["name"] != "id" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestParseConfigRejectsUnknownVersion(t *testing.T) {
	if _, err := Parse([]byte("version: 2")); err == nil {
		t.Fatal("expected version error")
	}
}
