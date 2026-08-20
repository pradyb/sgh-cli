// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package logger

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestSetLogLevel(t *testing.T) {
	t.Cleanup(func() { SetLogLevel(zerolog.InfoLevel) })

	SetLogLevel(zerolog.DebugLevel)

	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("GlobalLevel() = %v, want %v", zerolog.GlobalLevel(), zerolog.DebugLevel)
	}
	if Glog.GetLevel() != zerolog.DebugLevel {
		t.Errorf("Glog level = %v, want %v", Glog.GetLevel(), zerolog.DebugLevel)
	}
	if Flog.GetLevel() != zerolog.DebugLevel {
		t.Errorf("Flog level = %v, want %v", Flog.GetLevel(), zerolog.DebugLevel)
	}
}

func TestSetVerbose(t *testing.T) {
	t.Cleanup(func() { SetLogLevel(zerolog.InfoLevel) })

	SetVerbose(true)
	if Glog.GetLevel() != zerolog.DebugLevel {
		t.Errorf("after SetVerbose(true), level = %v, want %v", Glog.GetLevel(), zerolog.DebugLevel)
	}

	SetVerbose(false)
	if Glog.GetLevel() != zerolog.InfoLevel {
		t.Errorf("after SetVerbose(false), level = %v, want %v", Glog.GetLevel(), zerolog.InfoLevel)
	}
}
