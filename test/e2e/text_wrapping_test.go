package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

func TestTextWrappingCases(t *testing.T) {
	f := framework.NewFramework()

	input := `Short line
This is a much longer line that should definitely wrap when displayed in a narrow terminal window and contains some matches like 192.168.1.100
Another short line
File path /very/long/path/to/some/file/that/might/wrap/across/multiple/lines/in/narrow/terminal.txt
End`

	testCases := []framework.TestCase{
		{
			Name:           "Text Wrapping - Long Line with IP",
			Input:          "This is a very long line that contains an IP address 192.168.1.1 and should wrap gracefully across multiple terminal lines without breaking the matching functionality",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "192.168.1.1",
		},
		{
			Name:           "Text Wrapping - Multiple Matches in Long Line",
			Input:          "Very long line: IP 192.168.1.1 and another IP 10.0.0.1",
			Args:           []string{"--multi"},
			Keys:           "as ",
			ExpectedOutput: "192.168.1.1\r\n10.0.0.1",
		},
		{
			Name:           "Text Wrapping - Mixed Content",
			Input:          input,
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "192.168.1.100",
		},
		{
			Name:           "Text Wrapping - Very Long Single Word",
			Input:          "supercalifragilisticexpialidocious_very_long_filename_that_should_wrap.txt",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "supercalifragilisticexpialidocious_very_long_filename_that_should_wrap.txt",
		},
		{
			Name:           "Text Wrapping - Unicode Characters",
			Input:          "这是一个很长的包含中文字符的行，其中包含一个IP地址 192.168.1.1 应该能够正确换行显示",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "192.168.1.1",
		},
		{
			Name:           "Text Wrapping - Multiple Long Lines",
			Input:          "First very long line with IP 192.168.1.1 that should wrap\nSecond equally long line with different IP 10.0.0.1 that also wraps",
			Args:           []string{"--multi"},
			Keys:           "as ",
			ExpectedOutput: "192.168.1.1\r\n10.0.0.1",
		},
		{
			Name:           "Text Wrapping - Long URL",
			Input:          "Check out this very long URL: https://example.com/very/long/path/with/many/segments/that/should/wrap/gracefully",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "https://example.com/very/long/path/with/many/segments/that/should/wrap/gracefully",
		},
		{
			Name:           "Text Wrapping - File Path",
			Input:          "File: /very/long/path/to/some/important/file/that/might/wrap/config.yaml",
			Args:           []string{},
			Keys:           "a",
			ExpectedOutput: "/very/long/path/to/some/important/file/that/might/wrap/config.yaml",
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
