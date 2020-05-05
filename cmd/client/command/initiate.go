package command

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/relationship-example/internal/node"
	"github.com/tokenized/relationship-example/internal/platform/config"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/spf13/cobra"
)

var commandInitiate = &cobra.Command{
	Use:   "initiate <public key address> ...",
	Short: "Initiate a relationship with the public key addresses specified.",
	RunE: func(c *cobra.Command, args []string) error {
		ctx := Context()

		if len(args) < 1 {
			c.Help()
			logger.Fatal(ctx, "Wrong number of arguments")
		}

		envConfig, err := config.Environment()
		if err != nil {
			logger.Fatal(ctx, "Failed to get config : %s", err)
		}

		cfg, err := envConfig.Config()
		if err != nil {
			logger.Fatal(ctx, "Failed to convert config : %s", err)
		}

		var buf bytes.Buffer
		if _, err := buf.Write([]byte(node.CommandInitiate)); err != nil {
			logger.Fatal(ctx, "Failed to write command name : %s", err)
		}

		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(args))); err != nil {
			logger.Fatal(ctx, "Failed to write member count : %s", err)
		}

		for _, arg := range args {
			ad, err := bitcoin.DecodeAddress(arg)
			if err != nil {
				logger.Fatal(ctx, "Failed to parse address : %s", err)
			}

			ra := bitcoin.NewRawAddressFromAddress(ad)
			if _, err := buf.Write(ra.Bytes()); err != nil {
				logger.Fatal(ctx, "Failed to write raw address : %s", err)
			}
		}

		response, err := node.SendCommand(ctx, cfg, buf.Bytes())
		if err != nil {
			logger.Fatal(ctx, "Failed to send command : %s", err)
		}

		if t, m := isError(response); t {
			logger.Fatal(ctx, "Error Response : %s", m)
		}

		txid, err := bitcoin.NewHash32(response)
		if err != nil {
			logger.Fatal(ctx, "Failed to create txid : %s", err)
		}

		fmt.Printf("Relationship created with txid : %s\n", txid.String())
		return nil
	},
}
