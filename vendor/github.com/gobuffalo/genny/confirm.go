package genny

import (
	"bufio"
	"fmt"
	"os"
)

func Confirm(msg string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(msg)
	text, _ := reader.ReadString('\n')

	return (text == "y\n" || text == "Y\n")
}
