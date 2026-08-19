// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"io"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type ProgressBar = progressbar.ProgressBar

func NewProgressBar(maxSize int, description string) *progressbar.ProgressBar {
	bar := progressbar.NewOptions(maxSize,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	return bar
}

func NewSilentProgressBar(maxSize int) *progressbar.ProgressBar {
	return progressbar.NewOptions(maxSize,
		progressbar.OptionSetWriter(io.Discard),
		progressbar.OptionSetVisibility(false),
	)
}
