// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package encryption provides encryption support for [store.Marshaler].
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

// Marshaler encrypts and decrypts data from the underlying marshaler.
type Marshaler struct {
	underlying store.Marshaler
	cipher     *Cipher
}

// NewMarshaler creates new Marshaler.
func NewMarshaler(m store.Marshaler, c *Cipher) *Marshaler {
	return &Marshaler{underlying: m, cipher: c}
}

// MarshalResource implements Marshaler interface.
func (m *Marshaler) MarshalResource(r resource.Resource) ([]byte, error) {
	encoded, err := m.underlying.MarshalResource(r)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource: %w", err)
	}

	encrypted, err := m.cipher.Encrypt(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt resource: %w", err)
	}

	return encrypted, nil
}

// UnmarshalResource implements Marshaler interface.
func (m *Marshaler) UnmarshalResource(b []byte) (resource.Resource, error) { //nolint:ireturn
	decrypted, err := m.cipher.Decrypt(b)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt resource: %w", err)
	}

	return m.underlying.UnmarshalResource(decrypted)
}

// Cipher provides encryption and decryption.
type Cipher struct {
	cipher func() (cipher.AEAD, error)
}

// NewCipher creates new Cipher.
func NewCipher(provider KeyProvider) *Cipher {
	return &Cipher{
		cipher: sync.OnceValues(func() (cipher.AEAD, error) {
			// According to https://github.com/golang/go/issues/25882 cipher.AEAD is safe to share between goroutines.
			key, err := provider.ProvideKey()
			if err != nil {
				return nil, fmt.Errorf("failed to provide key: %w", err)
			}

			if len(key) != 32 {
				return nil, fmt.Errorf("key length is not 32 bytes")
			}

			block, err := aes.NewCipher(key)
			if err != nil {
				return nil, fmt.Errorf("failed to create cipher: %w", err)
			}

			aead, err := cipher.NewGCM(block)
			if err != nil {
				return nil, fmt.Errorf("failed to create GCM: %w", err)
			}

			return aead, nil
		}),
	}
}

// Encrypt encrypts data.
func (c *Cipher) Encrypt(b []byte) ([]byte, error) {
	aead, err := c.cipher()
	if err != nil {
		return nil, fmt.Errorf("failed to init cipher: %w", err)
	}

	slc := make([]byte, 13)
	slc[0] = 1

	nonce := slc[1:]
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// We attach nonce before the encrypted data
	encrypted := aead.Seal(slc, nonce, b, nil)

	return encrypted, nil
}

// Decrypt decrypts data.
func (c *Cipher) Decrypt(b []byte) ([]byte, error) {
	aead, err := c.cipher()
	if err != nil {
		return nil, fmt.Errorf("failed to init cipher: %w", err)
	}

	if len(b) < 13+1 {
		return nil, fmt.Errorf("encrypted data is too short")
	}

	if b[0] != 1 {
		return nil, fmt.Errorf("unknown data format")
	}

	nonce := b[1:13]
	encrypted := b[13:]

	decrypted, err := aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open failed: %w", err)
	}

	return decrypted, nil
}

// KeyProvider provides the encryption key.
type KeyProvider interface {
	ProvideKey() ([]byte, error)
}

// KeyProviderFunc is a function that provides the encryption key.
type KeyProviderFunc func() ([]byte, error)

// ProvideKey implements KeyProvider interface.
func (f KeyProviderFunc) ProvideKey() ([]byte, error) {
	return f()
}
