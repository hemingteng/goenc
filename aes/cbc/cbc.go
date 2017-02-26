package cbc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"

	"io"

	"github.com/alistanis/goenc/encerrors"
	"github.com/alistanis/goenc/generate"
)

const (
	NonceSize = aes.BlockSize
	MACSize   = 32 // Output size of HMAC-SHA-256
	CKeySize  = 32 // Cipher key size - AES-256
	MKeySize  = 32 // HMAC key size - HMAC-SHA-256
	KeySize   = CKeySize + MKeySize
)

func pad(in []byte) []byte {
	padding := 16 - (len(in) % 16)
	for i := 0; i < padding; i++ {
		in = append(in, byte(padding))
	}
	return in
}

func unpad(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}

	padding := in[len(in)-1]
	if int(padding) > len(in) || padding > aes.BlockSize {
		return nil
	} else if padding == 0 {
		return nil
	}

	for i := len(in) - 1; i > len(in)-int(padding)-1; i-- {
		if in[i] != padding {
			return nil
		}
	}
	return in[:len(in)-int(padding)]
}

type Cipher struct{}

func (c *Cipher) Encrypt(key, plaintext []byte) ([]byte, error) {
	return Encrypt(key, plaintext)
}

func (c *Cipher) Decrypt(key, ciphertext []byte) ([]byte, error) {
	return Decrypt(key, ciphertext)
}

func (c *Cipher) KeySize() int {
	return KeySize
}

func New() *Cipher {
	return &Cipher{}
}

func Key() (*[KeySize]byte, error) {
	key := new([KeySize]byte)
	_, err := io.ReadFull(rand.Reader, key[:])
	return key, err
}

func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, encerrors.ErrInvalidKeyLength
	}

	iv, err := generate.RandBytes(NonceSize)
	if err != nil {
		return nil, err
	}

	pmessage := pad(plaintext)
	ct := make([]byte, len(pmessage))

	// NewCipher only returns an error with an invalid key size,
	// but the key size was checked at the beginning of the function.
	c, _ := aes.NewCipher(key[:CKeySize])
	ctr := cipher.NewCBCEncrypter(c, iv)
	ctr.CryptBlocks(ct, pmessage)

	h := hmac.New(sha256.New, key[CKeySize:])
	ct = append(iv, ct...)
	h.Write(ct)
	ct = h.Sum(ct)
	return ct, nil
}

func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, encerrors.ErrInvalidKeyLength
	}

	// HMAC-SHA-256 returns a MAC that is also a multiple of the
	// block size.
	if (len(ciphertext) % aes.BlockSize) != 0 {
		return nil, encerrors.ErrInvalidMessageLength
	}

	// A ciphertext must have at least an IV block, a ciphertext block,
	// and two blocks of HMAC.
	if len(ciphertext) < (4 * aes.BlockSize) {
		return nil, encerrors.ErrInvalidMessageLength
	}

	macStart := len(ciphertext) - MACSize
	tag := ciphertext[macStart:]
	out := make([]byte, macStart-NonceSize)
	ciphertext = ciphertext[:macStart]

	h := hmac.New(sha256.New, key[CKeySize:])
	h.Write(ciphertext)
	mac := h.Sum(nil)
	if !hmac.Equal(mac, tag) {
		return nil, encerrors.ErrInvalidSum
	}

	// NewCipher only returns an error with an invalid key size,
	// but the key size was checked at the beginning of the function.
	c, _ := aes.NewCipher(key[:CKeySize])
	ctr := cipher.NewCBCDecrypter(c, ciphertext[:NonceSize])
	ctr.CryptBlocks(out, ciphertext[NonceSize:])

	pt := unpad(out)
	if pt == nil {
		return nil, encerrors.ErrInvalidPadding
	}

	return pt, nil
}