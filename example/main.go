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
