## example hello world via HTTP over DMSG

### Generate keys:

```
go run ../gen-keys/gen-keys.go | tee dmsgtest.keys
```
OR
```
go run ../gen-keys/gen-keys.go > dmsgtest.keys
```


### Start application using secret key


```
$ go run dmsghttp.go -s $(tail -n1 dmsgtest.key)
[2025-01-28T15:45:26.218738525-06:00] DEBUG disc.NewHTTP [dmsghttp]: Created HTTP client. addr="http://dmsgd.skywire.skycoin.com"
[2025-01-28T15:45:26.218798474-06:00] DEBUG [dmsg_client]: Discovering dmsg servers...
[2025-01-28T15:45:26.494061772-06:00] DEBUG [dmsg_client]: Dialing session... remote_pk=0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb
[2025-01-28T15:45:27.047394122-06:00] DEBUG [dmsg_client]: Serving session. remote_pk=0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb
[2025-01-28T15:45:27.047406948-06:00] INFO [dmsghttp]: Serving Hello World on DMSG address 03f45df9890955214bbfe2e06487741489266a60c487365989b9723680b45e0f6e:80
[2025-01-28T15:46:02.992699297-06:00] INFO [dmsghttp]: Received request from 0352b141c5a423a3788ad7202745b083e2240dea5ce304742a272ec4a80ece6c1c:49153

```

### Get using dmsgcurl


```
$ go run ../../cmd/dmsgcurl/dmsgcurl.go -l debug dmsg://$(head -n1 dmsgtest.key):80
[2025-01-28T15:46:00.785331607-06:00] DEBUG disc.NewHTTP [dmsgcurl]: Created HTTP client. addr="http://dmsgd.skywire.skycoin.com"
[2025-01-28T15:46:00.785375317-06:00] DEBUG [dmsgcurl]: Connecting to dmsg network... dmsg_disc="http://dmsgd.skywire.skycoin.com" public_key="0352b141c5a423a3788ad7202745b083e2240dea5ce304742a272ec4a80ece6c1c"
[2025-01-28T15:46:00.785475179-06:00] DEBUG [dmsg_client]: Discovering dmsg servers...
[2025-01-28T15:46:01.053319739-06:00] DEBUG [dmsg_client]: Dialing session... remote_pk=02a2d4c346dabd165fd555dfdba4a7f4d18786fe7e055e562397cd5102bdd7f8dd
[2025-01-28T15:46:01.611696253-06:00] DEBUG [dmsg_client]: Serving session. remote_pk=02a2d4c346dabd165fd555dfdba4a7f4d18786fe7e055e562397cd5102bdd7f8dd
[2025-01-28T15:46:01.611738628-06:00] DEBUG [dmsgcurl]: Dmsg network ready.
[2025-01-28T15:46:02.018101877-06:00] DEBUG [dmsg_client]: Dialing session... remote_pk=0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb
[2025-01-28T15:46:02.431323641-06:00] DEBUG [dmsg_client]: Updating entry. entry=	version: 0.0.1
	sequence: 0
	registered at: 1738100761468088283
	static public key: 0352b141c5a423a3788ad7202745b083e2240dea5ce304742a272ec4a80ece6c1c
	signature: 4a761f8e84f021683957f272832a007890de622414d155db49bf0cdac944477d6160f846e07f68d6d78c93209ebe593b3dc4c48d1ef729f7ee7c52fb79d295a200
	entry is registered as client. Related info:
		delegated servers:
			02a2d4c346dabd165fd555dfdba4a7f4d18786fe7e055e562397cd5102bdd7f8dd
			0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb


[2025-01-28T15:46:02.569096852-06:00] DEBUG [dmsg_client]: Serving session. remote_pk=0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb
Hello, World![2025-01-28T15:46:03.127242853-06:00] DEBUG [dmsg_client]: Stopped serving client!
[2025-01-28T15:46:03.127263963-06:00] DEBUG [dmsg_client]: Stopped accepting streams. error="session shutdown" session=02a2d4c346dabd165fd555dfdba4a7f4d18786fe7e055e562397cd5102bdd7f8dd
[2025-01-28T15:46:03.12746417-06:00] DEBUG [dmsg_client]: Session closed. error=<nil>
[2025-01-28T15:46:03.127561533-06:00] DEBUG [dmsg_client]: Stopped accepting streams. error="session shutdown" session=0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb
[2025-01-28T15:46:03.127602412-06:00] DEBUG [dmsg_client]: Session closed. error=<nil>
[2025-01-28T15:46:03.127623574-06:00] DEBUG [dmsg_client]: All sessions closed.
[2025-01-28T15:46:03.395174115-06:00] DEBUG [dmsg_client]: Deleting entry. entry=	version: 0.0.1
	sequence: 1
	registered at: 1738100762431422654
	static public key: 0352b141c5a423a3788ad7202745b083e2240dea5ce304742a272ec4a80ece6c1c
	signature: 40a80e385094c38a420d9229d517172a3859f28b79057b5b79cf2600d3021f9d7af57fa26d0f5c9c0643fa45d07bad8169860b842851543d866455a8f170cf6701
	entry is registered as client. Related info:
		delegated servers:
			02a2d4c346dabd165fd555dfdba4a7f4d18786fe7e055e562397cd5102bdd7f8dd
			0281a102c82820e811368c8d028cf11b1a985043b726b1bcdb8fce89b27384b2cb


[2025-01-28T15:46:03.533465754-06:00] DEBUG [dmsg_client]: Entry Deleted successfully.
[2025-01-28T15:46:03.533508976-06:00] DEBUG [dmsgcurl]: Disconnected from dmsg network. error=<nil>

```
