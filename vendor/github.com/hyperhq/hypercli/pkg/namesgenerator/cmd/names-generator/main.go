package main

import (
	"fmt"

	"github.com/hyperhq/hypercli/pkg/namesgenerator"
)

func main() {
	fmt.Println(namesgenerator.GetRandomName(0))
}
