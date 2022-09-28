// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package keystorage provides the key storage implementation.
package keystorage

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/ProtonMail/gopenpgp/v2/helper"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xerrors"

	"github.com/cosi-project/runtime/api/key_storage"
)

// KeyStorage is a key storage that can be used to store and retrieve the master key.
//
//nolint:govet
type KeyStorage struct {
	mx         sync.Mutex
	underlying key_storage.Storage
}

// InitializeRnd sets the master key for the key storage, encrypts it using public key and stores it in slot id.
// It is similar to Initialize() but it generates a random master key.
func (ks *KeyStorage) InitializeRnd(reader io.Reader, slotID, slotPublicKey string) error {
	var masterKey [32]byte

	_, err := io.ReadFull(reader, masterKey[:])
	if err != nil {
		return err
	}

	return ks.Initialize(masterKey[:], slotID, slotPublicKey)
}

// Initialize sets the master key for the key storage, encrypts it using public key and stores it in slot id.
func (ks *KeyStorage) Initialize(masterKey []byte, slotID, slotPublicKey string) error {
	switch {
	case len(masterKey) != 32:
		return fmt.Errorf("master key can only be 32 bytes long")
	case slotID == "":
		return fmt.Errorf("slot id cannot be empty")
	case slotPublicKey == "":
		return fmt.Errorf("slot public key cannot be empty")
	}

	ks.mx.Lock()
	defer ks.mx.Unlock()

	if !isZero(&ks.underlying) {
		return xerrors.NewTaggedf[AlreadyInitializedTag]("key storage is already initialized")
	}

	encryptedSlot, err := helper.EncryptBinaryMessageArmored(slotPublicKey, masterKey)
	if err != nil {
		return xerrors.NewTaggedf[KeyEncryptionFailureTag]("failed to encrypt slot '%s': %w", slotID, err)
	}

	ks.underlying.StorageVersion = key_storage.StorageVersion_STORAGE_VERSION_1
	ks.underlying.KeySlots = map[string]*key_storage.KeySlot{
		slotID: {
			Algorithm:    key_storage.Algorithm_PGP_AES_GCM_256,
			EncryptedKey: []byte(encryptedSlot),
		},
	}
	ks.underlying.KeysHmacHash = ks.hashSlots(masterKey)

	return nil
}

// AddKeySlot creates a new master key slot with the given slot id and public key using previus slot and its private key.
// It cannot be used to update an existing key slot.
// It is required to call Initialize() or UnmarshalBinary() to initialize the key storage first.
func (ks *KeyStorage) AddKeySlot(newSlotID, newSlotPublicKey, oldSlotID, oldSlotPrivateKey string) error {
	switch {
	case newSlotID == "":
		return fmt.Errorf("new slot id cannot be empty")
	case newSlotPublicKey == "":
		return fmt.Errorf("new slot public key cannot be empty")
	}

	ks.mx.Lock()
	defer ks.mx.Unlock()

	if newSlot := ks.underlying.GetKeySlots()[newSlotID]; newSlot != nil {
		return xerrors.NewTaggedf[SlotAlreadyExists]("new slot '%s' already exists", newSlotID)
	}

	masterKey, err := ks.getKey(oldSlotID, oldSlotPrivateKey)
	if err != nil {
		return err
	}

	encryptedSlot, err := helper.EncryptBinaryMessageArmored(newSlotPublicKey, masterKey)
	if err != nil {
		return xerrors.NewTaggedf[KeyEncryptionFailureTag]("failed to encrypt slot '%s': %w", newSlotID, err)
	}

	ks.underlying.GetKeySlots()[newSlotID] = &key_storage.KeySlot{
		Algorithm:    key_storage.Algorithm_PGP_AES_GCM_256,
		EncryptedKey: []byte(encryptedSlot),
	}

	ks.underlying.KeysHmacHash = ks.hashSlots(masterKey)

	return nil
}

// GetMasterKey returns the attempts to decrypt the master key slot with the given private key and return the master key.
func (ks *KeyStorage) GetMasterKey(slotID, slotPrivateKey string) ([]byte, error) {
	ks.mx.Lock()
	defer ks.mx.Unlock()

	return ks.getKey(slotID, slotPrivateKey)
}

// DeleteKeySlot removes the master key slot with the given slot id.
func (ks *KeyStorage) DeleteKeySlot(slotID, slotPrivateKey string) error {
	ks.mx.Lock()
	defer ks.mx.Unlock()

	slots := ks.underlying.GetKeySlots()
	switch len(slots) {
	case 0:
		return xerrors.NewTagged[NotInitializedTag](errors.New("key storage is not initialized"))
	case 1:
		return xerrors.NewTagged[LastKeyTag](errors.New("cannot delete the last key slot"))
	}

	masterKey, err := ks.getKey(slotID, slotPrivateKey)
	if err != nil {
		return err
	}

	delete(slots, slotID)

	ks.underlying.KeysHmacHash = ks.hashSlots(masterKey)

	return nil
}

func (ks *KeyStorage) getKey(slotID string, slotPrivateKey string) ([]byte, error) {
	switch {
	case slotID == "":
		return nil, fmt.Errorf("slot id cannot be empty")
	case slotPrivateKey == "":
		return nil, fmt.Errorf("slot private key cannot be empty")
	case isZero(&ks.underlying):
		return nil, xerrors.NewTagged[NotInitializedTag](errors.New("key storage is not initialized, please call Initialize() first"))
	case ks.underlying.GetStorageVersion() != key_storage.StorageVersion_STORAGE_VERSION_1:
		return nil, xerrors.NewTagged[VersionMismatchTag](errors.New("key storage version mismatch"))
	}

	slot, ok := ks.underlying.GetKeySlots()[slotID]
	if !ok {
		return nil, xerrors.NewTaggedf[SlotNotFoundTag]("slot '%s' not found", slotID)
	}

	if slot.Algorithm != key_storage.Algorithm_PGP_AES_GCM_256 {
		return nil, xerrors.NewTaggedf[AlgorithmMismatchTag]("slot '%s' algorithm mismatch", slotID)
	}

	masterKey, err := helper.DecryptBinaryMessageArmored(slotPrivateKey, nil, string(slot.EncryptedKey))
	if err != nil {
		return nil, xerrors.NewTaggedf[KeyDecryptionFailureTag]("failed to decrypt slot '%s': %w", slotID, err)
	}

	if err := ks.verifyKeySlots(masterKey); err != nil {
		return nil, err
	}

	return masterKey, nil
}

// MarshalBinary implements the [encoding.BinaryMarshaler] interface.
func (ks *KeyStorage) MarshalBinary() (data []byte, err error) {
	ks.mx.Lock()
	defer ks.mx.Unlock()

	result, err := ks.underlying.MarshalVT()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key storage: %w", err)
	}

	return result, nil
}

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.
func (ks *KeyStorage) UnmarshalBinary(data []byte) error {
	ks.mx.Lock()
	defer ks.mx.Unlock()

	if err := ks.underlying.UnmarshalVT(data); err != nil {
		return fmt.Errorf("failed to unmarshal key storage: %w", err)
	}

	if ks.underlying.GetStorageVersion() != key_storage.StorageVersion_STORAGE_VERSION_1 {
		return xerrors.NewTaggedf[VersionMismatchTag]("key storage version mismatch")
	}

	return nil
}

func (ks *KeyStorage) verifyKeySlots(masterKey []byte) error {
	if subtle.ConstantTimeCompare(ks.hashSlots(masterKey), ks.underlying.GetKeysHmacHash()) == 0 {
		return xerrors.NewTaggedf[HMACMismatchTag]("key storage HMAC mismatch, please verify key storage integrity")
	}

	return nil
}

func (ks *KeyStorage) hashSlots(masterKey []byte) []byte {
	hash := hmac.New(sha256.New, masterKey)

	keySlots := ks.underlying.GetKeySlots()
	keys := maps.Keys(keySlots)
	sort.Strings(keys)

	for _, key := range keys {
		hash.Write(keySlots[key].EncryptedKey)
	}

	return hash.Sum(nil)
}

func isZero(underlying *key_storage.Storage) bool {
	return underlying.GetStorageVersion() == key_storage.StorageVersion_STORAGE_VERSION_UNSPECIFIED &&
		len(underlying.GetKeySlots()) == 0 &&
		len(underlying.GetKeysHmacHash()) == 0
}

type (
	// NotInitializedTag is the error tag returned when the key storage is not initialized.
	NotInitializedTag struct{}
	// AlreadyInitializedTag is the error tag returned when the key storage is already initialized.
	AlreadyInitializedTag struct{}
	// SlotAlreadyExists is the error tag returned when a key slot already exists.
	SlotAlreadyExists struct{}
	// SlotNotFoundTag is the error tag returned when a key slot is not found.
	SlotNotFoundTag struct{}
	// VersionMismatchTag is used to indicate that the key storage version mismatch error returned.
	VersionMismatchTag struct{}
	// HMACMismatchTag is used to indicate that the mismatch HMAC key storage error returned.
	HMACMismatchTag struct{}
	// AlgorithmMismatchTag is used to indicate that the algorithm mismatch error returned.
	AlgorithmMismatchTag struct{}
	// KeyDecryptionFailureTag is used to indicate that the master key decryption error returned.
	KeyDecryptionFailureTag struct{}
	// KeyEncryptionFailureTag is used to indicate that the master key encryption error returned.
	KeyEncryptionFailureTag struct{}
	// LastKeyTag is used to indicate that the last key slot error returned.
	LastKeyTag struct{}
)
