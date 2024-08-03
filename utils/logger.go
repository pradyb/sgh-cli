package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Glog zerolog.Logger

func init() {
	/*output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		level, _ := zerolog.ParseLevel(i.(string))
		return fmt.Sprintf("| \x1b[%dm%-6v\x1b[0m|", zerolog.LevelColors[level], strings.ToUpper(level.String()))
	}
	Glog = zerolog.New(output).With().Timestamp().Caller().Logger()*/
	Glog = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}
