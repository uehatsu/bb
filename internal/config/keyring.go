package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// keyringService is the service name used in the OS credential store.
const keyringService = "bb:bitbucket-cli"

// KeyringCredentialStore keeps credentials in the operating system keychain
// (macOS Keychain, Windows Credential Manager, Secret Service on Linux).
type KeyringCredentialStore struct{}

// NewKeyringCredentialStore returns a keyring-backed store.
func NewKeyringCredentialStore() *KeyringCredentialStore { return &KeyringCredentialStore{} }

// Get implements CredentialStore.
func (s *KeyringCredentialStore) Get(host string) (Credential, error) {
	raw, err := keyring.Get(keyringService, host)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("reading keyring: %w", err)
	}
	var c Credential
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return Credential{}, fmt.Errorf("parsing keyring entry: %w", err)
	}
	if c.Token == "" {
		return Credential{}, ErrNotFound
	}
	if c.Method == "" {
		c.Method = AuthAPIToken
	}
	return c, nil
}

// Set implements CredentialStore.
func (s *KeyringCredentialStore) Set(host string, cred Credential) error {
	b, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, host, string(b)); err != nil {
		return fmt.Errorf("writing keyring: %w", err)
	}
	return nil
}

// Delete implements CredentialStore.
func (s *KeyringCredentialStore) Delete(host string) error {
	err := keyring.Delete(keyringService, host)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting from keyring: %w", err)
	}
	return nil
}

// StoreKind values.
const (
	StoreFile    = "file"
	StoreKeyring = "keyring"
)

// selectStore picks the credential backend: BB_CREDENTIAL_STORE overrides
// the credential_store config key; the default is the file store.
func selectStore(dir string, configured string) (CredentialStore, error) {
	kind := os.Getenv("BB_CREDENTIAL_STORE")
	if kind == "" {
		kind = configured
	}
	switch kind {
	case "", StoreFile:
		return NewFileCredentialStore(dir), nil
	case StoreKeyring:
		return NewKeyringCredentialStore(), nil
	}
	return nil, fmt.Errorf("invalid credential store %q (file|keyring)", kind)
}
