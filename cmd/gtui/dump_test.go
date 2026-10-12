package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func TestDump(t *testing.T) {
	gardens := []config.Garden{{Name: "dev", Alias: "d"}, {Name: "broken"}}
	factory := func(g config.Garden) (gardener.Client, error) {
		if g.Name == "broken" {
			return nil, errors.New("no kubeconfig")
		}
		return fake.Landscape(), nil
	}
	var buf bytes.Buffer
	if err := dump(context.Background(), &buf, gardens, factory); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"garden dev (d)",
		"project alpha",
		"shoot api-gw",
		"Hibernated",
		"garden broken",
		"error: no kubeconfig",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	if err := dump(context.Background(), &buf, gardens[1:], factory); err == nil {
		t.Fatal("expected error when all gardens fail")
	}
}
