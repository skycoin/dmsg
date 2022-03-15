package networkmonitor

// WhitelistPKs store whitelisted keys of network monitor
type WhitelistPKs map[string]struct{}

func GetWhitelistPKs() WhitelistPKs {
	return make(WhitelistPKs)
}

func (wl WhitelistPKs) Set(nmPkString string) {
	wl[nmPkString] = struct{}{}
}

func (wl WhitelistPKs) Get(nmPkString string) bool {
	_, ok := wl[nmPkString]
	return ok
}
