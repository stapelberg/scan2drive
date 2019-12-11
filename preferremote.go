// Copyright 2016 Michael Stapelberg and contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// preferRemote implements grpc.Balancer by preferring a healthy
// connection to a remote target, but falling back to localhost
// otherwise.
type preferRemote struct {
	local    string
	remote   string
	remoteUp bool
}

func (p *preferRemote) Start(target string, config grpc.BalancerConfig) error {
	p.remote = target
	return nil
}

func (p *preferRemote) Up(addr grpc.Address) (down func(error)) {
	log.Printf("preferRemote.Up(%q)", addr)
	if addr.Addr == p.remote {
		p.remoteUp = true
	}
	return func(error) {
		log.Printf("preferRemote.Down(%q)", addr)
		if addr.Addr == p.remote {
			p.remoteUp = false
		}
	}
}

func (p *preferRemote) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	a := p.local
	if p.remoteUp {
		a = p.remote
	}
	return grpc.Address{Addr: a}, nil, nil
}

func (p *preferRemote) Notify() <-chan []grpc.Address {
	ch := make(chan []grpc.Address, 1)
	addrs := []grpc.Address{{Addr: p.local}}
	if p.remote != "" {
		addrs = append(addrs, grpc.Address{Addr: p.remote})
	}
	ch <- addrs
	return ch
}

func (p *preferRemote) Close() error {
	return nil
}
