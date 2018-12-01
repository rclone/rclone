package parser

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// FromArgs is useful when writing packr store-cmd binaries.
/*
	package main

	import (
		"log"
		"os"

		"github.com/gobuffalo/packr/v2/jam/parser"
		"github.com/markbates/s3packr/s3packr"
	)

	func main() {
		err := parser.FromArgs(os.Args[1:], func(boxes parser.Boxes) error {
			for _, box := range boxes {
				s3 := s3packr.New(box)
				if err := s3.Pack(box); err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			log.Fatal(err)
		}
	}
*/
func FromArgs(args []string, fn func(Boxes) error) error {
	if len(args) == 0 {
		return errors.New("you must supply a payload")
	}
	payload := args[0]
	if len(payload) == 0 {
		return errors.New("you must supply a payload")
	}

	var boxes Boxes
	err := json.Unmarshal([]byte(payload), &boxes)
	if err != nil {
		return err
	}

	return fn(boxes)
}
