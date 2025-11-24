package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"

	"golang.org/x/crypto/bcrypt"
)

// hashType lets to generate hash based on this value.
type hashType int

// List of hash types
const (
	HashSHA2 hashType = iota
	HashBCRYPT
)

// GenerateHash generates hash value (may used as token).
func GenerateHash(hashType hashType, values ...string) (hash string, err error) {
	switch hashType {
	case HashSHA2:
		return hashSha2(values...)
	case HashBCRYPT:
		return hashBcrypt(values...)
	}

	return "", errors.New("generateHash: unknown hashType")
}

// CompareInBcrypt compares bcrypt hashed value against plain string.
func CompareInBcrypt(hashed, plain string) bool {
	var (
		hashedBytes = []byte(hashed)
		plainBytes  = []byte(plain)
	)

	return nil == bcrypt.CompareHashAndPassword(hashedBytes, plainBytes)
}

// hashSha2 uses HMAC and SHA256 to generate hash.
func hashSha2(values ...string) (hash string, err error) {
	if len(values) < 2 {
		return "", errors.New("hashSha2: not enough arguments")
	}

	var (
		secretKey []byte
		value     []byte
		valueStr  string
	)

	for i, v := range values {
		if i == 0 {
			secretKey = []byte(v)
			continue
		}
		valueStr += v
	}
	value = []byte(valueStr)

	hashHmac := hmac.New(sha256.New, secretKey)
	hashHmac.Write(value)
	hash = hex.EncodeToString(hashHmac.Sum(nil))
	return
}

// hashBcrypt uses Bcrypt to generate hash.
// NOTE: the best choice for password hashing: https://rietta.com/blog/2016/02/05/bcrypt-not-sha-for-passwords/
func hashBcrypt(values ...string) (hash string, err error) {
	if len(values) != 1 {
		return "", errors.New("hashBcrypt: not enough arguments")
	}

	var (
		value = []byte(values[0])
		bytes []byte
	)

	bytes, err = bcrypt.GenerateFromPassword(value, bcrypt.MinCost)
	if err != nil {
		return
	}

	hash = string(bytes)
	return
}

var crc32q = crc32.MakeTable(0xA7329436)

func GenCRC32(val string) string {
	return fmt.Sprintf("%x", crc32.Checksum([]byte(val), crc32q))
}

func GetHMac256(data []byte, secret string) string {

	// Create a new HMAC by defining the hash type and the key (as byte array)
	h := hmac.New(sha256.New, []byte(secret))

	// Write Data to it
	h.Write(data)

	// Get result and encode as hexadecimal string
	sha := hex.EncodeToString(h.Sum(nil))

	return sha
}
