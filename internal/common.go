package internal

type Message struct {
	Tag      string `json:"tag"`
	NodePort uint16 `json:"nodePort"`
}

type OutSeverInfo struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	NodePort uint16 `json:"nodePort"`
}

var (
	Osi = make(map[string]OutSeverInfo)
)
