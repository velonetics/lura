// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"os"
	"testing"

	"github.com/pucora/lura/v2/config"
)

func TestGetOptions_queryPath(t *testing.T) {
	const file = ".graphql_query_test.txt"
	queryContent := "query Hero { hero { name } }"
	if err := os.WriteFile(file, []byte(queryContent), 0664); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)

	opt, err := GetOptions(config.ExtraConfig{
		Namespace: map[string]interface{}{
			"type":       OperationQuery,
			"query_path": file,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Query != queryContent {
		t.Errorf("unexpected query: %q", opt.Query)
	}
}

func TestGetOptions_missingQueryPath(t *testing.T) {
	_, err := GetOptions(config.ExtraConfig{
		Namespace: map[string]interface{}{
			"type":       OperationQuery,
			"query_path": "/nonexistent/query.graphql",
		},
	})
	if err == nil {
		t.Fatal("expected error for missing query_path file")
	}
}

func TestGetOptions_noConfig(t *testing.T) {
	_, err := GetOptions(config.ExtraConfig{})
	if err != ErrNoConfigFound {
		t.Errorf("expected ErrNoConfigFound, got %v", err)
	}
}
