package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/prady-lab/sgh-cli/utils"
)

var (
	Glog zerolog.Logger
	Flog zerolog.Logger
)

func init() {
	configDir, err := utils.ConfigDir()
	if err != nil {
		// Fallback to current directory if config dir creation fails
		configDir = "."
		fmt.Fprintf(os.Stderr, "Warning: Failed to create config directory, using current directory: %v\n", err)
	}

	logFilePath := filepath.Join(configDir, utils.LOG_FILE_NAME)
	runLogFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open log file %s: %v\n", logFilePath, err)
		// Use stdout as fallback
		runLogFile = os.Stdout
	}

	multi := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, runLogFile)

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	Glog = log.Output(multi).With().Timestamp().Caller().Logger()
	Flog = zerolog.New(runLogFile).With().Timestamp().Caller().Logger()
}
