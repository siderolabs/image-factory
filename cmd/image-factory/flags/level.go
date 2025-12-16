// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags

import (
	"fmt"

	"go.uber.org/zap/zapcore"
)

var LevelValues = []zapcore.Level{
	zapcore.DebugLevel,
	zapcore.InfoLevel,
	zapcore.WarnLevel,
	zapcore.ErrorLevel,
	zapcore.DPanicLevel,
	zapcore.PanicLevel,
	zapcore.FatalLevel,
}

// Level is a pflag.Value wrapper for zapcore.Level.
type Level struct {
	level zapcore.Level
}

// String implements pflag.Value interface.
func (l *Level) String() string {
	return l.level.String()
}

// Set implements pflag.Value interface.
func (l *Level) Set(s string) error {
	if err := l.level.Set(s); err != nil {
		return fmt.Errorf("%w: must be one of %v", err, LevelValues)
	}

	return nil
}

// Type implements pflag.Value interface.
func (l *Level) Type() string {
	return "level"
}

// Value returns the underlying zapcore.Level value.
func (l *Level) Value() zapcore.Level {
	return l.level
}
