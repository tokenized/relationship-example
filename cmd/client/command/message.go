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

var commandMessage = &cobra.Command{
	Use:   "message <relationship tx id> <text of message>",
	Short: "Send a message to the relationship that was initiated in the specified transaction.",
	RunE: func(c *cobra.Command, args []string) error {
		ctx := Context()

		if len(args) != 2 {
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

		txid, err := bitcoin.NewHash32FromStr(args[0])
		if err != nil {
			logger.Fatal(ctx, "Failed to parse txid : %s", err)
		}

		var buf bytes.Buffer
		if _, err := buf.Write([]byte(node.CommandMessage)); err != nil {
			logger.Fatal(ctx, "Failed to write command name : %s", err)
		}

		if err := txid.Serialize(&buf); err != nil {
			logger.Fatal(ctx, "Failed to write txid : %s", err)
		}

		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(args[1]))); err != nil {
			logger.Fatal(ctx, "Failed to write message length : %s", err)
		}

		if _, err := buf.Write([]byte(args[1])); err != nil {
			logger.Fatal(ctx, "Failed to write message : %s", err)
		}

		response, err := node.SendCommand(ctx, cfg, buf.Bytes())
		if err != nil {
			logger.Fatal(ctx, "Failed to send command : %s", err)
		}

		if t, m := isError(response); t {
			logger.Fatal(ctx, "Error Response : %s", m)
		}

		fmt.Printf("%s\n", string(response))
		return nil
	},
}
