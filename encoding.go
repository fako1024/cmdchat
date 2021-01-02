package cmdchat

import (
	"strings"

	"github.com/valyala/gozstd"
)

// EncodeMessage encodes a message
func EncodeMessage(msg string) []byte {

	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	var buf []byte
	buf = gozstd.Compress(buf[:0], []byte(strings.ToValidUTF8(msg, "")))

	return buf
}

// DecodeMessage decodes a message
func DecodeMessage(data []byte) (string, error) {

	var (
		buf []byte
		err error
	)
	buf, err = gozstd.Decompress(buf[:0], data)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}
