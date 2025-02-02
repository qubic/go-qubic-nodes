# The Qubic nodes service

The purpose of the `qubic-nodes` service is to continuously check node reliability and provide, upon request, a list of reliable nodes. 

## Building from source

```shell
go build
```

## Running the service

### Configuration
The service can be configured either by CLI parameters or environment variables.
The current configuration will be printed at startup.

#### Required parameters
```shell
QUBIC_NODES_QUBIC_PEER_LIST:                "5.39.222.64;82.197.173.130;82.197.173.129"
```

The list of bootstrap nodes might be found [here](https://app.qubic.li/network/live).

#### Optional parameters
```shell
QUBIC_NODES_QUBIC_EXCHANGE_TIMEOUT:         (default: 2s)
QUBIC_NODES_QUBIC_MAX_TICK_ERROR_THRESHOLD: (default: 50)
QUBIC_NODES_QUBIC_RELIABLE_TICK_RANGE:      (default: 30)

QUBIC_NODES_SERVICE_TICKER_UPDATE_INTERVAL: (default: 5s)
```

### Docker (recommended)
A `docker-compose.yml` file is provided in this repository. You can run it as-is using `docker compose up -d`.

> It may be necessary to configure an up-to-date list of peers if the service fails to start.

### Standalone

Export the required environment variables:
```shell
export QUBIC_NODES_QUBIC_PEER_LIST="5.39.222.64;82.197.173.130;82.197.173.129"
```
You can now start the service:
```shell
./go-qubic-nodes
```

## Available endpoints

### /status
```shell
curl http://127.0.0.1:8080/status  
```
```json
{
  "max_tick":13692658,
  "last_update":1714654658,
  "reliable_nodes":[
    {
      "address":"5.39.222.64",
      "peers":[
        "194.45.36.121",
        "109.230.239.139",
        "66.23.193.243",
        "5.39.217.105"
      ],
      "last_tick":13692657,
      "last_update":1714654657
    },
    {
      "address":"82.197.173.130",
      "peers":[
        "194.45.36.144",
        "66.248.204.30",
        "66.23.193.218",
        "109.230.239.74"
      ],
      "last_tick":13692658,
      "last_update":1714654658
    },
    {
      "address":"82.197.173.129",
      "peers":[
        "185.130.224.45",
        "95.156.231.27",
        "82.197.173.130",
        "5.39.216.39"
      ],
      "last_tick":13692658,
      "last_update":1714654658
    }
  ],
  "most_reliable_node":{
    "address":"82.197.173.129",
    "peers":[
      "185.130.224.45",
      "95.156.231.27",
      "82.197.173.130",
      "5.39.216.39"
    ],
    "last_tick":13692658,
    "last_update":1714654658
  }
}
```

### /max-tick
```shell
curl http://127.0.0.1:8080/max-tick      
```
```json
{
  "max_tick":13692662
}
```