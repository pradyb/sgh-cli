package logger

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Glog zerolog.Logger
var Flog zerolog.Logger

func init() {
	/*output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		level, _ := zerolog.ParseLevel(i.(string))
		return fmt.Sprintf("| \x1b[%dm%-6v\x1b[0m|", zerolog.LevelColors[level], strings.ToUpper(level.String()))
	}
	Glog = zerolog.New(output).With().Timestamp().Caller().Logger()*/

	d, _ := os.UserHomeDir()
	runLogFile, _ := os.OpenFile(filepath.Join(d, "sgh.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	multi := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, runLogFile)

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	Glog = log.Output(multi).With().Timestamp().Caller().Logger()
	Flog = zerolog.New(runLogFile).With().Timestamp().Caller().Logger()
}
