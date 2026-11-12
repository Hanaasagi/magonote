package entity

import "github.com/Hanaasagi/magonote/pkg/textdetection/screen"

type Kind string

const (
	KindURL        Kind = "url"
	KindEmail      Kind = "email"
	KindPath       Kind = "path"
	KindFilename   Kind = "filename"
	KindSHA        Kind = "sha"
	KindDigest     Kind = "digest"
	KindUUID       Kind = "uuid"
	KindIPv4       Kind = "ipv4"
	KindIPv4Port   Kind = "ipv4_port"
	KindIPv6       Kind = "ipv6"
	KindIPv6Port   Kind = "ipv6_port"
	KindMAC        Kind = "mac"
	KindHexAddress Kind = "hex_address"
	KindColor      Kind = "color"
	KindDatetime   Kind = "datetime"
	KindDate       Kind = "date"
	KindIPFS       Kind = "ipfs"
	KindTestName   Kind = "test_name"
	KindBranch     Kind = "branch"
	KindCustom     Kind = "custom"
	KindStyled     Kind = "styled"
	KindCell       Kind = "cell"
	KindWord       Kind = "word"
)

const (
	ClassWord      = 0
	ClassStructure = 1
	ClassSemantic  = 2
	ClassCustom    = 3
)

type FileLocation struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type Candidate struct {
	Kind       Kind              `json:"kind"`
	Text       string            `json:"text"`
	Span       screen.Span       `json:"span"`
	Confidence float64           `json:"confidence"`
	Class      int               `json:"class"`
	Source     string            `json:"source"`
	Meta       map[string]string `json:"meta,omitempty"`
	Location   *FileLocation     `json:"location,omitempty"`
}

type Entity = Candidate
