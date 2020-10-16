package utils

import (
	"github.com/npathai/github-cli-clone/ui"
	"os"
	"time"
)

var timeNow = time.Now()

func Check(err error) {
	if err != nil {
		ui.Errorln(err)
		os.Exit(1)
	}
}
