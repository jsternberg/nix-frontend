package main

import (
	"fmt"
	"os"

	"example.com/sampleapp/greeter"
	"example.com/sampleapp/mathutil"
)

func main() {
	name := "world"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	fmt.Println(greeter.Greet(name))
	fmt.Printf("2 + 3 = %d\n", mathutil.Add(2, 3))
	fmt.Printf("4 * 5 = %d\n", mathutil.Multiply(4, 5))
}
