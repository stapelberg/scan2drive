package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"upspin.io/cloud/https"
	"upspin.io/config"
	"upspin.io/exp/filesystem"
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
	https.ListenAndServeFromFlags(nil)
}
