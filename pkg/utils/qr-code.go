package utils

import (
	"fmt"
	"os"

	"github.com/skip2/go-qrcode"
)

func QRCode(merchantID string, stolNumber int) (filename string, err error) {
	dir := "./statics/qr-code/" + merchantID
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	data := fmt.Sprintf("%s&%d", merchantID, stolNumber)

	qrCode, err := qrcode.New(data, qrcode.Highest)
	if err != nil {
		return "", err
	}

	filename = fmt.Sprintf("%s/%d.png", dir, stolNumber)

	err = qrCode.WriteFile(256, filename)
	if err != nil {
		return "", err
	}

	return filename, nil
}
