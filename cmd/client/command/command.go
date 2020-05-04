package command

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tokenized/smart-contract/pkg/json"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/rpcnode"
	"github.com/tokenized/smart-contract/pkg/spynode"
	"github.com/tokenized/smart-contract/pkg/txbuilder"

	"github.com/spf13/cobra"
)

var clientCommand = &cobra.Command{
	Use:   "relationship",
	Short: "Relationship CLI",
}

func Execute() {
	clientCommand.AddCommand(commandReceive)
	clientCommand.AddCommand(commandInitiate)
	clientCommand.AddCommand(commandAccept)
	clientCommand.Execute()
}

// Context returns an app level context for testing.
func Context() context.Context {

	// -------------------------------------------------------------------------
	// Logging

	logConfig := logger.NewDevelopmentConfig()
	logConfig.Main.Format |= logger.IncludeSystem | logger.IncludeMicro
	logConfig.Main.MinLevel = logger.LevelVerbose
	logConfig.EnableSubSystem(rpcnode.SubSystem)
	logConfig.EnableSubSystem(txbuilder.SubSystem)

	if strings.ToUpper(os.Getenv("LOG_FORMAT")) == "TEXT" {
		logConfig.IsText = true
	}

	logPath := os.Getenv("LOG_FILE_PATH")
	if len(logPath) > 0 {
		os.MkdirAll(path.Dir(os.Getenv("LOG_FILE_PATH")), os.ModePerm)
		logFileName := filepath.FromSlash(os.Getenv("LOG_FILE_PATH"))
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(fmt.Sprintf("Failed to open log file : %v\n", err))
		}
		defer logFile.Close()

		logConfig.Main.AddWriter(logFile)
	}

	// Configure spynode logs
	logPath = os.Getenv("SPYNODE_LOG_FILE_PATH")
	if len(logPath) > 0 {
		spynodeConfig := logger.NewDevelopmentSystemConfig()
		spynodeConfig.SetFile(logPath)
		spynodeConfig.MinLevel = logger.LevelDebug
		logConfig.SubSystems[spynode.SubSystem] = spynodeConfig
	}

	return logger.ContextWithLogConfig(context.Background(), logConfig)
}

// dumpJSON pretty prints a JSON representation of a struct.
func dumpJSON(o interface{}) error {
	js, err := json.MarshalIndent(o, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("```\n%s\n```\n\n", js)

	return nil
}

func isError(response []byte) (bool, string) {
	if len(response) >= 5 && bytes.Equal(response[:5], []byte("err: ")) {
		return true, string(response[5:])
	}
	return false, ""
}
