# Troubleshooting

## Invalid Certification Error 

If you get errors like below from scheduler, aggregator or node agent, you may want to check if the time is sychronized across the cluster.  
`Err: connection error: desc = "transport: authentication handshake failed: x509: certificate has expired or is not yet valid: current time 2023-09-20T01:59:37Z is before 2023-09-21T01:58:58Z"`
`rpc error: code = Unavailable desc = connection error: desc = "error reading server preface: remote error: tls: bad certificate"`