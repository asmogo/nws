# Nostr Web Services

Nostr Web Services (NWS) replaces the IP layer in TCP transport using Nostr, enabling a secure and efficient connection between clients and backend services.


## System Overview

NWS consists of two main components:

1. The **Entry Node**: This is a simple SOCKS proxy that listens on port 8882 and forwards requests to the NProfile.
2. The **Exit Node**: This is a TCP proxy that listens for incoming Nostr subscriptions and forwards the payload to the
   designated backend service.

<img src="nws.png" width="900"/>


## How to use it
You can run the gateway and exit node either on your localhost or using a Docker Compose file.


### Using Docker Compose
To set up using Docker Compose, run:

### Sending Requests via the Gateway
To send a request to the NProfile using the gateway running on your localhost, use:

```
docker compose up -d --build
```

When running the gateway on your localhost, you can use the following command to send a request to the nprofile:

```
curl -v -x socks5h://localhost:8882  http://nprofile1qqsp98rnlp7sn4xuf7meyec48njp2qyfch0jktwvfuqx8vdqgexkg8gpz4mhxw309ahx7um5wgkhyetvv9un5wps8qcqggauk8/v1/info --insecure
```

If the nprofile supports TLS, you can choose to connect using https scheme

```
curl -v -x socks5h://localhost:8882  https://nprofile1qqs8a8nk09fhrxylcd42haz8ev4cprhnk5egntvs0whafvaaxpk8plgpzemhxue69uhhyetvv9ujuwpnxvejuumsv93k2g6k9kr/v1/info --insecure
```

When using https, the socks5 proxy can be used as a service, since the operator of the proxy will not be able to see the request data.

```
curl -v -x socks5h://:8882  https://nprofile1qqs8a8nk09fhrxylcd42haz8ev4cprhnk5egntvs0whafvaaxpk8plgpzemhxue69uhhyetvv9ujuwpnxvejuumsv93k2g6k9kr/v1/info --insecure
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
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no nprofile in the request.
- `NOSTR_PRIVATE_KEY`: The private key to sign the events
- `BACKEND_HOST`: The host of the backend to forward requests to

Run the following command to start the exit node:

```
go run cmd/exit/main.go
```
 
