# Nostr Web Services (NWS)


NWS replaces the IP layer in TCP transport using Nostr, enabling a secure connection between
clients and backend services.

Exit nodes are reachable through their [nprofiles](https://nostr-nips.com/nip-19), which are combinations of a Nostr public key and multiple relays.

### Prerequisites

- A list of Nostr relays that the exit node is connected to.
- The Nostr private key of the exit node.

The exit node utilizes the private key and relay list to generate an [nprofile](https://nostr-nips.com/nip-19), which is printed in the console on startup.

## Overview

### NWS main components

1. **Entry node**: It forwards tcp packets to the exit node using a SOCKS proxy and creates encrypted events for the public key of the exit node.
2. **Exit node**: It is a TCP reverse proxy that listens for incoming Nostr subscriptions and forwards the payload to the designated backend service.

<img src="nws.png" width="900"/>

## Quickstart

Running NWS using Docker is recommended. For instructions on running NWS on your local machine, refer to the [Build from source](#build-from-source) section.

### Using Docker Compose

Please navigate to the `docker-compose.yaml` file and set `NOSTR_PRIVATE_KEY` to your own private key.
Leaving it empty will generate a new private key on startup.

To set up using Docker Compose, run the following command:
```
docker compose up -d --build
```

This will start an example environment, including: 
* Entry node 
* Exit node
* Exit node with https reverse proxy 
* [Cashu Nutshell](https://github.com/cashubtc/nutshell) (backend service)
* [nostr-relay](https://github.com/scsibug/nostr-rs-relay) 

You can run the following commands to receive your nprofiles:

```bash
docker logs exit-https 2>&1 | awk -F'profile=' '{if ($2) print $2}' | awk '{print $1}'
```
```bash
docker logs exit 2>&1 | awk -F'profile=' '{if ($2) print $2}' | awk '{print $1}`
```

### Sending Requests to the Entry node

With the log information from the previous step, you can use the following command to send a request to the nprofile:

```
curl -v -x socks5h://localhost:8882  http://"$(docker logs exit 2>&1 | awk -F'profile=' '{if ($2) print $2}' | awk '{print $1}' | tail -n 1)"/v1/info --insecure
```

If the nprofile supports TLS, you can choose to connect using https scheme

```
curl -v -x socks5h://localhost:8882  https://"$(docker logs exit-https 2>&1 | awk -F'profile=' '{if ($2) print $2}' | awk '{print $1}' | tail -n 1)"/v1/info --insecure
```

When using https, the entry node can be used as a service, since the operator will not be able to see the request data.

## Build from source

The exit node must be set up to make your services reachable via Nostr.

### Exit node Configuration

Configuration should be completed using environment variables.
Alternatively, you can create a `.env` file in the current working directory with the following content:
```
NOSTR_RELAYS = 'ws://localhost:6666;wss://relay.damus.io'
NOSTR_PRIVATE_KEY = "EXITPRIVATEHEX"
BACKEND_HOST = 'localhost:3338'
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no relay data in the
  request.
- `NOSTR_PRIVATE_KEY`: The private key to sign the events
- `BACKEND_HOST`: The host of the backend to forward requests to

To start the exit node, use this command:

```
go run cmd/exit/exit.go
```

If your backend services support TLS, your service can now start using TLS encryption through a publicly available entry node.

---

### Entry node Configuration

To run an entry node for accessing NWS services behind exit nodes, use the following command:
```
go run cmd/entry/main.go
```
If you don't want to use the `PUBLIC_ADDRESS` feature, no further configuration is needed.

```
PUBLIC_ADDRESS = '<public_ip>:<port>'
```

- `PUBLIC_ADDRESS`: This can be set if the entry node is publicly available. When set, the entry node will additionally bind to this address. Exit node discovery will still be done using Nostr. Once a connection is established, this public address will be used to transmit further data. 
- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no relay data in the
  request.
