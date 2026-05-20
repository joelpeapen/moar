package util

import log "github.com/sirupsen/logrus"

// TwinLogger adapts logrus to the twin.Logger interface
type TwinLogger struct{}

func (l *TwinLogger) Debug(message string) {
	log.Debug(message)
}

func (l *TwinLogger) Info(message string) {
	log.Info(message)
}

func (l *TwinLogger) Error(message string) {
	log.Error(message)
}
