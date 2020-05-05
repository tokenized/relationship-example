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

var commandList = &cobra.Command{
	Use:   "list",
	Short: "Lists the relationships available.",
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
		if _, err := buf.Write([]byte(node.CommandList)); err != nil {
			logger.Fatal(ctx, "Failed to write command name : %s", err)
		}

		response, err := node.SendCommand(ctx, cfg, buf.Bytes())
		if err != nil {
			logger.Fatal(ctx, "Failed to send command : %s", err)
		}

		if t, m := isError(response); t {
			logger.Fatal(ctx, "Error Response : %s", m)
		}

		var count uint32
		read := bytes.NewReader(response)
		if err := binary.Read(read, binary.LittleEndian, &count); err != nil {
			logger.Fatal(ctx, "Failed to read relationships count : %s", err)
		}

		fmt.Printf("List : \n")
		for i := uint32(0); i < count; i++ {
			var txid bitcoin.Hash32
			if err := txid.Deserialize(read); err != nil {
				logger.Fatal(ctx, "Failed to read relationship : %s", err)
			}
			fmt.Printf("  %s\n", txid.String())
		}

		return nil
	},
}
