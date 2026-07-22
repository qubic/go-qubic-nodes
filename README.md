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

Run `./go-qubic-nodes --help` to print the authoritative list of options.

#### Bootstrap peers
```shell
QUBIC_NODES_QUBIC_PEER_LIST: "5.39.222.64;82.197.173.130;82.197.173.129"   # --qubic-peer-list
```

The service ships with a default peer list, but there is no guarantee that those bootstrap peers will be
available, and the list is not maintained. There are several sources of public peers, for example you can
find them [here](https://app.qubic.li/network/live). If discovery is disabled, this list *is* the set of
nodes the service uses.

#### Peer options
| Environment variable                             | CLI flag                               | Type       | Default                                     | Description                                                                                                     |
|--------------------------------------------------|----------------------------------------|------------|---------------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| `QUBIC_NODES_QUBIC_PEER_LIST`                    | `--qubic-peer-list`                    | `[]string` | `5.39.222.64;82.197.173.130;82.197.173.129` | Semicolon-separated list of bootstrap peer addresses.                                                           |
| `QUBIC_NODES_QUBIC_PEER_PORT`                    | `--qubic-peer-port`                    | `string`   | `21841`                                     | Port used to connect to peers.                                                                                  |
| `QUBIC_NODES_QUBIC_PEER_TIMEOUT`                 | `--qubic-peer-timeout`                 | `duration` | `3s`                                        | Timeout for a single peer info exchange.                                                                        |
| `QUBIC_NODES_QUBIC_PEER_CAP`                     | `--qubic-peer-cap`                     | `int`      | `10`                                        | Maximum number of reliable peers kept. Ignored when discovery is off, where the peer list determines the count. |
| `QUBIC_NODES_QUBIC_PEER_SYNC_THRESHOLD`          | `--qubic-peer-sync-threshold`          | `uint`     | `30`                                        | How many ticks a peer may lag behind the network max tick and still count as reliable.                          |
| `QUBIC_NODES_QUBIC_PEER_RESPONSE_TIME_THRESHOLD` | `--qubic-peer-response-time-threshold` | `duration` | `250ms`                                     | Maximum peer response time still considered acceptable.                                                         |

#### Service options
| Environment variable                   | CLI flag                     | Type       | Default | Description                                                                                                     |
|----------------------------------------|------------------------------|------------|---------|-----------------------------------------------------------------------------------------------------------------|
| `QUBIC_NODES_SERVICE_ENABLE_DISCOVERY` | `--service-enable-discovery` | `bool`     | `true`  | Discover new peers from the peers reported by known nodes. When `false`, only the configured peer list is used. |
| `QUBIC_NODES_SERVICE_UPDATE_INTERVAL`  | `--service-update-interval`  | `duration` | `15s`   | Interval between peer status updates.                                                                           |
| `QUBIC_NODES_SERVICE_DEBUG`            | `--service-debug`            | `bool`     | `false` | Enable debug level logging.                                                                                     |
| `QUBIC_NODES_SERVICE_FAST_WARMUP`      | `--service-fast-warmup`      | `bool`     | `true`  | Poll at half the update interval while the peer set is still filling up.                                        |

#### Metrics options
| Environment variable            | CLI flag              | Type     | Default       | Description                                |
|---------------------------------|-----------------------|----------|---------------|--------------------------------------------|
| `QUBIC_NODES_METRICS_NAMESPACE` | `--metrics-namespace` | `string` | `qubic_nodes` | Prometheus namespace for exported metrics. |

### Docker (recommended)
A `docker-compose.yml` file is provided in this repository. You can run it as-is using `docker compose up -d`.

> It may be necessary to configure an up-to-date list of peers if the service fails to start.

### Standalone

Override any defaults you need, either as environment variables:
```shell
export QUBIC_NODES_QUBIC_PEER_LIST="5.39.222.64;82.197.173.130;82.197.173.129"
./go-qubic-nodes
```
or as CLI arguments:
```shell
./go-qubic-nodes --qubic-peer-list="5.39.222.64;82.197.173.130;82.197.173.129"
```

## Available endpoints

### GET /status

Returns the current view of the network. Responds with `503` and a plain text body while no reliable
nodes are known (for example during startup).

```shell
curl http://127.0.0.1:8080/status
```
```json
{
  "max_tick":13692658,
  "last_update":1714654658,
  "number_of_configured_nodes":10,
  "reliable_nodes":[
    {
      "address":"5.39.222.64",
      "port":"21841",
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
      "port":"21841",
      "peers":[
        "194.45.36.144",
        "66.248.204.30",
        "66.23.193.218",
        "109.230.239.74"
      ],
      "last_tick":13692658,
      "last_update":1714654658
    }
  ],
  "most_reliable_node":{
    "address":"82.197.173.129",
    "port":"21841",
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

### GET /max-tick
```shell
curl http://127.0.0.1:8080/max-tick
```
```json
{
  "max_tick":13692662
}
```

### POST /reliable-nodes

Returns only the reliable nodes that are at or above the requested tick.

```shell
curl -X POST http://127.0.0.1:8080/reliable-nodes -d '{"minimum_tick":13692650}'
```
```json
{
  "requested_minimum_tick":13692650,
  "reliable_nodes":[
    {
      "Address":"82.197.173.129",
      "Port":"21841",
      "Peers":[
        "185.130.224.45",
        "95.156.231.27",
        "82.197.173.130",
        "5.39.216.39"
      ],
      "LastTick":13692658,
      "LastUpdate":1714654658,
      "LastUpdateSuccess":true
    }
  ]
}
```

> The node fields of this endpoint are intentionally upper-cased: that is the established wire format and
> renaming them would break existing clients.

### GET /metrics

Prometheus metrics, exported under the namespace configured by `QUBIC_NODES_METRICS_NAMESPACE`.

```shell
curl http://127.0.0.1:8080/metrics
```