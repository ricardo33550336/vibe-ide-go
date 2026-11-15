//go:build !darwin && !linux

package main

import "errors"

// localPTY is unused on platforms where MyGo's own pseudo-terminal is the
// one that works.
type localPTY struct{}

var errLocalPTY = errors.New("local terminal: not supported here")

func startLocalPTY(string) (*localPTY, error) { return nil, errLocalPTY }

func (p *localPTY) setOnEnd(func(int)) {}

func (p *localPTY) Read([]byte) (int, error) { return 0, errLocalPTY }

func (p *localPTY) Write([]byte) (int, error) { return 0, errLocalPTY }

func (p *localPTY) Close() error { return nil }
