# gqlformatter

This is a GraphQL formatter, based on [vektah/gqlparser](https://github.com/vektah/gqlparser).

The main objective is to provide a line by line output of GraphQL so it's easy to spot changes in committed queries.

Example use:

```go
package main

import (
    "fmt"
    "log"

    "github.com/diegosz/gqlformatter"
)

func main() {
    s := "query{products(where:{and:{id:{gte:20} label:{eq:$label}}}){id name price}}"
    q, err := gqlformatter.FormatQuery(s)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(q)
}
```

```graphql
query {
  products(
    where: {
      and: {
        id: { gte: 20 }
        label: { eq: $label }
      }
    }
  ) {
    id
    name
    price
  }
}
```
