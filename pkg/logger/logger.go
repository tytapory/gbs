package logger

import (
	"fmt"
	"log"
	"os"
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	FATAL
)

var l Logger

type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
	level       int
}

func InitializeLoggers(logLevel string, logFilepath string) {
	setLogLevel(logLevel)
	var err error
	var logFile *os.File
	if logFilepath != "" {
		logFile, err = openLogFile(logFilepath)
	}
	if err != nil || logFilepath == "" {
		fmt.Printf("ERROR: Can't open/create file %s. Redirecting logs to stdout.\n", logFilepath)
		logFile = os.Stdout
	}
	l.debugLogger = log.New(logFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.infoLogger = log.New(logFile, "INFO: ", log.Ldate|log.Ltime)
	l.warnLogger = log.New(logFile, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.errorLogger = log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.fatalLogger = log.New(logFile, "FATAL: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func openLogFile(filepath string) (*os.File, error) {
	logFile, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return logFile, err
}

func setLogLevel(level string) {
	switch level {
	case "debug":
		l.level = DEBUG
	case "info":
		l.level = INFO
	case "warn":
		l.level = WARN
	case "error":
		l.level = ERROR
	case "fatal":
		l.level = FATAL
	default:
		l.level = INFO
		errMessage := fmt.Sprintf("Config error. Unknown logging level: %s. Correct levels: debug, info, warn, error, fatal.", level)
		if l.errorLogger != nil {
			Error(errMessage)
		} else {
			fmt.Println(errMessage)
		}
	}
}

func Debug(msg string) {
	if l.level <= DEBUG && l.debugLogger != nil {
		l.debugLogger.Output(3, msg)
	}
}

func Info(msg string) {
	if l.level <= INFO && l.infoLogger != nil {
		l.infoLogger.Output(3, msg)
	}
}

func Warn(msg string) {
	if l.level <= WARN && l.warnLogger != nil {
		l.warnLogger.Output(3, msg)
	}
}

func Error(msg string) {
	if l.level <= ERROR && l.errorLogger != nil {
		l.errorLogger.Output(3, msg)
	}
}

func Fatal(msg string) {
	if l.level <= FATAL && l.fatalLogger != nil {
		l.fatalLogger.Output(3, msg)
	}
	os.Exit(1)
}
