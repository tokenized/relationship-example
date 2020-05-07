# Relationship Example

An example implementation of the relationship protocol. This is not designed to scale or work in a production environment. It is simply for testing and displaying concepts of the relationship protocol.

The relationship protocol provides the ability to create a secure, verified, private communication channel on chain.

There are TODO comments in many of the places that could be done better or that have a higher than reasonable chance for failure.

# Configuration

Create a configuration file based on conf/dev.env.example.

`START_HASH` - A recent block hash before any activity is on chain for your new key.
`DUST_LIMIT` - Must be 576 for 1 satoshi per byte P2PK outputs used in this protocol.

`NODE_ADDRESS` - The IP address and port of your full bitcoin node.
`RPC_HOST` - The IP address and port of your full bitcoin node RPC.
`RPC_USERNAME` - Your RPC username.
`RPC_PASSWORD` - Your RPC password.

Set the `_BUCKET` values to "standalone" and the `_ROOT` values to a local file path to use local storage. Otherwise AWS S3 storage can be configured.

`COMMAND_PATH` - A local file path for a file to be used to send commands from the client (CLI) to the daemon (service).

`XKEY` - Your root private key. Keep this secret. It can be generated in the proper format by running the command `go run cmd/smartcontract/main.go gen --x` from within the smart-contract repo directory.

`WALLET_PATH` - Is the path within your `XKEY` to use as the base for deriving addresses.

`LOG_FILE_PATH` - Is a local file path for the log output. If left blank logging will be to the terminal (stdout) only.
`SPYNODE_LOG_FILE_PATH` - Is a local file path for the spynode specific log output. If not set then the spynode log output is put in the main log.
`LOG_FORMAT` - Can be set to "text" for normal text logging. Leave blank for json logging.

# Running

In a terminal go to the repo directory.

Set your configuration variables to the environment by running the command `source <conf file>`.

Run the command `make run-daemon` to start the daemon. Make sure to let the daemon run to sync with the chain. It should be a matter of minutes if your `START_HASH` is recent and your full node doesn't have latency.

In a separate terminal go to the repo directory again and set the configuration variables again.

Run commands in the client by running `go run cmd/client/main.go <command>`. Use `-h` to see available commands and `<command> -h` to see the additional parameters for that command.

# Usage

Setup your system as describec in "Running".

Your wallet will need funding to function. Get a bitcoin address with the `receive` command. Send a small amount of bitcoin to it, like 50,000 satoshis or less.

You will need one or more "relationship addresses" from other people to initiate a relationship. Get one of yours by running the command `receive --r` and give it to someone else.

If you have one or more relationship addresses from other people, run the command `initiate <address 1> <address 2> ...`. This should create and send 2 transactions. One to fund your relationship address, and the other to send a relationship initiation message from that address.

Relationships are identified by the transaction ID of the relationship initiation message. You can list these by running the `list` command.

All members should accept the relationship before sending any messages within it. Do this by running the `accept <initiation txid>` command. This should also create and send a funding tx and an accept tx.

To send a message within a relationship use the command `message <initiation txid> "Message text"`. Put the text in quotes in case there are spaces so it acts as one parameter to the command line. This should also create and send a funding tx and a message tx.

Messages currently only show up in the log file. Their contents are ASCII, but will be base64 encoded because it is technically a binary field. Copy the base64 text and paste into a base64 decoder to see the message text. There are many available free online.

# Protocol


