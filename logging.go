package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

type logLevel int

const (
	logLevelUnknown logLevel = iota
	logLevelDebug
	logLevelInfo
	logLevelWarn
	logLevelError
	logLevelCrit
)

func (self logLevel) String() string {
	switch self {
	case logLevelDebug:
		return "DEBUG"
	case logLevelInfo:
		return "INFO"
	case logLevelWarn:
		return "WARN"
	case logLevelError:
		return "ERROR"
	case logLevelCrit:
		return "CRIT"
	default:
		return "Unknown"
	}
}

func logLevelOf(value string) logLevel {
	switch value {
	case "DEBUG":
		return logLevelDebug
	case "INFO":
		return logLevelInfo
	case "WARN":
		return logLevelWarn
	case "ERROR":
		return logLevelError
	case "CRIT":
		return logLevelCrit
	default:
		return logLevelUnknown
	}
}

type logger struct {
	logger   *log.Logger
	name     string
	path     string
	logfile  *os.File
	loglevel logLevel
	lock     chan int
}

func newLogger(name, filePath string, loglevel logLevel) *logger {
	self := &logger{logger: nil, name: name, path: filePath, logfile: nil, loglevel: loglevel, lock: make(chan int, 1)}
	self.lock <- 1
	self._openLogFile()
	return self
}

func (self *logger) _openLogFile() {
	f, err := os.OpenFile(self.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err != nil {
		panic(fmt.Sprintf("error opening file: %v", err))
	}
	_logger := log.New(io.MultiWriter(os.Stdout, f),
		fmt.Sprintf("%v\t", self.name), log.LstdFlags)
	self.logger = _logger
	self.logfile = f
}

func (self *logger) setLogLevel(level logLevel) {
	self.loglevel = level
}

func (self *logger) log(level logLevel, format string, args ...interface{}) {
	<-self.lock
	defer func() { self.lock <- 1 }()
	if level >= self.loglevel {
		if len(args) > 0 {
			self.logger.Print(level.String() + "\t" + fmt.Sprintf(format, args...))
		} else {
			self.logger.Print(level.String() + "\t" + format)
		}
	}
}

func (self *logger) debug(format string, args ...interface{}) {
	self.log(logLevelDebug, format, args...)
}

func (self *logger) info(format string, args ...interface{}) {
	self.log(logLevelInfo, format, args...)
}

func (self *logger) warn(format string, args ...interface{}) {
	self.log(logLevelWarn, format, args...)
}

func (self *logger) error(format string, args ...interface{}) {
	self.log(logLevelError, format, args...)
}

func (self *logger) crit(format string, args ...interface{}) {
	self.log(logLevelCrit, format, args...)
}

func (self *logger) _closeFile() {
	self.logfile.Close()
}

func (self *logger) closeFile() {
	<-self.lock
	defer func() { self.lock <- 1 }()
	self._closeFile()
}

func (self *logger) reloadFile() {
	<-self.lock
	defer func() { self.lock <- 1 }()
	self._closeFile()
	self._openLogFile()
}
