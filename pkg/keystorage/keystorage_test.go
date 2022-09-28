// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keystorage_test

import (
	_ "embed"
	"math/rand"
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/keystorage"
)

var (
	//go:embed testdata/private.key
	privateKey string

	//go:embed testdata/private.key
	publicKey string
)

const (
	masterKey = "this key len is exactly 32 bytes"
	slotID    = "slot-id"
)

func TestKeyStorage_Initialize(t *testing.T) {
	t.Parallel()

	type args struct {
		slotID        string
		slotPublicKey string
		masterKey     []byte
	}

	tests := map[string]struct {
		testErr func(*testing.T, error)
		args    args
	}{
		"empty master key": {
			args: args{
				masterKey:     []byte{},
				slotID:        slotID,
				slotPublicKey: "slot-public-key",
			},
			testErr: regexpTest("master key can only be 32 bytes long"),
		},
		"empty slot id": {
			args: args{
				masterKey:     []byte(masterKey),
				slotID:        "",
				slotPublicKey: "slot-public-key",
			},
			testErr: regexpTest("slot id cannot be empty"),
		},
		"empty slot public key": {
			args: args{
				masterKey:     []byte(masterKey),
				slotID:        slotID,
				slotPublicKey: "",
			},
			testErr: regexpTest("slot public key cannot be empty"),
		},
		"small public key": {
			args: args{
				masterKey:     []byte(masterKey),
				slotID:        slotID,
				slotPublicKey: publicKey[:32],
			},
			testErr: tagTest[keystorage.KeyEncryptionFailureTag](),
		},
		"proper key": {
			args: args{
				masterKey:     []byte(masterKey),
				slotID:        slotID,
				slotPublicKey: publicKey,
			},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var ks keystorage.KeyStorage

			if err := ks.Initialize(tt.args.masterKey, tt.args.slotID, tt.args.slotPublicKey); err != nil || tt.testErr != nil {
				tt.testErr(t, err)
			}
		})
	}
}

func TestKeyStorage_Inititialize_Complete(t *testing.T) {
	t.Parallel()

	var ks keystorage.KeyStorage

	require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))

	err := ks.Initialize([]byte(masterKey), slotID, publicKey)

	require.True(t, xerrors.TagIs[keystorage.AlreadyInitializedTag](err))
}

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	var ks keystorage.KeyStorage

	require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))

	binary, err := ks.MarshalBinary()
	require.NoError(t, err)

	var newStore keystorage.KeyStorage

	require.NoError(t, newStore.UnmarshalBinary(binary))

	key, err := newStore.GetMasterKey(slotID, privateKey)
	require.NoError(t, err)

	ksKey, err := ks.GetMasterKey(slotID, privateKey)
	require.NoError(t, err)

	require.Equal(t, ksKey, key)
}

func TestKeyStorage_DeleteMasterKeySlot(t *testing.T) {
	type args struct {
		slotID         string
		slotPrivateKey string
	}

	// execution order is important here
	tests := []struct {
		name    string
		testErr func(*testing.T, error)
		args    args
	}{
		{
			name:    "non-existing slot",
			testErr: tagTest[keystorage.SlotNotFoundTag](),
			args: args{
				slotID:         "slot-id-3",
				slotPrivateKey: privateKey,
			},
		},
		{
			name: "existing slot",
			args: args{
				slotID:         "slot-id-2",
				slotPrivateKey: privateKey,
			},
		},
		{
			name: "proper last slot",
			args: args{
				slotID:         "slot-id",
				slotPrivateKey: privateKey,
			},
			testErr: tagTest[keystorage.LastKeyTag](),
		},
	}

	var ks keystorage.KeyStorage

	require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))
	require.NoError(t, ks.AddKeySlot("slot-id-2", privateKey, slotID, privateKey))

	require.True(t, xerrors.TagIs[keystorage.SlotAlreadyExists](ks.AddKeySlot("slot-id-2", privateKey, slotID, privateKey)))

	for _, tt := range tests {
		if !t.Run(tt.name, func(t *testing.T) {
			if err := ks.DeleteKeySlot(tt.args.slotID, tt.args.slotPrivateKey); err != nil || tt.testErr != nil {
				tt.testErr(t, err)
			}
		}) {
			t.FailNow()
		}
	}
}

func TestMarshalUnmarshalIncorrectHmac(t *testing.T) {
	t.Parallel()

	var ks keystorage.KeyStorage

	require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))

	binary, err := ks.MarshalBinary()
	require.NoError(t, err)

	setSlice(binary, -4, 0, 0, 0, 0)

	var newStore keystorage.KeyStorage

	require.NoError(t, newStore.UnmarshalBinary(binary))

	_, err = newStore.GetMasterKey(slotID, privateKey)
	require.True(t, xerrors.TagIs[keystorage.HMACMismatchTag](err))
}

func TestKeyStorage_Get(t *testing.T) {
	t.Parallel()
	t.Run("not initialized", func(t *testing.T) {
		t.Parallel()

		var ks keystorage.KeyStorage

		_, err := ks.GetMasterKey(slotID, privateKey)
		require.True(t, xerrors.TagIs[keystorage.NotInitializedTag](err))
	})
	t.Run("slot not found", func(t *testing.T) {
		t.Parallel()

		var ks keystorage.KeyStorage

		require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))

		_, err := ks.GetMasterKey("slot-id-2", privateKey)
		require.True(t, xerrors.TagIs[keystorage.SlotNotFoundTag](err))
	})
}

func TestKeyStorage_Set(t *testing.T) {
	t.Parallel()

	type args struct {
		slotID        string
		slotPublicKey string
		newSlotID     string
	}

	tests := map[string]struct {
		testErr func(*testing.T, error)
		args    args
	}{
		"empty slot id": {
			args: args{
				slotID:        "",
				slotPublicKey: "slot-public-key",
				newSlotID:     "new-slot-id",
			},
			testErr: regexpTest("slot id cannot be empty"),
		},
		"empty new slot id": {
			args: args{
				slotID:        slotID,
				slotPublicKey: "slot-public-key",
				newSlotID:     "",
			},
			testErr: regexpTest("slot id cannot be empty"),
		},
		"empty slot public key": {
			args: args{
				slotID:        slotID,
				slotPublicKey: "",
				newSlotID:     "new-slot-id",
			},
			testErr: regexpTest("slot public key cannot be empty"),
		},
		"small public key": {
			args: args{
				slotID:        slotID,
				slotPublicKey: publicKey[:32],
				newSlotID:     "new-slot-id",
			},
			testErr: tagTest[keystorage.KeyDecryptionFailureTag](),
		},
		"proper key": {
			args: args{
				slotID:        slotID,
				slotPublicKey: publicKey,
				newSlotID:     "new-slot-id",
			},
		},
	}

	for name, tt := range tests {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var ks keystorage.KeyStorage

			require.NoError(t, ks.Initialize([]byte(masterKey), slotID, publicKey))

			err := ks.AddKeySlot(tt.args.newSlotID, tt.args.slotPublicKey, tt.args.slotID, tt.args.slotPublicKey)
			if err != nil || tt.testErr != nil {
				tt.testErr(t, err)
			}
		})
	}
}

func TestKeyStorage_InitializeRnd(t *testing.T) {
	var ks keystorage.KeyStorage

	r := rand.New(rand.NewSource(42))
	require.NoError(t, ks.InitializeRnd(r, slotID, publicKey))
}

func setSlice[T any](s []T, i int, v ...T) {
	if i < 0 {
		i = len(s) + i
	}

	copy(s[i:i+len(v)], v)
}

func regexpTest(re string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		require.Error(t, err)
		require.NotZero(t, re)
		require.Regexp(t, re, err.Error())
	}
}

func tagTest[T xerrors.Tag]() func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		require.Error(t, err)
		require.True(t, xerrors.TagIs[T](err))
	}
}
