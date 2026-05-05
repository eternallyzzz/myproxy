package internal

import "sync"

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
	osi   = make(map[string]OutSeverInfo)
	osiMu sync.RWMutex
)

func GetOsi(key string) (OutSeverInfo, bool) {
	osiMu.RLock()
	v, ok := osi[key]
	osiMu.RUnlock()
	return v, ok
}

func SetOsi(key string, v OutSeverInfo) {
	osiMu.Lock()
	osi[key] = v
	osiMu.Unlock()
}

func RangeOsi(f func(key string, v OutSeverInfo) bool) {
	osiMu.RLock()
	defer osiMu.RUnlock()
	for k, v := range osi {
		if !f(k, v) {
			return
		}
	}
}
