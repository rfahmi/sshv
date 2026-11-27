package model

// HostEntry represents a single concrete SSH host block in ~/.ssh/config.
type HostEntry struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

// HostInput holds form input for creating or updating a host, including optional Keychain password.
type HostInput struct {
	HostEntry
	Password  string
	ClearPass bool
}

// ConfigStore manages reading and writing host entries in ~/.ssh/config.
type ConfigStore interface {
	List() ([]HostEntry, error)
	Upsert(originalAlias string, entry HostEntry) error
	Delete(alias string) error
}

// VaultStore manages encrypted passwords in macOS Keychain.
type VaultStore interface {
	GetPassword(alias string) (string, bool, error)
	SetPassword(alias string, password string) error
	DeletePassword(alias string) error
}
