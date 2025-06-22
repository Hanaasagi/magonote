package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

func TestSingleSelection(t *testing.T) {
	f := framework.NewFramework()

	testCase := framework.TestCase{
		Name:           "Single Selection - IP Address",
		Input:          "192.168.1.1",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "192.168.1.1",
	}

	result := f.RunTest(testCase)
	if !result.Passed {
		t.Errorf("Test failed: %s", result.Error)
	}
}

func TestMultiSelection(t *testing.T) {
	f := framework.NewFramework()

	testCase := framework.TestCase{
		Name:           "Multi Selection - Multiple IP Addresses",
		Input:          "192.168.1.1\n10.0.0.1\n172.16.0.1",
		Args:           []string{"--multi"},
		Keys:           "as ",
		ExpectedOutput: "192.168.1.1\r\n10.0.0.1",
	}

	result := f.RunTest(testCase)
	if !result.Passed {
		t.Errorf("Test failed: %s", result.Error)
	}
}

func TestAllBasicCases(t *testing.T) {
	f := framework.NewFramework()

	testCases := []framework.TestCase{
		{
			Name:           "Single Selection - IP Address",
			Input:          "192.168.1.1",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "192.168.1.1",
		},
		{
			Name:           "Multi Selection - Multiple IP Addresses",
			Input:          "192.168.1.1\n10.0.0.1\n172.16.0.1",
			Args:           []string{"--multi"},
			Keys:           "as ",
			ExpectedOutput: "192.168.1.1\r\n10.0.0.1",
		},
	}

	results := f.RunTests(testCases)
	f.PrintSummary(results)

	// Check if any test failed
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Test '%s' failed: %s", result.Name, result.Error)
		}
	}
}
