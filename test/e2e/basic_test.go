package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

func TestBasicCases(t *testing.T) {
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

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := f.RunTest(tc)
			if !result.Passed {
				t.Errorf("Test failed: %s", result.Error)
			}
		})
	}
}
