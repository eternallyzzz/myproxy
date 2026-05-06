package packet

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

const maxPayloadSize = 16 * 1024 * 1024

var (
	xorKey  byte
	padding bool
)

func InitObfuscation(xorKeyString string, enablePadding bool) {
	if xorKeyString != "" {
		for _, b := range []byte(xorKeyString) {
			xorKey ^= b
		}
	}
	padding = enablePadding
}

func DePacket(r io.Reader) ([]byte, error) {
	var l int64
	if err := binary.Read(r, binary.BigEndian, &l); err != nil {
		return nil, err
	}

	l ^= int64(xorKey)

	if l < 0 || l > maxPayloadSize {
		return nil, errors.New("invalid packet length")
	}

	buf := make([]byte, l)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	if padding {
		var padLen uint8
		if err := binary.Read(r, binary.BigEndian, &padLen); err != nil {
			return nil, err
		}
		if padLen > 0 {
			discard := make([]byte, padLen)
			_, err = io.ReadFull(r, discard)
			if err != nil {
				return nil, err
			}
		}
	}

	return buf, nil
}

func EnPacket(data []byte) []byte {
	buffer := bytes.NewBuffer(nil)
	l := int64(len(data)) ^ int64(xorKey)
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write(data)

	if padding {
		padLen := pad()
		if padLen > 0 {
			padBuf := make([]byte, padLen)
			_, _ = rand.Read(padBuf)
			binary.Write(buffer, binary.BigEndian, uint8(padLen))
			buffer.Write(padBuf)
		}
	}

	return buffer.Bytes()
}

func pad() int {
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	return int(b[0])
}
