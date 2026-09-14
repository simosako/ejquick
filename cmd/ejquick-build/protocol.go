package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/simosako/ejquick/internal/buildprotocol"
)

const maxProtocolLine = buildprotocol.MaxLineBytes

type commandReader struct {
	reader *bufio.Reader
}

func newCommandReader(r io.Reader) *commandReader {
	return &commandReader{reader: bufio.NewReaderSize(r, maxProtocolLine+1)}
}

func (r *commandReader) next() (protocolCommand, error) {
	var command protocolCommand
	line, err := r.reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) || len(line) > maxProtocolLine {
		return command, errors.New("control line exceeds 64 KiB")
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return command, err
	}
	if errors.Is(err, io.EOF) && len(line) == 0 {
		return command, io.EOF
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	if len(line) == 0 {
		return command, errors.New("empty control line")
	}
	if err := json.Unmarshal(line, &command); err != nil {
		return command, fmt.Errorf("decode control command: %w", err)
	}
	if command.Protocol != machineProtocolVersion {
		return command, fmt.Errorf("unsupported protocol %d", command.Protocol)
	}
	if command.Command == "" {
		return command, errors.New("control command is missing command")
	}
	return command, nil
}
