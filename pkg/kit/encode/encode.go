package encode

import (
	"bytes"
	"encoding/binary"
	"io"
)

func Decode(r io.Reader) ([]byte, error) {
	var l int64
	if err := binary.Read(r, binary.BigEndian, &l); err != nil {
		return nil, err
	}

	buf := make([]byte, l)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func Encode(data []byte) []byte {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, int64(len(data)))
	buffer.Write(data)
	return buffer.Bytes()
}
