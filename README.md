# Nostr Web Services (NWS)

---
Nostr Web Services (NWS) replaces the IP layer in TCP transport using Nostr, enabling a secure connection between
clients and backend services.

### Prerequisites

Exit nodes are reachable using their [nprofile](https://nostr-nips.com/nip-19). The nprofile is a composition of a nostr
public key and multiple relays.

- A list of nostr relays, that the exit node is connected to.
- The nostr private key (for the exit node).

Using this private key and the relay list, the exit node will generate a [nprofile](https://nostr-nips.com/nip-19) and
print it to the console on startup.

## Overview

---
NWS consists of two main components:

1. The **Entry Node** is used to forward tcp packets to the exit node using a SOCKS proxy. It creates encrypted events
   for the public key of the exit node.
2. The **Exit Node** is a TCP reverse proxy that listens for incoming Nostr subscriptions and forwards the payload to
   the
   designated backend service.

<img src="nws.png" width="900"/>

## Quickstart

---
It is recommended to run NWS using docker.

There are instructions for running NWS on your local machine in the [Build from source](#build-from-source) section.

### Using Docker Compose

To set up using Docker Compose, run:

```
docker compose up -d --build
```

This command will start an example setup including the entry node, exit node and a backend service.

### Sending Requests to the entry node

You can use the following command to send a request to the nprofile:

```
curl -v -x socks5h://localhost:8882  http://nprofile1qqsp98rnlp7sn4xuf7meyec48njp2qyfch0jktwvfuqx8vdqgexkg8gpz4mhxw309ahx7um5wgkhyetvv9un5wps8qcqggauk8/v1/info --insecure
```

If the nprofile supports TLS, you can choose to connect using https scheme

```
curl -v -x socks5h://localhost:8882  https://nprofile1qqstw2nc544vkl4760yeq9xt2yd0gthl4trm6ruvpukdthx9fy5xqjcpz4mhxw309ahx7um5wgkhyetvv9un5wps8qcqcelsf6/v1/info --insecure
```

When using https, the entry node can be used as a service, since the operator will not be able to see the request data.

## Build from source

You need to configure set up the exit node to make you services reachable via nostr.

### Configuration

Configuration can be done using environment variables.
Alternatively, you can create a `.env` file in the `cmd/exit` directory with the following content:

```
NOSTR_RELAYS = 'ws://localhost:6666'
NOSTR_PRIVATE_KEY = "EXITPUBLICHEX"
BACKEND_HOST = 'localhost:3338'
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no nprofile in the
  request.
- `NOSTR_PRIVATE_KEY`: The private key to sign the events
- `BACKEND_HOST`: The host of the backend to forward requests to

### Running the exit node

Run the following command to start the exit node:

```
go run cmd/exit/main.go
```

If your backend services supports TLS, you can now start using your service with TLS encryption using a publicly
available entry node.

### Running the entry node

If you want to run an entry node for accessing NWS services behind exit nodes, please use the following command:

```
go run cmd/proxy/main.go
```

#### Entry Node Configuration

If you used environment variables, there is no further configuration needed.
Otherwise, you can create a `.env` file in the `cmd/proxy` directory with the following content:

```
NOSTR_RELAYS = 'ws://localhost:6666'
```

- `NOSTR_RELAYS`: A list of nostr relays to publish events to. Will only be used if there was no nprofile in the
  request.

