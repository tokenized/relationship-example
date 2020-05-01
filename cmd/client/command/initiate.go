package command

import (
	"bytes"
	"fmt"

	"github.com/tokenized/relationship-example/internal/node"
	"github.com/tokenized/relationship-example/internal/platform/config"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/spf13/cobra"
)

var commandInitiate = &cobra.Command{
	Use:   "initiate <public key address>",
	Short: "Initiate a relationship with the public key address specified.",
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
		if _, err := buf.Write([]byte(node.CommandInitiate)); err != nil {
			logger.Fatal(ctx, "Failed to write command name : %s", err)
		}

		ad, err := bitcoin.DecodeAddress(args[0])
		if err != nil {
			logger.Fatal(ctx, "Failed to parse address : %s", err)
		}

		ra := bitcoin.NewRawAddressFromAddress(ad)
		if _, err := buf.Write(ra.Bytes()); err != nil {
			logger.Fatal(ctx, "Failed to write raw address : %s", err)
		}

		response, err := node.SendCommand(ctx, cfg, buf.Bytes())
		if err != nil {
			logger.Fatal(ctx, "Failed to send command : %s", err)
		}

		if isError(response) {
			logger.Fatal(ctx, "Error Response : %s", string(response))
		}

		fmt.Printf("Relationship created with flag : %x\n", response)
		return nil
	},
}
