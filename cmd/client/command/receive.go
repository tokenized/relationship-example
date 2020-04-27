package command

import (
	"fmt"

	"github.com/tokenized/relationship-example/pkg/client"

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

		client, err := client.NewClient(ctx)
		if err != nil {
			logger.Fatal(ctx, "Failed to create client : %s", err)
		}

		isRelationship, _ := c.Flags().GetBool(FlagRelationship)
		if isRelationship {
			ra, err := client.Wallet.GetRelationshipAddress(ctx)
			if err != nil {
				logger.Fatal(ctx, "Failed to get address : %s", err)
			}

			fmt.Printf("Relationship address : %s\n", bitcoin.NewAddressFromRawAddress(ra,
				client.Config.Net))
		} else {
			ra, err := client.Wallet.GetPaymentAddress(ctx)
			if err != nil {
				logger.Fatal(ctx, "Failed to get address : %s", err)
			}

			fmt.Printf("Payment address : %s\n", bitcoin.NewAddressFromRawAddress(ra,
				client.Config.Net))
		}
		return nil
	},
}

func init() {
	commandReceive.Flags().Bool(FlagRelationship, false, "Relationship key")
}
