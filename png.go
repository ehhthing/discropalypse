package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// combination of the 8 byte png header with the 8 byte iHDR metadata
var header = []byte("\x89PNG\r\n\x1a\n\u0000\u0000\u0000\rIHDR")

func readDimensions(buf []byte) (uint32, uint32, error) {
	if !bytes.Equal(header, buf[0:16]) {
		return 0, 0, fmt.Errorf("not a png file, mismatched png header: %#v", string(buf[0:16]))
	}

	return binary.BigEndian.Uint32(buf[16:20]), binary.BigEndian.Uint32(buf[20:24]), nil
}
