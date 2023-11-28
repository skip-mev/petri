# petri (alpha) - an abstraction layer for infrastructure providers

## how to use

```bash
go get github.com/skip-mev/petri
```

add to your code:

```go
import (
  "github.com/skip-mev/petri/provider/docker"
  "github.com/skip-mev/petri/provider"

  "context"
)

ctx := context.Background()

dockerProvider, err := docker.CreateProvider() // more providers to be added later

if err != nil {
  panic(err)
}

task, err := provider.CreateTask(context.Background(), dockerProvider, provider.TaskDefinition{
  Image: provider.ImageDefinition {
    Name: "nginx:latest",
    UID:  "101",
    GID:  "101",
  },
  Name: "petri-test"
})

if err != nil {
  panic(err)
}

err := task.Start(ctx, false)

if err != nil {
  panic(err)
}
```
