package main

import (
	"strings"
	"testing"
)

func TestRedactDatabaseURL_StripsPassword(t *testing.T) {
	dsn := "postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable"
	got := redactDatabaseURL(dsn)
	if strings.Contains(got, "lynk_password") {
		t.Fatalf("redacted DSN still contains password: %s", got)
	}
	if !strings.Contains(got, "lynk_user") {
		t.Fatalf("expected username preserved, got %s", got)
	}
	if !strings.Contains(got, "localhost:5432") {
		t.Fatalf("expected host preserved, got %s", got)
	}
	if !strings.Contains(got, "lynk_db") {
		t.Fatalf("expected db name preserved, got %s", got)
	}
}

func TestRedactDatabaseURL_Unparseable(t *testing.T) {
	got := redactDatabaseURL("not a url ::: secret_password")
	if strings.Contains(got, "secret_password") {
		t.Fatalf("unparseable DSN must not echo secrets: %s", got)
	}
}
