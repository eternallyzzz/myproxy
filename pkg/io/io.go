package io

import (
	"golang.org/x/net/quic"
	"io"
	"myproxy/internal/mlog"
	"sync"
)

func Copy(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	f := func(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
		defer wg.Done()
		defer dst.Close()

		var buf [64 * 1024]byte

		_, err := io.CopyBuffer(dst, src, buf[:])
		if err != nil && !mlog.Ignore(err) {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}

	wg.Add(2)
	go f(dst, src)
	go f(src, dst)
	wg.Wait()

	for _, err := range errs {
		mlog.Error(err.Error())
	}
}

type Pipe struct {
	Stream *quic.Stream
}

func (p *Pipe) Read(b []byte) (n int, err error) {
	return p.Stream.Read(b)
}

func (p *Pipe) Write(b []byte) (n int, err error) {
	return p.Stream.Write(b)
}

func (p *Pipe) Close() error {
	return p.Stream.Close()
}
