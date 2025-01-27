package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/tokenized/relationship-example/internal/platform/config"

	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

const (
	CommandReceive  = "rec"
	CommandInitiate = "ini"
	CommandAccept   = "acc"
	CommandMessage  = "mes"
	CommandList     = "lst"
)

func (n *Node) RunCommandServer(ctx context.Context) error {

	address, err := net.ResolveUnixAddr("unix", n.cfg.CommandPath)
	if err != nil {
		return errors.Wrap(err, "resolve address")
	}

	listener, err := net.ListenUnix("unix", address)
	if err != nil {
		return errors.Wrap(err, "start listening")
	}

	n.netLock.Lock()
	n.netListener = net.Listener(listener)
	n.netLock.Unlock()

	logger.Info(ctx, "Listening for commands at path : %s", n.cfg.CommandPath)
	for {
		listener.SetDeadline(time.Now().Add(1e9))
		conn, err := listener.AcceptUnix()
		if err != nil {
			if opErr, isOpErr := err.(*net.OpError); isOpErr && opErr.Timeout() {
				// Check for stop request
				val := n.stop.Load()
				s, ok := val.(bool)
				if !ok || s {
					break
				}
				continue
			}
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
		var count uint32
		if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
			return nil, errors.Wrap(err, "member count")
		}

		members := make([]bitcoin.PublicKey, 0, count)
		for i := uint32(0); i < count; i++ {
			var ra bitcoin.RawAddress
			if err := ra.Deserialize(buf); err != nil {
				return nil, errors.Wrap(err, "deserialize address")
			}

			publicKey, err := ra.GetPublicKey()
			if err != nil {
				return nil, errors.Wrap(err, "get public key")
			}

			members = append(members, publicKey)
		}

		// TODO Add support for proof of identity --ce
		txid, _, err := n.rs.InitiateRelationship(ctx, members, nil)
		if err != nil {
			return nil, errors.Wrap(err, "initiate relationship")
		}

		return txid.Bytes(), nil

	case CommandAccept:
		var txid bitcoin.Hash32
		if err := txid.Deserialize(buf); err != nil {
			return nil, errors.Wrap(err, "deserialize txid")
		}

		r := n.rs.FindRelationshipForTxId(ctx, txid)
		if r == nil {
			return nil, errors.New("Relationship not found")
		}

		_, err := n.rs.AcceptRelationship(ctx, r, nil)
		if err != nil {
			return nil, errors.Wrap(err, "accept relationship")
		}

		return []byte("Accept Sent"), nil

	case CommandMessage:
		var txid bitcoin.Hash32
		if err := txid.Deserialize(buf); err != nil {
			return nil, errors.Wrap(err, "deserialize txid")
		}

		r := n.rs.FindRelationshipForTxId(ctx, txid)
		if r == nil {
			return nil, errors.New("Relationship not found")
		}

		var size uint32
		if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
			return nil, errors.Wrap(err, "read message size")
		}

		b := make([]byte, size)
		if _, err := buf.Read(b); err != nil {
			return nil, errors.Wrap(err, "read message")
		}

		// Plain text message
		message := &messages.PrivateMessage{
			PrivateMessage: &messages.DocumentField{
				Type:     "text/plain",
				Contents: b,
			},
		}

		if err := n.rs.SendMessage(ctx, r, message); err != nil {
			return nil, errors.Wrap(err, "send message")
		}

		return []byte("Message Sent"), nil

	case CommandList:
		l := n.rs.ListRelationships(ctx)

		var buf bytes.Buffer
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(l))); err != nil {
			return nil, errors.Wrap(err, "write relationship count")
		}

		for _, r := range l {
			if _, err := buf.Write(r.TxId[:]); err != nil {
				return nil, errors.Wrap(err, "write relationship")
			}
		}

		return buf.Bytes(), nil
	}

	return nil, fmt.Errorf("Unknown command name : %s", string(name))
}

func SendCommand(ctx context.Context, cfg *config.Config, command []byte) ([]byte, error) {
	address, err := net.ResolveUnixAddr("unix", cfg.CommandPath)
	if err != nil {
		return nil, errors.Wrap(err, "resolve address")
	}

	conn, err := net.DialUnix("unix", nil, address)
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
