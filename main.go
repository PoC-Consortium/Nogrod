// (c) 2018-present PoC Consortium ALL RIGHTS RESERVED

package main

import (
	. "github.com/PoC-Consortium/Nogrod/pkg/config"
	"github.com/PoC-Consortium/Nogrod/pkg/modelx"
	"github.com/PoC-Consortium/Nogrod/pkg/pool"
	"github.com/PoC-Consortium/Nogrod/pkg/wallethandler"
	"github.com/PoC-Consortium/Nogrod/pkg/webserver"
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
