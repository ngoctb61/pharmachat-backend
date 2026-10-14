package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	s, err := NewSealer(key(1))
	require.NoError(t, err)

	sealed, err := s.Seal("sk-ant-secret")
	require.NoError(t, err)
	assert.NotContains(t, sealed, "sk-ant-secret")

	again, err := s.Seal("sk-ant-secret")
	require.NoError(t, err)
	assert.NotEqual(t, sealed, again, "each seal uses a fresh nonce")

	got, err := s.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-secret", got)
}

func TestOpenRejectsTamperingAndWrongKey(t *testing.T) {
	s, err := NewSealer(key(1))
	require.NoError(t, err)
	other, err := NewSealer(key(2))
	require.NoError(t, err)
	sealed, err := s.Seal("secret")
	require.NoError(t, err)

	_, err = other.Open(sealed)
	assert.ErrorIs(t, err, ErrDecrypt)

	tampered := []byte(sealed)
	tampered[len(tampered)-2] ^= 'A' ^ 'B'
	_, err = s.Open(string(tampered))
	assert.ErrorIs(t, err, ErrDecrypt)

	_, err = s.Open("not base64!")
	assert.ErrorIs(t, err, ErrDecrypt)
	_, err = s.Open("")
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestNewSealerRejectsShortKey(t *testing.T) {
	_, err := NewSealer(make([]byte, 16))
	assert.Error(t, err)
}
