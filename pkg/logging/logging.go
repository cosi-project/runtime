// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logging defines logging helpers.
package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Controller creates controller zap field.
func Controller(name string) zap.Field {
	return zap.String("controller", name)
}

// DefaultLogger creates default logger.
func DefaultLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ := config.Build() //nolint:errcheck

	return logger.With(zap.String("component", "controller-runtime"))
}
