// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package main

import (
	. "config"
	"modelx"
	"pool"
	"runtime"
	"util"
	"wallet"
	"webserver"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	LoadConfig()
	modelx.InitCache()
	util.CacheAlphas(Cfg.NAVG, Cfg.NMin)

	wallet := wallet.NewBrsWallet(Cfg.WalletUrls, Cfg.SecretPhrase)
	modelx := modelx.NewModelX(wallet)

	webServer := webserver.NewWebServer(modelx)
	webServer.Run()

	pool := pool.NewPool(modelx, wallet)
	pool.Run()

	select {}
}
