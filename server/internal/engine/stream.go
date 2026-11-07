package engine

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

// Demux decodes Docker's multiplexed stdout/stderr frame format. When the
// payload is not framed (TTY mode), it is returned unchanged.
func Demux(raw []byte) string {
	var out bytes.Buffer
	r := NewDemuxReader(bytes.NewReader(raw))
	_, _ = io.Copy(&out, r)
	if out.Len() == 0 && len(raw) > 0 {
		return string(raw)
	}
	return out.String()
}

// DemuxReader strips the 8-byte docker frame headers from a stream and
// falls back to raw passthrough when the stream is not framed.
type DemuxReader struct {
	r         *bufio.Reader
	rawMode   bool
	checked   bool
	remaining int
	header    [8]byte
}

func NewDemuxReader(r io.Reader) *DemuxReader {
	return &DemuxReader{r: bufio.NewReaderSize(r, 32*1024)}
}

func (d *DemuxReader) Read(p []byte) (int, error) {
	for {
		if !d.checked {
			peek, err := d.r.Peek(8)
			if err != nil {
				// too short to be framed -> raw
				return d.r.Read(p)
			}
			frameType := peek[0]
			size := binary.BigEndian.Uint32(peek[4:8])
			if frameType <= 2 && size <= 32*1024*1024 {
				d.checked = true
			} else {
				d.rawMode = true
				d.checked = true
			}
		}
		if d.rawMode {
			return d.r.Read(p)
		}
		if d.remaining == 0 {
			if _, err := io.ReadFull(d.r, d.header[:]); err != nil {
				return 0, err
			}
			d.remaining = int(binary.BigEndian.Uint32(d.header[4:8]))
			if d.remaining == 0 {
				continue
			}
		}
		n := len(p)
		if n > d.remaining {
			n = d.remaining
		}
		read, err := d.r.Read(p[:n])
		d.remaining -= read
		if err != nil {
			return read, err
		}
		return read, nil
	}
}
