package command

import (
	"github.com/spf13/cobra"
)

var commandInitiate = &cobra.Command{
	Use:   "initiate <public key address>",
	Short: "Initiate a relationship with the public key address specified.",
	RunE: func(c *cobra.Command, args []string) error {
		// ctx := Context()

		// client, err := client.NewClient(ctx)
		// if err != nil {
		// 	logger.Fatal(ctx, "Failed to create client : %s", err)
		// }

		// TODO Create and send initiate relationship message

		return nil
	},
}
