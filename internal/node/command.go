package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/tokenized/relationship-example/internal/platform/config"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

const (
	CommandReceive  = "rec"
	CommandInitiate = "ini"
)

func (n *Node) RunCommandServer(ctx context.Context) error {

	var err error
	n.netLock.Lock()
	n.netListener, err = net.Listen("tcp", fmt.Sprintf(":%d", n.cfg.CommandPort))
	n.netLock.Unlock()
	if err != nil {
		return errors.Wrap(err, "start listening")
	}

	logger.Info(ctx, "Listening for commands on port : %d", n.cfg.CommandPort)
	for {
		conn, err := n.netListener.Accept()
		if err != nil {
			return errors.Wrap(err, "net accept")
		}

		n.netLock.Lock()
		n.netConns = append(n.netConns, conn)
		n.netLock.Unlock()

		logger.Info(ctx, "Received command connection")
		go n.RunConnection(ctx, conn)

		// Check for stop request
		val := n.stop.Load()
		s, ok := val.(bool)
		if !ok || s {
			break
		}
	}

	return nil
}

func (n *Node) RunConnection(ctx context.Context, conn net.Conn) error {
	// TODO Check only localhost allowed

	for {
		command, err := readBytes(conn)
		if err != nil {
			return errors.Wrap(err, "receive response")
		}

		logger.Info(ctx, "Received command : %x", command)

		response, err := n.ProcessCommand(ctx, command)

		if err != nil {
			if err := writeBytes(conn, []byte("err: "+err.Error())); err != nil {
				return errors.Wrap(err, "send response error")
			}
		} else {
			if err := writeBytes(conn, response); err != nil {
				return errors.Wrap(err, "send response")
			}
		}

		// Check for stop request
		val := n.stop.Load()
		s, ok := val.(bool)
		if !ok || s {
			break
		}
	}

	logger.Info(ctx, "Finished command connection")
	return nil
}

func (n *Node) ProcessCommand(ctx context.Context, command []byte) ([]byte, error) {
	buf := bytes.NewReader(command)

	name := make([]byte, 3)
	if _, err := buf.Read(name); err != nil {
		return nil, errors.Wrap(err, "read name")
	}

	switch string(name) {
	case CommandReceive:
		var t uint32
		if err := binary.Read(buf, binary.LittleEndian, &t); err != nil {
			return nil, errors.Wrap(err, "read type")
		}

		ra, err := n.wallet.GetUnusedRawAddress(ctx, t)
		if err != nil {
			return nil, errors.Wrap(err, "get address")
		}

		return ra.Bytes(), nil

	case CommandInitiate:
		var ra bitcoin.RawAddress
		if err := ra.Deserialize(buf); err != nil {
			return nil, errors.Wrap(err, "deserialize address")
		}

		publicKey, err := ra.GetPublicKey()
		if err != nil {
			return nil, errors.Wrap(err, "get public key")
		}

		// TODO Add support for more receivers and proof of identity --ce
		initiate, err := n.rs.InitiateRelationship(ctx, []bitcoin.PublicKey{publicKey}, nil)
		if err != nil {
			return nil, errors.Wrap(err, "initiate relationship")
		}

		return initiate.Seed, nil
	}

	return nil, fmt.Errorf("Unknown command name : %s", string(name))
}

func SendCommand(ctx context.Context, cfg *config.Config, command []byte) ([]byte, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf(":%d", cfg.CommandPort))
	if err != nil {
		return nil, errors.Wrap(err, "dial")
	}

	if err := writeBytes(conn, command); err != nil {
		return nil, errors.Wrap(err, "send command")
	}

	logger.Info(ctx, "Sent command : %x", command)

	response, err := readBytes(conn)
	if err != nil {
		if errors.Cause(err) == io.EOF {
			return nil, errors.New("Connection closed")
		}
		return nil, errors.Wrap(err, "receive response")
	}

	logger.Info(ctx, "Received response : %x", response)

	if err := conn.Close(); err != nil {
		return nil, errors.Wrap(err, "close")
	}

	return response, nil
}

func writeBytes(w io.Writer, b []byte) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(b))); err != nil {
		return errors.Wrap(err, "write bytes length")
	}

	if len(b) == 0 {
		return nil
	}

	if _, err := w.Write(b); err != nil {
		return errors.Wrap(err, "write bytes")
	}

	return nil
}

func readBytes(r io.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, errors.Wrap(err, "read bytes length")
	}

	if size == 0 {
		return nil, nil
	}

	b := make([]byte, size)
	if _, err := r.Read(b); err != nil {
		return nil, errors.Wrap(err, "read bytes")
	}

	return b, nil
}
