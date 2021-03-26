package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_sortCandidateIndices(t *testing.T) {
	var slices = []string{
		"1-test-2021-02-02",
		"2-prod-2021-02-02",
		"3-test-2021-01-02",
		"4-prod-2021-01-02",
	}

	sortCandidateIndices(slices)

	assert.Equal(t, []string{
		"4-prod-2021-01-02",
		"3-test-2021-01-02",
		"2-prod-2021-02-02",
		"1-test-2021-02-02",
	}, slices)
}
