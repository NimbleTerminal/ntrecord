package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPlayHelpAndArguments(t *testing.T) {
	var stderr bytes.Buffer
	if err := playCast(context.Background(), []string{"-h"}, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "paused") || strings.Contains(stderr.String(), "browser") {
		t.Fatal(stderr.String())
	}
	if err := playCast(context.Background(), nil, &stderr); err == nil {
		t.Fatal("missing input accepted")
	}
	if err := playCast(context.Background(), []string{"missing.cast", "-speed", "2"}, &stderr); err == nil || !strings.Contains(err.Error(), "missing.cast") {
		t.Fatal(err)
	}
}
