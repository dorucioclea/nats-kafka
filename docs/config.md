# NATS-Kafka Bridge Configuration

The bridge uses a single configuration file passed on the command line or environment variable. Configuration is organized into a root section and several blocks.

* [Specifying the Configuration File](#specify)
* [Shared](#root)
* [TLS](#tls)
* [Logging](#logging)
* [Monitoring](#monitoring)
* [NATS](#nats)
* [NATS Streaming](#stan)
* [Connectors](#connectors)

The configuration file format matches the NATS server and supports file includes of the form:

```yaml
include "./includes/connectors.conf"
```

<a name="specify"></a>

## Specifying the Configuration File

To set the configuration on the command line, use:

```bash
% nats-kafka -c <path to config file>
```

To set the configuration file using an environment variable, export `NATS_KAFKA_BRIDGE_CONFIG` with the path to the configuration.

<a name="root"></a>

## Root Section

The root section:

```yaml
reconnectinterval: 5000,
connecttimeout: 5000,
```

can currently contain settings for:

* `reconnectinterval` - this value, in milliseconds, is the time used in between reconnection attempts for a connector when it fails. For example, if a connector loses access to NATS, the bridge will try to restart it every `reconnectinterval` milliseconds.
* `connecttimeout` - this value, in milliseconds, is the time used when trying to connect to Kafka.

## TLS <a name="tls"></a>

NATS, streaming, Kafka and HTTP configurations all take an optional TLS setting. The TLS configuration takes three possible settings:

* `root` - file path to a CA root certificate store, used for NATS connections
* `cert` - file path to a server certificate, used for HTTPS monitoring and optionally for client side certificates with NATS
* `key` - key for the certificate store specified in cert

<a name="logging"></a>

### Logging

Logging is configured in a manner similar to the nats-server:

```yaml
logging: {
  time: true,
  debug: false,
  trace: false,
  colors: true,
  pid: false,
}
```

These properties are configured for:

* `time` - include the time in logging statements
* `debug` - include debug logging
* `trace` - include verbose, or trace, logging
* `colors` - colorize the logging statements
* `pid` - include the process id in logging statements

<a name="monitoring"></a>

## Monitoring

The monitoring section:

```yaml
monitoring: {
  httpsport: -1,
  tls: {
      cert: /a/server-cert.pem,
      key: /a/server-key.pem,
  }
}
```

Is used to configure an HTTP or HTTPS port, as well as TLS settings when HTTPS is used.

* `httphost` - the network interface to publish monitoring on, valid for HTTP or HTTPS. An empty value will tell the server to use all available network interfaces.
* `httpport` - the port for HTTP monitoring, no TLS configuration is expected, a value of -1 will tell the server to use an ephemeral port, the port will be logged on startup.

`2019/03/20 12:06:38.027822 [INF] starting http monitor on :59744`

* `httpsport` - the port for HTTPS monitoring, a TLS configuration is expected, a value of -1 will tell the server to use an ephemeral port, the port will be logged on startup.
* `tls` - a [TLS configuration](#tls).

The `httpport` and `httpsport` settings are mutually exclusive, if both are set to a non-zero value the bridge will not start.

<a name="nats"></a>

## NATS

The bridge makes a single connection to NATS. This connection is shared by all connectors. Configuration is through the `nats` section of the config file:

```yaml
nats: {
  Servers: ["localhost:4222"],
  ConnectTimeout: 5000,
  MaxReconnects: 5,
  ReconnectWait: 5000,
}
```

NATS can be configured with the following properties:

* `servers` - an array of server URLS
* `connecttimeout` - the time, in milliseconds, to wait before failing to connect to the NATS server
* `reconnectwait` - the time, in milliseconds, to wait between reconnect attempts
* `maxreconnects` - the maximum number of reconnects to try before exiting the bridge with an error.
* `tls` - (optional) [TLS configuration](#tls). If the NATS server uses unverified TLS with a valid certificate, this setting isn't required.
* `usercredentials` - (optional) the path to a credentials file for connecting to NATs.

<a name="stan"></a>

## NATS Streaming

The bridge makes a single connection to a NATS streaming server. This connection is shared by all connectors. Configuration is through the `stan` section of the config file:

```yaml
stan: {
  ClusterID: "test-cluster"
  ClientID: "kafkabridge"
}
```

NATS streaming can be configured with the following properties:

* `clusterid` - the cluster id for the NATS streaming server.
* `clientid` - the client id for the bridge, shared by all connections.
* `pubackwait` - the time, in milliseconds, to wait before a publish fails due to a timeout.
* `discoverprefix` - the discover prefix for the streaming server.
* `maxpubacksinflight` - maximum pub ACK messages that can be in flight for this connection.
* `connectwait` - the time, in milliseconds, to wait before failing to connect to the streaming server.

<a name="connectors"></a>

## Connectors

The final piece of the bridge configuration is the `connect` section. Connect specifies an array of connector configurations. All connector configs use the same format, relying on optional settings to determine what the do.

```yaml
connect: [
  {
      type: "KafkaToNATS",
      brokers: ["localhost:9092"]
      id: "zip",
      topic: "test",
      subject: "one",
  },{
      type: "NATSToKafka",
      brokers: ["localhost:9092"]
      id: "zap",
      topic: "test2",
      subject: "two",
  },
],
```

The most important property in the connector configuration is the `type`. The type determines which kind of connector is instantiated. Available, uni-directional, types include:

* `KafkaToNATS` - a topic to NATS connector
* `KafkaToStan` - a topic to streaming connector
* `NATSToKafka` - a streaming to topic connector
* `STANToKafka` - a NATS to topic connector

All connectors can have an optional id, which is used in monitoring:

* `id` - (optional) user defined id that will tag the connection in monitoring JSON.

For NATS connections, specify:

* `subject` - the subject to subscribe/publish to, depending on the connections direction.
* `natsqueue` - the queue group to use in subscriptions, this is optional but useful for load balancing.

Keep in mind that NATS queue groups do not guarantee ordering, since the queue subscribers can be on different nats-servers in a cluster. So if you have to bridges running with connectors on the same NATS queue/subject pair and have a high message rate you may get messages in the Kafka topic out of order.

For streaming connections, there is a single required setting and several optional ones:

* `channel` - the streaming channel to subscribe/publish to.
* `durablename` - (optional) durable name for the streaming subscription (if appropriate.)
* `startatsequence` - (optional) start position, use -1 for start with last received, 0 for deliver all available (the default.)
* `startattime` - (optional) the start position as a time, in Unix seconds since the epoch, mutually exclusive with `startatsequence`.

All connectors must specify Kafka connection properties, with a few optional settings available as well:

* `brokers` - a string array of broker host:port settings
* `topic` - the Kafka topic to listen/send to
* `tls` - A tls config for the connection
* `balancer` - required for a writer, should be "hash" or "leastbytes"
* `groupid` - (exclusive with partition) used by the reader to set a group id
* `partition` - (exclusive with groupid) used by the reader to set a partition
* `minbytes` - (optional) used by Kafka readers to set the minimum bytes for a read
* `maxbytes` - (optional) used by a Kafka reader to set the maximum bytes for a read
* `keytype` - (optional) defines the way keys are assigned to messages coming from NATS (see below)
* `keyvalue` - (optional) extra data that may be used depending on the key type

Available key types are:

* `fixed` - the value in the `keyvalue` field is assigned to all messages
* `subject` - the subject the incoming NATS message was sent on is used as the key
* `reply` - the reply-to sent with the incoming NATS messages is used as the key
* `subjectre` - the value in the `keyvalue` field used as a regular expression and the first capture group, when matched to the subject, is used as the key
* `replyre` - the value in the `keyvalue` field used as a regular expression and the first capture group, when matched to the reply-to, is used as the key

If unset, an empty key is assigned during translation from NATS to Kafka. If the regex types are used and they don't match, an empty key is used.

For nats streaming connections channel is treated as the subject and durable name is treated as the reply to, so that reply key type will use the durable name as the key.