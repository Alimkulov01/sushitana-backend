package utils

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

func RemoveSpecialChars2(s string) string {
	r := strings.NewReplacer("\t", "", "\n", " ", "\r", "", "\x00", "")
	return r.Replace(s)
}

func IntToStr(value int) string {
	return strconv.Itoa(value)
}

func StrToInt(value string) int {
	val, _ := strconv.Atoi(value)
	return val
}

func StrEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func BodyToJson(r *http.Request, value interface{}) error {
	return json.NewDecoder(r.Body).Decode(&value)
}

func InArray[T comparable](val T, array []T) bool {
	for _, v := range array {
		if val == v {
			return true
		}
	}
	return false
}

func StripSpace(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}

func Increment(i *int) int {
	*i++
	return *i
}

// +998991234567
func ValidPhone(phone string) bool {
	if len(phone) != 13 {
		return false
	}
	if phone[0:4] != "+998" {
		return false
	}
	if _, err := strconv.Atoi(phone[1:]); err != nil {
		return false
	}
	return true
}

func Marshal(value interface{}) []byte {
	byt, _ := json.Marshal(value)
	return byt
}

func Unmarshal(data []byte, value interface{}) error {
	err := json.Unmarshal(data, &value)
	return err
}

var Layout = "2006-01-02 15:04:05"
var DateLayout = "02.01.2006" // DD.MM.YYYY
