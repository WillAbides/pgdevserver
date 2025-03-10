# pgdevserver

Pgdevserver is a command line tool for managing ephemeral postgres servers in a
development environment.

When you start a server, pgdevserver will:

- If an existing server meets your requirements it outputs a that server's connection url and exits.
- Checks for the necessary postgres binaries and downloads them if required.
- Creates and starts a new server.
- Outputs a connection url for the new server.



## Install

### With [bindown](https://github.com/WillAbides/bindown)

```shell
bindown template-source add pgdevserver https://github.com/WillAbides/pgdevserver/releases/latest/download/bindown.yaml
bindown dependency add pgdevserver --source pgdevserver -y
```

### With Go

```shell
go install github.com/willabides/pgdevserver@latest
```

### From Releases

Download
the [latest release](https://github.com/willabides/pgdevserver/releases/latest)
for your platform, extract and do whatever you normally do with a binary.

## Usage

<!--- everything between the next line and the "end usage output" comment is generated by script/generate-readme --->
<!--- start usage output --->

```
Usage: pgdevserver <command> [flags]

Flags:
  -h, --help    Show context-sensitive help.

Commands:
  start [flags]
    Start a server.

  create [flags]
    Create a server without starting it.

  list [flags]
    List servers.

  stop [flags]
    Stop a server.

  rm [flags]
    Remove a server.

  pg list [flags]
    List installed postgres versions.

  pg available [flags]
    List postgres versions available to download.

  pg install <version> [flags]
    Install a postgres version.

  pg rm <version> [flags]
    Remove a postgres version.

Run "pgdevserver <command> --help" for more information on a command.
```

<!--- end usage output --->
