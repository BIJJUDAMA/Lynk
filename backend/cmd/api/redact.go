package main

import (
	"net/url"
	"strings"
)

func redactDatabaseURL(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return "[redacted-dsn]"
	}
	return u.Redacted()
}
