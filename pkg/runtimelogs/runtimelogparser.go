package runtimelogs

import (
	"go.uber.org/zap"
	"regexp"
	"strings"
)

var logger, _ = zap.NewDevelopment()
var slogger = logger.Sugar()

// Runtimelog is our internal representation the runtime log of a program.
type Runtimelog struct {
	Functions []string `json:"functions"`
}

// ParseComponentE2ELogs parses the logs that are generated by the "make test-e2e" command in Openshift.
func ParseComponentE2ELogs(testLog []string) (*Runtimelog, error) {
	pattern := regexp.MustCompile(`\[\s*\d\s*]\s*(EXIT|ENTER):\s*(?P<filename>\S*):(?P<lno>\d*)\s*(?P<fname>[a-zA-z0-9]*)`)
	log := Runtimelog{}

	for _, logEntry := range testLog {
		slogger.Debugf("log entry: %s", strings.TrimSpace(logEntry))
		matches := pattern.FindStringSubmatch(logEntry)

		if len(matches) > 4 {
			slogger.Infof("Fn name: %v\n", matches[4])
			log.Functions = append(log.Functions, matches[4])
		}
	}
	return &log, nil
}
