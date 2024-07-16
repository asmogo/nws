# NoNet

NoNet is a nostr gateway that allows you to use nostr as a proxy to access other exit nodes.


## How it works

The gateway is a simple http proxy. It listens on port 8881 and 8882 (socks5) and forwards requests to the configured nostr relays. 

## How to use it
```
curl --insecure nprofile1qqs8a8nk09fhrxylcd42haz8ev4cprhnk5egntvs0whafvaaxpk8plgpzdmhxw309akx7cmpd35x7um58gmrvd3k07smk7/v1/resource --proxy http://localhost:8881
```

### Prerequisites

- A nostr private key
- A nostr relay
- A nostr public key that you want to reach


### Running the gateway proxy
#### Configuration

Create a `.env` file in the `cmd/proxy` directory with the following content:

```
NOSTR_RELAYS = 'ws://localhost:6666'
NOSTR_PRIVATE_KEY = "PROXYPRIVATEKEYHEX"
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no nprofile in the request.
- `NOSTR_PRIVATE_KEY`: The private key to sign the events

Run the following command to start the gateway:

```
go run cmd/proxy/main.go
```

### Running the exit node

#### Configuration

Create a `.env` file in the `cmd/exit` directory with the following content:

```
NOSTR_RELAYS = 'ws://localhost:6666'
NOSTR_PRIVATE_KEY = "EXITPUBLICHEX"
BACKEND_HOST = 'localhost:3338'
BACKEND_SCHEME = 'http'
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no nprofile in the request.
- `NOSTR_PRIVATE_KEY`: The private key to sign the events
- `BACKEND_HOST`: The host of the backend to forward requests to
- `BACKEND_SCHEME`: The scheme of the backend to forward requests to

Run the following command to start the exit node:

```
go run cmd/exit/main.go
```
