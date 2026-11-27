package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sshv/internal/model"
)

func TestSSHConfigStore_PreservesGlobalsAndExtras(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	initial := `# Global defaults
Host *
  AddKeysToAgent yes
  UseKeychain yes

Host harnic
  HostName 10.0.0.1
  User root
  ForwardAgent yes
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	store := &SSHConfigStore{Path: cfgPath}
	hosts, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Alias != "harnic" || hosts[0].HostName != "10.0.0.1" {
		t.Fatalf("unexpected hosts: %+v", hosts)
	}

	// Update harnic and verify ForwardAgent + Host * are preserved, and .bak is written.
	err = store.Upsert("harnic", model.HostEntry{
		Alias:    "harnic-prod",
		HostName: "10.0.0.2",
		User:     "ubuntu",
		Port:     "2222",
	})
	if err != nil {
		t.Fatalf("Upsert update failed: %v", err)
	}

	updated, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, want := range []string{"Host *", "UseKeychain yes", "Host harnic-prod", "Port 2222", "ForwardAgent yes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in updated config:\n%s", want, text)
		}
	}

	if _, err := os.Stat(cfgPath + ".bak"); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}

	// Add second host, then delete first.
	if err := store.Upsert("", model.HostEntry{Alias: "db", HostName: "10.0.0.3", User: "admin", Port: "22"}); err != nil {
		t.Fatalf("Upsert add failed: %v", err)
	}
	if err := store.Delete("harnic-prod"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	hosts, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Alias != "db" {
		t.Fatalf("expected only db after delete, got %+v", hosts)
	}
}
