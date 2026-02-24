package backend

import (
	"flag"
	"fmt"

	"github.com/rclone/go-proton-api"
)

func (s *Backend) RunQuarkCommand(command string, args ...string) (any, error) {
	switch command {
	case "encryption:id":
		return s.quarkEncryptionID(args...)

	case "user:create":
		return s.quarkUserCreate(args...)

	case "user:create:address":
		return s.quarkUserCreateAddress(args...)

	case "user:create:subscription":
		return s.quarkUserCreateSubscription(args...)

	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

func (s *Backend) quarkEncryptionID(args ...string) (string, error) {
	fs := flag.NewFlagSet("encryption:id", flag.ContinueOnError)

	// Positional arguments.
	// arg0: value

	decrypt := fs.Bool("decrypt", false, "decrypt the given encrypted ID")

	if err := fs.Parse(args); err != nil {
		return "", err
	}

	// TODO: Encrypt/decrypt are currently no-op.
	if *decrypt {
		return fs.Arg(0), nil
	} else {
		return fs.Arg(0), nil
	}
}

func (s *Backend) quarkUserCreate(args ...string) (proton.User, error) {
	fs := flag.NewFlagSet("user:create", flag.ContinueOnError)

	// Flag arguments.
	name := fs.String("name", "", "new user's name")
	pass := fs.String("password", "", "new user's password")
	newAddr := fs.Bool("create-address", false, "create the user's default address, will not automatically setup the address key")
	genKeys := fs.String("gen-keys", "", "generate new address keys for the user")
	status := fs.Int("status", 2, "User status")

	if err := fs.Parse(args); err != nil {
		return proton.User{}, err
	}

	addressStatus, err := quarkStatusToAddressStatus(*status)
	if err != nil {
		return proton.User{}, err
	}

	userID, err := s.CreateUser(*name, []byte(*pass))
	if err != nil {
		return proton.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	// TODO: Create keys of different types (we always use RSA2048).
	if *newAddr || *genKeys != "" {
		if _, err := s.CreateAddress(userID, *name+"@"+s.domain, []byte(*pass), *genKeys != "", addressStatus, proton.AddressTypeOriginal); err != nil {
			return proton.User{}, fmt.Errorf("failed to create address with keys: %w", err)
		}
	}

	return s.GetUser(userID)
}

func (s *Backend) quarkUserCreateAddress(args ...string) (proton.Address, error) {
	fs := flag.NewFlagSet("user:create:address", flag.ContinueOnError)

	// Positional arguments.
	// arg0: userID
	// arg1: password
	// arg2: email

	// Flag arguments.
	genKeys := fs.String("gen-keys", "", "generate new address keys for the user")
	status := fs.Int("status", 2, "User status")

	if err := fs.Parse(args); err != nil {
		return proton.Address{}, err
	}

	addressStatus, err := quarkStatusToAddressStatus(*status)
	if err != nil {
		return proton.Address{}, err
	}

	// TODO: Create keys of different types (we always use RSA2048).
	addrID, err := s.CreateAddress(fs.Arg(0), fs.Arg(2), []byte(fs.Arg(1)), *genKeys != "", addressStatus, proton.AddressTypeOriginal)
	if err != nil {
		return proton.Address{}, fmt.Errorf("failed to create address with keys: %w", err)
	}

	return s.GetAddress(fs.Arg(0), addrID)
}

func (s *Backend) quarkUserCreateSubscription(args ...string) (any, error) {
	fs := flag.NewFlagSet("user:create:subscription", flag.ContinueOnError)

	// Positional arguments.
	// arg0: userID

	// Flag arguments.
	planID := fs.String("planID", "", "plan ID for the user")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if err := s.CreateSubscription(fs.Arg(0), *planID); err != nil {
		return proton.Address{}, fmt.Errorf("failed to create subscription: %w", err)
	}

	return nil, nil
}

func quarkStatusToAddressStatus(status int) (proton.AddressStatus, error) {
	switch status {
	case 0:
		return proton.AddressStatusDeleting, nil
	case 1:
		return proton.AddressStatusDisabled, nil
	case 2:
		fallthrough
	case 3:
		fallthrough
	case 4:
		fallthrough
	case 5:
		return proton.AddressStatusEnabled, nil
	}

	return 0, fmt.Errorf("invalid status value")
}
