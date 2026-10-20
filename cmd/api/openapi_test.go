package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// Every /api route the server mounts must be in api/openapi.yaml with the same
// method, and the spec must not document routes the server lacks (F-3).
func TestOpenAPI_RoutesMatchSpec(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))

	var documented []string
	for path, ops := range spec.Paths {
		for method := range ops {
			switch method {
			case "get", "put", "post", "delete", "patch", "head", "options", "trace":
				documented = append(documented, strings.ToUpper(method)+" "+path)
			}
		}
	}

	r, ok := newRouter(routerDeps{log: slog.New(slog.NewTextHandler(io.Discard, nil))}).(chi.Routes)
	require.True(t, ok, "newRouter must return a chi router")
	var served []string
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/api/") {
			served = append(served, method+" "+route)
		}
		return nil
	}))

	sort.Strings(documented)
	sort.Strings(served)
	assert.NotEmpty(t, served)
	assert.Equal(t, documented, served)
}
