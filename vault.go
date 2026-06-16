package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type VaultMeta struct {
	Salt         []byte `json:"salt"`
	Hash         []byte `json:"hash"` // SHA256 of the Argon2 derived key
	Passwordless bool   `json:"passwordless"`
}

func isVaultPasswordless(vaultPath string) bool {
	metaPath := filepath.Join(vaultPath, ".vault_meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return false
	}
	var meta VaultMeta
	if err := json.Unmarshal(metaBytes, &meta); err == nil {
		return meta.Passwordless
	}
	return false
}

func initVault(vaultPath string, password string) error {
	salt, err := generateSalt()
	if err != nil {
		return err
	}

	key := deriveKey(password, salt)
	hash := sha256.Sum256(key)

	meta := VaultMeta{
		Salt:         salt,
		Hash:         hash[:],
		Passwordless: password == "",
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	metaPath := filepath.Join(vaultPath, ".vault_meta")
	return os.WriteFile(metaPath, metaBytes, 0600)
}

func loadVault(vaultPath string, password string) ([]byte, bool, error) {
	metaPath := filepath.Join(vaultPath, ".vault_meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, false, err
	}

	var meta VaultMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, false, err
	}

	key := deriveKey(password, meta.Salt)
	hash := sha256.Sum256(key)

	if string(hash[:]) != string(meta.Hash) {
		return nil, false, nil // Invalid password
	}

	return key, true, nil
}

func changeVaultPassword(vaultPath string, oldPassword, newPassword string) error {
	// 1. load old key
	oldKey, valid, err := loadVault(vaultPath, oldPassword)
	if err != nil {
		return err
	}
	if !valid {
		return os.ErrPermission // use as invalid password error
	}

	// 2. generate new salt & new key
	newSalt, err := generateSalt()
	if err != nil {
		return err
	}
	newKey := deriveKey(newPassword, newSalt)

	// 3. re-encrypt all notes
	entries, err := os.ReadDir(vaultPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".enc") {
			notePath := filepath.Join(vaultPath, e.Name())
			content, err := os.ReadFile(notePath)
			if err != nil {
				return err
			}

			if len(content) > 0 {
				decrypted, err := decrypt(content, oldKey)
				if err != nil {
					return err
				}

				reencrypted, err := encrypt(decrypted, newKey)
				if err != nil {
					return err
				}

				if err := os.WriteFile(notePath, reencrypted, 0600); err != nil {
					return err
				}
			}
		}
	}

	// 4. save new meta
	hash := sha256.Sum256(newKey)
	meta := VaultMeta{
		Salt:         newSalt,
		Hash:         hash[:],
		Passwordless: newPassword == "",
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	metaPath := filepath.Join(vaultPath, ".vault_meta")
	return os.WriteFile(metaPath, metaBytes, 0600)
}
