// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	. "logger"
	"time"

	"go.uber.org/zap"
)

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     uint32 `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type Config struct {
	Version                string
	BlockHeightPayoutDelay uint64   `yaml:"blockHeightPayoutDelay"`
	SecretPhrase           string   `yaml:"secretPhrase"`
	WalletUrls             []string `yaml:"walletUrls"`
	PoolPublicID           uint64   `yaml:"poolPublicId"`
	MinimumPayout          int64    `yaml:"minimumPayout"`
	PoolFeeShare           float64  `yaml:"poolFeeShare"`
	DeadlineLimit          uint64   `yaml:"deadlineLimit"`
	WinnerShare            float64  `yaml:"winnerShare"`
	DB                     DBConfig `yaml:"db"`
	WalletDB               DBConfig `yaml:"walletDB"`
	FeeAccountID           uint64   `yaml:"feeAccountId"`
	TxFee                  int64    `yaml:"txFee"`
	InactiveAfterXBlocks   uint64   `yaml:"inactiveAfterXBlocks"`
	PoolAddress            string   `yaml:"poolAddress"`
	PoolListenAddress      string   `yaml:"poolListenAddress"`
	PoolPort               uint     `yaml:"poolPort"`
	WebServerPort          uint     `yaml:"webServerPort"`
	WebServerListenAddress string   `yaml:"webServerListenAddress"`
	AllowRequestsPerSecond int      `yaml:"allowRequestsPerSecond"`
	NAVG                   int      `yaml:"nAvg"`
	NMin                   int      `yaml:"nMin"`
	APIPort                uint     `yaml:"apiPort"`
	APIListenAddress       string   `yaml:"apiListenAddress"`
	TMin                   int32    `yaml:"tMin"`
	PoC2StartHeight        uint64   `yaml:"PoC2StartHeight"`
	SetNowFee              int64    `yaml:"setNowFee"`
	SetWeeklyFee           int64    `yaml:"setWeeklyFee"`
	SetDailyFee            int64    `yaml:"setDailyFee"`
	SetMinPayoutFee        int64    `yaml:"setMinPayoutFee"`
	WalletTimeout          int64    `yaml:"walletTimeout"`
	WalletTimeoutDur       time.Duration
}

var Cfg Config

func LoadConfig() {
	Cfg.Version = "v0.9.5"

	raw, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		Logger.Fatal("reading config failed", zap.Error(err))
	}

	err = yaml.Unmarshal(raw, &Cfg)
	if err != nil {
		Logger.Fatal("unpacking config failed", zap.Error(err))
	}

	validateConfig()
}

func validateConfig() {
	if Cfg.SecretPhrase == "" {
		Logger.Fatal("'secretPhrase' can't be empty")
	}

	if len(Cfg.WalletUrls) == 0 {
		Logger.Fatal("no wallet urls defined in 'walletUrls'")
	}

	if Cfg.PoolPublicID == 0 {
		Logger.Fatal("'poolPublicId' can't be empty")
	}

	if Cfg.PoolFeeShare > 1.0 {
		Logger.Fatal("'poolFeeShare' must be between 0.0 and 1.0")
	}

	if Cfg.DB.Host == "" {
		Cfg.DB.Host = "127.0.0.1"
	}

	if Cfg.DB.Port == 0 {
		Cfg.DB.Port = 3306
	}

	if Cfg.DB.Name == "" {
		Logger.Fatal("'dbName' can't be empty")
	}

	if Cfg.DB.User == "" {
		Logger.Fatal("'dbUser' can't be empty")
	}

	if Cfg.WalletDB.Name == "" {
		Logger.Fatal("'walletDB.Name' can't be empty")
	}

	if Cfg.WalletDB.Host == "" {
		Cfg.WalletDB.Host = "127.0.0.1"
	}

	if Cfg.WalletDB.Port == 0 {
		Cfg.WalletDB.Port = 3306
	}

	if Cfg.WalletDB.User == "" {
		Logger.Fatal("'walletDB.User' can't be empty")
	}

	if Cfg.FeeAccountID == 0 && Cfg.PoolFeeShare > 0.0 {
		Logger.Fatal("'feeAccountId' can't be empty if PoolFee is over 0.0")
	}

	if Cfg.WinnerShare < 0.0 || Cfg.WinnerShare > 1.0 {
		Logger.Fatal("'winnerShare' must be between 0.0 and 1.0")
	}

	if Cfg.InactiveAfterXBlocks == 0 {
		Logger.Fatal("'InactiveAfterXBlocks' must be bigger than 0")
	}

	if Cfg.PoolPort == 0 {
		Logger.Fatal("'poolPort' can't be empty or 0")
	}

	if Cfg.WebServerPort == 0 {
		Logger.Fatal("'webServerPort' can't be empty or 0")
	}

	if Cfg.PoolAddress == "" {
		Logger.Fatal("'poolAddress' can't be empty")
	}

	if Cfg.AllowRequestsPerSecond < 0 {
		Logger.Fatal("'allowRequestsPerSecond' can't be negativ")
	}

	if Cfg.AllowRequestsPerSecond == 0 {
		Cfg.AllowRequestsPerSecond = 4
		Logger.Info("Using default 4 for allowRequestsPerSecond")
	}

	if Cfg.NAVG < 0 {
		Logger.Fatal("'nAvg' can't be negativ")
	}

	if Cfg.NAVG == 0 {
		Cfg.NAVG = 360
		Logger.Info("using default 360 for 'nAvg'")
	}

	if Cfg.NMin < 0 {
		Logger.Fatal("'nMin' can't be negativ")
	}

	if Cfg.NMin == 0 {
		Cfg.NMin = 10
		Logger.Info("using default 10 for 'nMin'")
	}

	if Cfg.NMin >= Cfg.NAVG {
		Logger.Info("'nAvg' must be bigger than 'nMin'")
	}

	if Cfg.TMin < 0 {
		Logger.Fatal("'tMin' can't be negativ")
	}

	if Cfg.TxFee == 0 {
		Cfg.TxFee = 100000000
		Logger.Info("Using default 100000000 for Cfg.TxFee")
	}

	if Cfg.PoC2StartHeight == 0 {
		Cfg.PoC2StartHeight = ^uint64(0)
	}

	if Cfg.WalletTimeout <= 0 {
		Cfg.WalletTimeoutDur = 5 * time.Second
		Logger.Info("Using default 5s for Cfg.WalletTimeout")
	} else {
		Cfg.WalletTimeoutDur = time.Duration(Cfg.WalletTimeout) * time.Second
	}
}
