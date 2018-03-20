package main

import (
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/gokrazy/gokrazy"

	"exp.upspin.io/filesystem"
	"upspin.io/cloud/https"
	"upspin.io/config"
	"upspin.io/flags"
	"upspin.io/rpc/dirserver"
	"upspin.io/rpc/storeserver"
	"upspin.io/upspin"

	// So that the remote key server can be queried for user
	// information.
	_ "upspin.io/key/transports"
	// So that files can be read.
	_ "upspin.io/pack/ee"
	_ "upspin.io/pack/eeintegrity"
	_ "upspin.io/pack/plain"
)

// TODO: change to s2d-upspin once the 8.3 limitation is lifted:
// https://github.com/gokrazy/gokrazy/issues/10
const cmdName = "s2dup"

func main() {
	rand.Seed(time.Now().UnixNano())
	flags.Parse(flags.Server)

	addr := upspin.NetAddr(flags.NetAddr)
	cfg, err := config.FromFile("/perm/upspin-config/config")
	if err != nil {
		log.Fatal(err)
	}

	if err := config.SetFlagValues(cfg, cmdName); err != nil {
		log.Fatal(err)
	}

	fs, err := filesystem.New(cfg, "/perm/scans")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/api/Store/", storeserver.New(cfg, fs.StoreServer(), addr))
	http.Handle("/api/Dir/", dirserver.New(cfg, fs.DirServer(), addr))

	options := https.OptionsFromFlags()

	// Only listen on public addresses (gokrazy listens on private addresses):
	publicAddrs, err := gokrazy.PublicInterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	if len(publicAddrs) == 0 {
		log.Fatalf("no public IP addresses found, cannot obtain LetsEncrypt certificate")
	}
	// TODO: handle listening on multiple IP addresses
	options.HTTPAddr = net.JoinHostPort(publicAddrs[0], "http")

	https.ListenAndServe(nil, options)
}
