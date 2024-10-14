package utils

import (
	"os"
	"runtime"
)

func ConfigDir() string {
	d, _ := os.UserHomeDir()
	switch os := runtime.GOOS; os {
	case "darwin", "linux":
		d = d + "/.config/sgh"
	}
	return d
}
