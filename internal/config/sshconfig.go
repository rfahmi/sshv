package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sshv/internal/model"
)

// SSHConfigStore reads and writes ~/.ssh/config while preserving unrelated lines and wildcard blocks.
type SSHConfigStore struct {
	Path string
}

// NewDefaultStore returns an SSHConfigStore pointing to ~/.ssh/config.
func NewDefaultStore() (*SSHConfigStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return &SSHConfigStore{
		Path: filepath.Join(home, ".ssh", "config"),
	}, nil
}

// List parses the SSH config and returns all non-wildcard Host entries.
func (s *SSHConfigStore) List() ([]model.HostEntry, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ssh config: %w", err)
	}

	blocks := parseBlocks(string(data))
	var entries []model.HostEntry
	for _, b := range blocks {
		if b.alias == "" || isWildcard(b.alias) {
			continue
		}
		entries = append(entries, b.toEntry())
	}
	return entries, nil
}

// Upsert inserts a new Host block or updates an existing one matched by originalAlias.
func (s *SSHConfigStore) Upsert(originalAlias string, entry model.HostEntry) error {
	if strings.TrimSpace(entry.Alias) == "" {
		return fmt.Errorf("alias is required")
	}

	existing, err := s.readOrEmpty()
	if err != nil {
		return err
	}

	blocks := parseBlocks(existing)
	target := originalAlias
	if target == "" {
		target = entry.Alias
	}

	updated := false
	for i := range blocks {
		if blocks[i].alias == target {
			blocks[i].applyEntry(entry)
			updated = true
			break
		}
	}

	if !updated {
		// Prevent duplicate alias when creating a new host.
		for _, b := range blocks {
			if b.alias == entry.Alias {
				return fmt.Errorf("host %q already exists", entry.Alias)
			}
		}
		var nb hostBlock
		nb.applyEntry(entry)
		blocks = append(blocks, nb)
	}

	return s.writeWithBackup(renderBlocks(blocks))
}

// Delete removes the Host block matching alias.
func (s *SSHConfigStore) Delete(alias string) error {
	existing, err := s.readOrEmpty()
	if err != nil {
		return err
	}

	blocks := parseBlocks(existing)
	filtered := make([]hostBlock, 0, len(blocks))
	removed := false
	for _, b := range blocks {
		if b.alias == alias {
			removed = true
			continue
		}
		filtered = append(filtered, b)
	}
	if !removed {
		return fmt.Errorf("host %q not found", alias)
	}

	return s.writeWithBackup(renderBlocks(filtered))
}

func (s *SSHConfigStore) readOrEmpty() (string, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read ssh config: %w", err)
	}
	return string(data), nil
}

func (s *SSHConfigStore) writeWithBackup(content string) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create ssh dir: %w", err)
	}

	if existing, err := os.ReadFile(s.Path); err == nil {
		bakPath := s.Path + ".bak"
		if err := os.WriteFile(bakPath, existing, 0o600); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
	}

	if err := os.WriteFile(s.Path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write ssh config: %w", err)
	}
	return nil
}

type hostBlock struct {
	alias string // empty for preamble / Match / non-Host sections
	lines []string
}

func parseBlocks(raw string) []hostBlock {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	rawLines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var blocks []hostBlock
	var current *hostBlock

	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		key, val := splitDirective(trimmed)

		if strings.EqualFold(key, "Host") && val != "" {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &hostBlock{
				alias: val,
				lines: []string{line},
			}
			continue
		}

		if strings.EqualFold(key, "Match") {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &hostBlock{
				alias: "",
				lines: []string{line},
			}
			continue
		}

		if current == nil {
			current = &hostBlock{}
		}
		current.lines = append(current.lines, line)
	}

	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks
}

func (b *hostBlock) toEntry() model.HostEntry {
	entry := model.HostEntry{
		Alias: b.alias,
		Port:  "22",
	}
	for _, line := range b.lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, val := splitDirective(trimmed)
		switch strings.ToLower(key) {
		case "hostname":
			entry.HostName = val
		case "user":
			entry.User = val
		case "port":
			if val != "" {
				entry.Port = val
			}
		case "identityfile":
			entry.IdentityFile = val
		}
	}
	return entry
}

func (b *hostBlock) applyEntry(entry model.HostEntry) {
	b.alias = entry.Alias

	// Preserve any extra directives (e.g. ForwardAgent, LocalForward) from existing block.
	var extras []string
	if len(b.lines) > 1 {
		for _, line := range b.lines[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			key, _ := splitDirective(trimmed)
			switch strings.ToLower(key) {
			case "hostname", "user", "port", "identityfile":
				continue
			default:
				extras = append(extras, line)
			}
		}
	}

	lines := []string{fmt.Sprintf("Host %s", entry.Alias)}
	if entry.HostName != "" {
		lines = append(lines, fmt.Sprintf("  HostName %s", entry.HostName))
	}
	if entry.User != "" {
		lines = append(lines, fmt.Sprintf("  User %s", entry.User))
	}
	if entry.Port != "" && entry.Port != "22" {
		lines = append(lines, fmt.Sprintf("  Port %s", entry.Port))
	}
	if entry.IdentityFile != "" {
		lines = append(lines, fmt.Sprintf("  IdentityFile %s", entry.IdentityFile))
	}
	lines = append(lines, extras...)
	b.lines = lines
}

func renderBlocks(blocks []hostBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		blockText := strings.TrimRight(strings.Join(b.lines, "\n"), "\n")
		if blockText != "" {
			parts = append(parts, blockText)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n") + "\n"
}

func splitDirective(trimmed string) (string, string) {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", ""
	}
	// Handle both "Key Value" and "Key=Value".
	if idx := strings.IndexAny(trimmed, " \t="); idx != -1 {
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(strings.TrimLeft(trimmed[idx:], " \t="))
		return key, val
	}
	return trimmed, ""
}

func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?!") || strings.Contains(alias, " ")
}
