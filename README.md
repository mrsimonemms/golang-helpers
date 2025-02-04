# golang-helpers

Common Golang helpers

<!-- toc -->

* [gRPC](#grpc)
  * [Root](#root)
  * [Run](#run)
  * [Example](#example)
* [Logger](#logger)
* [Contributing](#contributing)
  * [Open in a container](#open-in-a-container)

<!-- Regenerate with "pre-commit run -a markdown-toc" -->

<!-- tocstop -->

## gRPC

Because sometimes you need to be able to debug gRPC services in a standalone manner.

I love gRPC. It allows me to build apps in a way that I like - a [NestJS](https://nestjs.com)
control plane with Golang microservices that do all the hard work. This allows
me to build reliable services that can be widely scaled.

One problem with gRPC is that it can be hard to invoke functions to do either
end-to-end tests or actually check how something works. So I created a helper
library that creates a simple [Cobra](https://cobra.dev) app with two commands:

### Root

```sh
Usage:
  go run .
```

This is the command to use in production. This command creates the standard
gRPC server with both [Reflection](https://grpc.io/docs/guides/reflection) and
[Health Checks](https://grpc.io/docs/guides/health-checking) enabled by default.

### Run

```sh
Usage:
  go run . run <cmd> <args>
```

This is the command to use in development. This sets up an individual gRPC
command as a Cobra command. You can the gRPC inputs via [Flags](https://github.com/spf13/cobra?tab=readme-ov-file#flags)
and use sensible defaults where necessary.

The response from the implementation is printed to your terminal, including any
sensitive information, so this should be used for local development only.

### Example

[Example application](./examples/grpc/basic/)

## Logger

This is useful for getting a custom log level in a Cobra program.

```go
package cmd

import "github.com/mrsimonemms/golang-helpers/logger"

var logLevel string

var rootCmd = &cobra.Command{
  PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    return logger.SetLevel(logLevel)
  },
}

func init() {
  rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", logrus.InfoLevel.String(), fmt.Sprintf("log level: %s", logger.GetAllLevels()))
}
```

## Contributing

### Open in a container

* [Open in a container](https://code.visualstudio.com/docs/devcontainers/containers)
