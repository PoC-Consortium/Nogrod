// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package main

import (
	. "github.com/PoC-Consortium/goburstpool/pkg/config"
	"github.com/PoC-Consortium/goburstpool/pkg/modelx"
	"github.com/PoC-Consortium/goburstpool/pkg/pool"
	"github.com/PoC-Consortium/goburstpool/pkg/wallethandler"
	"github.com/PoC-Consortium/goburstpool/pkg/webserver"
)

func main() {
	LoadConfig()
	modelx.InitCache()

	walletHandler := wallethandler.NewWalletHandler(Cfg.WalletUrls, Cfg.SecretPhrase, Cfg.WalletTimeoutDur,
		Cfg.TrustAllWalletCerts)
	modelx := modelx.NewModelX(walletHandler, true)

	webServer := webserver.NewWebServer(modelx)
	webServer.Run()

	pool := pool.NewPool(modelx, walletHandler)
	pool.Run()

	select {}
}
