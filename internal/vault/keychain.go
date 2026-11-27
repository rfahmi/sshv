package vault

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const serviceName = "sshv"

// KeychainVault stores SSH passwords in macOS Keychain via go-keyring.
type KeychainVault struct{}

// NewKeychainVault creates a new macOS Keychain-backed VaultStore.
func NewKeychainVault() *KeychainVault {
	return &KeychainVault{}
}

// GetPassword retrieves a stored password for alias. Returns ("", false, nil) if none is stored.
func (v *KeychainVault) GetPassword(alias string) (string, bool, error) {
	secret, err := keyring.Get(serviceName, alias)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return secret, true, nil
}

// SetPassword saves or updates the password for alias in macOS Keychain.
func (v *KeychainVault) SetPassword(alias, password string) error {
	return keyring.Set(serviceName, alias, password)
}

// DeletePassword removes any stored password for alias; missing entries are ignored.
func (v *KeychainVault) DeletePassword(alias string) error {
	err := keyring.Delete(serviceName, alias)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
