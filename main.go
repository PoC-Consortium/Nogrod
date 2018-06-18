// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package main

import (
	. "config"
	"modelx"
	"pool"
	"wallethandler"
	"webserver"
)

func main() {
	LoadConfig()
	modelx.InitCache()

	walletHandler := wallethandler.NewWalletHandler(Cfg.WalletUrls, Cfg.SecretPhrase, Cfg.WalletTimeoutDur,
		Cfg.TrustAllWalletCerts)
	modelx := modelx.NewModelX(walletHandler)

	webServer := webserver.NewWebServer(modelx)
	webServer.Run()

	pool := pool.NewPool(modelx, walletHandler)
	pool.Run()

	select {}
}
