package io

import (
	"golang.org/x/net/quic"
	"io"
	"myproxy/internal/mlog"
	"sync"
)

func Copy(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
	var errs []error
	var wg sync.WaitGroup
	f := func(i, o io.ReadWriteCloser) {
		defer wg.Done()
		defer i.Close()
		defer o.Close()

		var buf [64 * 1024]byte

		_, err := io.CopyBuffer(i, o, buf[:])
		//_, err := io.Copy(i, o)
		if err != nil && !mlog.Ignore(err) {
			errs = append(errs, err)
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
	n, err = p.Stream.Write(b)
	p.Stream.Flush()
	return
}

func (p *Pipe) Close() error {
	return p.Stream.Close()
}
