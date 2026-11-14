package main

import (
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
)

func listHistory(status, from, to string) (any, error) {
	filter, err := report.ParseFilter(status, from, to)
	if err != nil {
		return nil, err
	}
	return report.List(paths.ReportsDir(), filter)
}

func readHistory(id string) (any, error) {
	return report.Open(paths.ReportsDir(), id)
}
