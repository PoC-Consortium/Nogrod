# Burstpool written in GO

## Highlights

- SSE4 + AVX2 support
- fair share system based on estimated capacity
- grpc api
- can use multiple wallets as backends using the wallet API
- can talk directly to wallet database
- dynamic payout thresholds/intervals based on messages on the blockchain

## Requirements

- mariadb
- go > v1.9

## Setup

1. Edit config (see config section for settings)

2. Create user for db (as set in the config)

``` shellsession
CREATE USER 'newuser'@'localhost' IDENTIFIED BY 'password';
```

3. Run make

``` shellsession
make
```

## Config

``` yaml
# numeric id of pool
# all miners should set their reward recipient to
# this numeric id
poolPublicId: 10282355196851764065

# secret phrase of the poolPublicId
# used for transactions
secretPhrase: "I shall never let anyone know my secrete phrase"

# the pool can talk to multiple wallets with failover
# at least one is needed for it to work
walletUrls:
    - "http://176.9.47.157:6876"

# pending for miners will increase until
# this threshold (in planck) is reached
# then payout happens
minimumPayout: 25000000000 # in planck

# txFee used for transaction in planck
txFee: 100000000

# blocks after blockHeightPayoutDelay will be checked
# if they were won or not (in order to avoid forks)
blockHeightPayoutDelay: 10

# share of pool on forged blocks:
# 1.0  = 100%
# 0.01 = 1%
poolFeeShare: 0.0

# all deadlines bigger than this limit will
# be ignored
# the limit will be sent as "targetDeadline" in
# the getMiningInfo response
# in s
deadlineLimit: 10000000000

# database connection data for pool's database
db:
    host: "127.0.0.1"
    port: 3306
    user: "burstpool"
    password: "super secret password for pool"
    name: "burstpooldb"

# database connection data base of wallet to fetch reward recips
# if ommited recips will be queried through api
walletDB:
    host: "127.0.0.1"
    port: 3306
    user: "burstwallet"
    password: "super secret password for wallet"
    name: "burstwalletdb"

# account where fees will be transferred to
feeAccountId: 6418289488649374107

# n blocks after miners will be removed from cache
inactiveAfterXBlocks: 10

# share of winner on block forge 1 = 100%, 0.01 = 1%
winnerShare: 1

# port on which pool listens for submitNonce and getMiningInfo requests
poolPort: 8124

# web address of pool
poolAddress: "http://127.0.0.1"

# port on which web ui is reachable
webServerPort: 8080

# all deadlines of blocks with a generation time < tMin
# will be discarded for calculating the effective capacities
# this helps miners with bigger scan time
tMin: 20

# deadlines of the last nAvg blocks with a generation time > tMin
# will be used to estimate the capacities
nAvg: 20

# until nMin confirmed deadlines a miner's historical share is 0
nMin: 1

# port for grpc address
# if ommitted api server won't start
apiPort: 7777

# requests per second until the rate limiter kicks in
# by IP and requestType
allowRequestsPerSecond: 3

# fee for forcing a payment to the miner as soon as possible
# in planck
setNowFee: 500000000

# fee for enabling weekly payout
# first payment in seven days from now
setWeeklyFee: 100000000


# fee for enabling daily payout
# frist payment in one day from now
setDailyFee: 200000000

# fee for setting a custom payout threshold
# payment if pending >= threshold + txFee
setMinPayoutFee: 500000000
```

## Dynamic Payout

Miners can send messages to the pool account to change their payment
thresholds/intervals.

The following messages (unencrypted) are supported:

### now
- forces a payout if you have a positive balance
- after that pool defaults are set

### daily
- forces a daily payout if you have a positive balance at the time, when the payment is made

### weekly
- forces a weekly payout if you have a positive balance at the time, when the payment is made

### 0
- reset to pool's defaults
- any existing recurring payment settings will be removed

### 12345 or similar (unsigned integer) in **planck**
- you can set any integer value you like to eg. 12345
- payment is done after you pass this threshold + txFee with your pendings
- any existing recurring payment settings will be removed

## Donations

For
- maths behind the share system
- patience in explaining the system on discord
- countless statistical analyses
you may thank **Herscht**: **BURST-HWKA-CTBB-J69E-79YHU**

For
- database migration, easier deployment and more flexible configuration
you may thank **ymijorski**: **BURST-SGLT-GEKB-AMP2-GCFT8**

For the implementation you may thank **bold**: **BURST-8V9Y-58B4-RVWP-8HQAV**
