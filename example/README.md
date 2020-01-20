# RCE Agent Example

This example demonstrates how to write a client and server (agent). Read the source code, `client/main.go` and `server/main.go`, to learn how it's done.

### Running Client and Server (Agent)

To run the example, first `go build` in each directory, `client/` and `server/`.

Second, `cp server/slow-count.sh /tmp/`. This script slowly counts to 10 to demonstrate streaming output (shown later).

Third, run the server (agent) without TLS certificates:

```bash
$ cd server/

$ ./server
2020/01/19 21:26:39.344626 server.go:77: insecure server listening on 127.0.0.1:5501
CTRL-C to shut down
```

By default, the server listens on `127.0.0.1:5501`. The file `server/commands.yaml` contains the whitelist commands that the client can run:

```yaml
commands:
  - name: exit-zero
    exec: ["/bin/bash", "-c", "exit 0"]
  - name: exit-one
    exec: ["/bin/bash", "-c", "exit 1"]
  - name: echo
    exec: ["/bin/echo"]
  - name: ls-tmp
    exec: ["/bin/ls", "/tmp/"]
  - name: slow-count
    exec: ["/tmp/slow-count.sh"]
```

The client can run `show-count`, for example (shown later).

Fourth, in another terminal, run the client:

```bash
$ cd client

$ ./client ls-tmp
2020/01/19 16:32:58 Connecting to 127.0.0.1:5501...
       ID: 6537d416d2744284b7cd6f613e4717ae
     Name: ls-tmp
    State: COMPLETE
      PID: 68820
StartTime: 1579469585591061000
 StopTime: 1579469585597939000
 ExitCode: 0
    Error: 
   Stdout: 
         : com.apple.launchd.O797Q43x0q
         : com.apple.launchd.zJM2uY1rsy
         : mysql.sock
         : mysql.sock.lock
         : powerlog
         : slow-count.sh
   Stderr: 
```

This command makes the agent run `ls /tmp`. The full STDOUT and STDERR of the remote command is always returned. The `Stdout` output should match what is actually in `/tmp` on your computer.

### Streaming Output

The `ls-tmp` command is almost instantaneous, so let's run the `slow-count` command which makes the agent run `/tmp/slow-count.sh`, the script you copied in step 2. Before the final output above, the client will stream the remote command's STDOUT every 2s:

```bash
$ ./client slow-count
2020/01/19 16:39:05 Connecting to 127.0.0.1:5501...
1
2
3
4
5
6
7
8
9
10
       ID: 119f8927d7ba417299e73079886f4af3
     Name: slow-count
    State: COMPLETE
      PID: 68863
StartTime: 1579469945914887000
 StopTime: 1579469955999737000
 ExitCode: 0
    Error:
   Stdout:
         : 1
         : 2
         : 3
         : 4
         : 5
         : 6
         : 7
         : 8
         : 9
         : 10
   Stderr:
```

The output at top (1 through 10) should print every 2 seconds before the command finishes and the final output is printed at bottom. This demonstrates how clients can stream the STDOUT and STDERR of long-running remote commands.

### Using TLS Certificates

Normally, both client and server (agent) use TLS certificates (certs) issued by your private certificate authority (CA). This is how client and server (agent) _mutually_ authenticate: the client verifies the agent's cert, and the agent verifies the client's cert. This requires a properly built Go `tls.Config`; see `TLSConfig()` in `rce.go` in the project root directory.

Re-run the agent and client with `-tls-*` options using the test certs:

```bash
$ cd server/

$ ./server -tls-ca ../../test/tls/test_root_ca.crt -tls-key ../../test/tls/test_server.key -tls-cert ../../test/tls/test_server.crt
TLS loaded
2020/01/19 21:48:21.513650 server.go:75: secure server listening on 127.0.0.1:5501
CTRL-C to shut down
```

Be sure the output says "secure server listening".

Then run the client again:

```bash
$ cd client/

$ ./client -tls-ca ../../test/tls/test_root_ca.crt -tls-key ../../test/tls/test_client.key -tls-cert ../../test/tls/test_client.crt ls-tmp
TLS loaded
2020/01/19 16:49:20 Connecting to 127.0.0.1:5501...
       ID: 6de0867081c2432f945de8500b85da3f
     Name: ls-tmp
    State: COMPLETE
      PID: 69067
<output truncated>
```

If there are any problems with the certs, the client won't connect.

### Increasing gRPC Verbosity

Run the client and server with environment variables `GRPC_GO_LOG_VERBOSITY_LEVEL=99 GRPC_GO_LOG_SEVERITY_LEVEL=info`, like:

```bash
$ GRPC_GO_LOG_VERBOSITY_LEVEL=99 GRPC_GO_LOG_SEVERITY_LEVEL=info ./client ls-tmp
2020/01/19 16:53:16 Connecting to 127.0.0.1:5501...
INFO: 2020/01/19 16:53:16 parsed scheme: ""
INFO: 2020/01/19 16:53:16 scheme "" not registered, fallback to default scheme
INFO: 2020/01/19 16:53:16 ccResolverWrapper: sending new addresses to cc: [{127.0.0.1:5501 0  <nil>}]
INFO: 2020/01/19 16:53:16 ClientConn switching balancer to "pick_first"
INFO: 2020/01/19 16:53:16 pickfirstBalancer: HandleSubConnStateChange: 0xc00009d440, CONNECTING
INFO: 2020/01/19 16:53:16 pickfirstBalancer: HandleSubConnStateChange: 0xc00009d440, READY
INFO: 2020/01/19 16:53:16 transport: loopyWriter.run returning. connection error: desc = "transport is closing"
INFO: 2020/01/19 16:53:16 pickfirstBalancer: HandleSubConnStateChange: 0xc00009d440, TRANSIENT_FAILURE
2020/01/19 16:53:16 client.Start: %srpc error: code = Unavailable desc = transport is closing
```

The client failed ("transport is closing") because the agent is using TLS but the client is not.
