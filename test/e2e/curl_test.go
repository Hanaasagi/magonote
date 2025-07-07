package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

func TestSimpleCurlURL(t *testing.T) {
	f := framework.NewFramework()

	testCase := framework.TestCase{
		Name:           "Simple Curl URL - Single Selection",
		Input:          "curl 'https://example.com/test'",
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "https://example.com/test",
	}

	result := f.RunTest(testCase)
	if !result.Passed {
		t.Errorf("Test failed: %s", result.Error)
	}
}

func TestCurlURLExtraction(t *testing.T) {
	f := framework.NewFramework()

	curlInput := `curl 'https://github.com/Hanaasagi/magonote/hovercards/citation/sidebar_partial?tree_name=master' \
	-H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:142.0) Gecko/20100101 Firefox/142.0' \
	-H 'Accept: text/html' \
	-H 'Accept-Language: zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2' \
	-H 'Accept-Encoding: gzip, deflate, br, zstd' \
	-H 'X-Requested-With: XMLHttpRequest' \
	-H 'Connection: keep-alive' \
	-H 'Sec-Fetch-Dest: empty' \
	-H 'Sec-Fetch-Mode: cors' \
	-H 'Sec-Fetch-Site: same-origin' \
	-H 'Priority: u=4' \
	-H 'TE: trailers'`

	testCase := framework.TestCase{
		Name:           "Curl URL Extraction - Single Selection",
		Input:          curlInput,
		Args:           []string{},
		Keys:           "a",
		ExpectedOutput: "https://github.com/Hanaasagi/magonote/hovercards/citation/sidebar_partial?tree_name=master",
	}

	result := f.RunTest(testCase)
	if !result.Passed {
		t.Errorf("Test failed: %s", result.Error)
	}
}
