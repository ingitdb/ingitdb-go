# inGitDB server

`inGitDB` can be started to serve as an API and/or MCP serve.

## MCP server

To start an MCP server, run:

```shell
ingitdb serve --mcp [--mcp-port=8080] [--mcp-domains=mcp.ingitdb.com,localhost]
```

## HTTP(s) server

To start an HTTP server, run:

```shell
ingitdb serve --http [--api-port=8080] [--api-domains=api.ingitdb.com,localhost]
```

- `--api-port` – _optional_ parameter for a port to use for an HTTP connection, if not set defaults to `8080`.
- `--api-domain` – _optional_ paramer for domain names to be used for hosting API,
  if not defaults to `"localhost,api.ingitdb.com"`

## Enabling HTTPS

**TODO**: _Needs instructions on how to enable HTTPS connections (_and probably implementation_)._

## Public inGitDB servers

- [**api**.ingitdb.com](https://api.ingitdb.com) –
  query and modify inGitDBs in public and private GitHub repositories using REST API.
- [**mcp**.ingitdb.com](https://mcp.ingitdb.com) –
  grant AI agents access to inGitDBs in public and private GitHub repositories. 
