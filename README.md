# golang-helpers

Common Golang helpers

<!-- toc -->

* [Logger](#logger)
* [Contributing](#contributing)
  * [Open in Gitpod](#open-in-gitpod)
  * [Open in a container](#open-in-a-container)

<!-- Regenerate with "pre-commit run -a markdown-toc" -->

<!-- tocstop -->

## Logger

This is useful for getting a custom log level in a Cobra program.

```go
package cmd

import "github.com/mrsimonemms/golang-helpers/logger"

var logLevel string

var root = &cobra.Command{
  PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    return logger.SetLevel(logLevel)
  },
}

func init() {
  rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", logrus.InfoLevel.String(), fmt.Sprintf("log level: %s", logger.GetAllLevels()))
}
```

## Contributing

### Open in Gitpod

* [Open in Gitpod](https://gitpod.io/from-referrer/)

### Open in a container

* [Open in a container](https://code.visualstudio.com/docs/devcontainers/containers)
