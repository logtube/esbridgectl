package main

import (
	"testing"
)

func TestSortProd(t *testing.T) {
	ss := []string{
		"info-prod-2021-03-02",
		"info-prod-2021-03-01",
		"access-prod-2021-03-02",
		"access-prod-2021-03-01",
		"warn-prod-2021-03-02",
		"warn-prod-2021-03-01",
		"access-test-2021-03-02",
		"access-test-2021-03-01",
	}
	sortCandidateIndices(ss)
	t.Log(ss)
}
