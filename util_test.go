package main

import (
	"sort"
	"strings"
	"testing"
)

func TestSortProd(t *testing.T) {
	ss := []string{"hello-prod", "hello-dev", "hello-test"}
	sort.Slice(ss, func(i, j int) bool {
		if strings.Contains(ss[j], "prod") {
			return true
		}
		return false
	})
	t.Log(ss)
}
