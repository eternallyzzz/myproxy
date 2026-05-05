package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const maxPayloadSize = 16 * 1024 * 1024

func DePacket(r io.Reader) ([]byte, error) {
	var l int64
	if err := binary.Read(r, binary.BigEndian, &l); err != nil {
		return nil, err
	}

	if l < 0 || l > maxPayloadSize {
		return nil, errors.New("invalid packet length")
	}

	buf := make([]byte, l)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func EnPacket(data []byte) []byte {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, int64(len(data)))
	buffer.Write(data)
	return buffer.Bytes()
}
