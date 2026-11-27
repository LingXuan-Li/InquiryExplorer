package main

import (
	"os"
	"testing"

	"InquiryExplorer/timing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	timing.Flush()
	os.Exit(code)
}
