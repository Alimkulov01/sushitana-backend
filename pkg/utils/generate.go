package utils

import (
	"errors"
	"math/rand"
	"strings"
	"unicode"

	"github.com/segmentio/ksuid"
)

func GenKSUID() string {
	return ksuid.New().String()
}

// remove only space symbols
func RemoveSpaceSymbol(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// GeneratePassword generates random password with length given in args
func GeneratePassword(length int) (password string, err error) {
	symbols := "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	if length < 1 {
		err = errors.New("length of password must be positive number")
		return
	}

	for i := 0; i < length; i++ {
		password += string(symbols[rand.Intn(len(symbols))])
	}
	return
}
