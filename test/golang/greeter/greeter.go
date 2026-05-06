package greeter

import "fmt"

// Greet returns a friendly greeting for the given name.
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
