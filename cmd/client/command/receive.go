package command

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/relationship-example/internal/node"
	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/spf13/cobra"
)

const (
	FlagRelationship = "r"
)

var commandReceive = &cobra.Command{
	Use:   "receive",
	Short: "Display receiving address/key. \"--r\" for relationship base key",
	RunE: func(c *cobra.Command, args []string) error {
		ctx := Context()

		envConfig, err := config.Environment()
		if err != nil {
			logger.Fatal(ctx, "Failed to get config : %s", err)
		}

		cfg, err := envConfig.Config()
		if err != nil {
			logger.Fatal(ctx, "Failed to convert config : %s", err)
		}

		var buf bytes.Buffer
		if _, err := buf.Write([]byte(node.CommandReceive)); err != nil {
			logger.Fatal(ctx, "Failed to write command name : %s", err)
		}

		t := wallet.KeyTypeExternal
		isRelationship, _ := c.Flags().GetBool(FlagRelationship)
		if isRelationship {
			t = wallet.KeyTypeRelateIn
		}
		if err := binary.Write(&buf, binary.LittleEndian, t); err != nil {
			logger.Fatal(ctx, "Failed to write type : %s", err)
		}

		response, err := node.SendCommand(ctx, cfg, buf.Bytes())
		if err != nil {
			logger.Fatal(ctx, "Failed to send command : %s", err)
		}

		if isError(response) {
			logger.Fatal(ctx, "Error Response : %s", string(response))
		}

		ra, err := bitcoin.DecodeRawAddress(response)
		if err != nil {
			logger.Fatal(ctx, "Failed to parse response : %s", err)
		}

		fmt.Printf("Receive Address : %s\n", bitcoin.NewAddressFromRawAddress(ra, cfg.Net).String())
		return nil
	},
}

func init() {
	commandReceive.Flags().Bool(FlagRelationship, false, "Relationship key")
}
