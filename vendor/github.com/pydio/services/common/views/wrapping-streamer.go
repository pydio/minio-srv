package views

import (
	"encoding/json"
	"github.com/pydio/services/common/proto/tree"
	"io"
)

type WrappingStreamer struct {
	w      *io.PipeWriter
	r      *io.PipeReader
	closed bool
}

func NewWrappingStreamer() *WrappingStreamer {
	r, w := io.Pipe()

	return &WrappingStreamer{
		w:      w,
		r:      r,
		closed: false,
	}
}

func (l *WrappingStreamer) Send(resp *tree.ListNodesResponse) error {
	enc := json.NewEncoder(l.w)
	enc.Encode(resp)
	return nil
}

func (l *WrappingStreamer) SendMsg(interface{}) error {
	return nil
}

func (l *WrappingStreamer) Recv() (*tree.ListNodesResponse, error) {
	if l.closed {
		return nil, io.EOF
	}
	resp := &tree.ListNodesResponse{}
	dec := json.NewDecoder(l.r)
	err := dec.Decode(resp)
	return resp, err
}

func (l *WrappingStreamer) RecvMsg(interface{}) error {
	return nil
}

func (l *WrappingStreamer) Close() error {
	l.closed = true
	l.w.Close()
	return nil
}
