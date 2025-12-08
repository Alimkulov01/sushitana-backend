package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"
)

func ClickAuthHeader(merchantUserID, secretKey string) (string, string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	h := sha1.New()
	h.Write([]byte(timestamp + secretKey))
	digest := hex.EncodeToString(h.Sum(nil))

	auth := fmt.Sprintf("%s:%s:%s", merchantUserID, digest, timestamp)

	return auth, timestamp
}
