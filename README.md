Go WebSub Client
================

A Go implementation of a [WebSub](https://www.w3.org/TR/websub/) client. It has been tested to pass every [WebSub Rocks! Subscriber test](https://websub.rocks/subscriber).

See `examples/basic.go` for a basic example which uses a built-in webserver to subscribe.

Looking for a server? Check out [WebSub Server](https://github.com/tystuyfzand/websub-server)!

Importing:

```
go get meow.tf/websub/client
```

Features
--------

* Acts as it's own http server, or can be used externally by using `VerifySubscription` and handing requests yourself.
* Supports secrets and sha1, sha256, sha384, sha512 validation
* Integrates into the matching [WebSub Server](https://github.com/tystuyfzand/websub-server) stores to use the same across both.
* Supports discovery of HTTP Headers, RSS, Atom, and HTML for hubs.
* Correctly follows topic and hub redirects, as well as uses the discovered "self" url instead of the passed topic.