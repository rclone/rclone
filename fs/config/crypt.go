package config

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/text/unicode/norm"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
)

var (
	// Key to use for password en/decryption.
	// When nil, no encryption will be used for saving.
	configKey []byte

	// PasswordPromptOutput is output of prompt for password
	PasswordPromptOutput = os.Stderr

	// PassConfigKeyForDaemonization if set to true, the configKey
	// is obscured with obscure.Obscure and saved to a temp file
	// when it is calculated from the password. The path of that
	// temp file is then written to the environment variable
	// `_RCLONE_CONFIG_KEY_FILE`. If `_RCLONE_CONFIG_KEY_FILE` is
	// present, password prompt is skipped and
	// `RCLONE_CONFIG_PASS` ignored. For security reasons, the
	// temp file is deleted once the configKey is successfully
	// loaded. This can be used to pass the configKey to a child
	// process.
	PassConfigKeyForDaemonization = false
)

// Decrypt will automatically decrypt a reader
func Decrypt(b io.ReadSeeker) (io.Reader, error) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	var usingPasswordCommand bool

	// Find first non-empty line
	r := bufio.NewReader(b)
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				if _, err := b.Seek(0, io.SeekStart); err != nil {
					return nil, err
				}
				return b, nil
			}
			return nil, err
		}
		l := strings.TrimSpace(string(line))
		if len(l) == 0 || strings.HasPrefix(l, ";") || strings.HasPrefix(l, "#") {
			continue
		}
		// First non-empty or non-comment must be ENCRYPT_V0
		if l == "RCLONE_ENCRYPT_V0:" {
			break
		}
		if strings.HasPrefix(l, "RCLONE_ENCRYPT_V") {
			return nil, errors.New("unsupported configuration encryption - update rclone for support")
		}
		if _, err := b.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return b, nil
	}

	if len(configKey) == 0 {
		if len(ci.PasswordCommand) != 0 {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			cmd := exec.Command(ci.PasswordCommand[0], ci.PasswordCommand[1:]...)

			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Stdin = os.Stdin

			if err := cmd.Run(); err != nil {
				// One does not always get the stderr returned in the wrapped error.
				fs.Errorf(nil, "Using --password-command returned: %v", err)
				if ers := strings.TrimSpace(stderr.String()); ers != "" {
					fs.Errorf(nil, "--password-command stderr: %s", ers)
				}
				return nil, errors.Wrap(err, "password command failed")
			}
			if pass := strings.Trim(stdout.String(), "\r\n"); pass != "" {
				err := setConfigPassword(pass)
				if err != nil {
					return nil, errors.Wrap(err, "incorrect password")
				}
			} else {
				return nil, errors.New("password-command returned empty string")
			}

			if len(configKey) == 0 {
				return nil, errors.New("unable to decrypt configuration: incorrect password")
			}
			usingPasswordCommand = true
		} else {
			usingPasswordCommand = false

			envpw := os.Getenv("RCLONE_CONFIG_PASS")

			if envpw != "" {
				err := setConfigPassword(envpw)
				if err != nil {
					fs.Errorf(nil, "Using RCLONE_CONFIG_PASS returned: %v", err)
				} else {
					fs.Debugf(nil, "Using RCLONE_CONFIG_PASS password.")
				}
			}
		}
	}

	// Encrypted content is base64 encoded.
	dec := base64.NewDecoder(base64.StdEncoding, r)
	box, err := ioutil.ReadAll(dec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load base64 encoded data")
	}
	if len(box) < 24+secretbox.Overhead {
		return nil, errors.New("Configuration data too short")
	}

	var out []byte
	for {
		if envKeyFile := os.Getenv("_RCLONE_CONFIG_KEY_FILE"); len(envKeyFile) > 0 {
			fs.Debugf(nil, "attempting to obtain configKey from temp file %s", envKeyFile)
			obscuredKey, err := ioutil.ReadFile(envKeyFile)
			if err != nil {
				errRemove := os.Remove(envKeyFile)
				if errRemove != nil {
					log.Fatalf("unable to read obscured config key and unable to delete the temp file: %v", err)
				}
				log.Fatalf("unable to read obscured config key: %v", err)
			}
			errRemove := os.Remove(envKeyFile)
			if errRemove != nil {
				log.Fatalf("unable to delete temp file with configKey: %v", err)
			}
			configKey = []byte(obscure.MustReveal(string(obscuredKey)))
			fs.Debugf(nil, "using _RCLONE_CONFIG_KEY_FILE for configKey")
		} else {
			if len(configKey) == 0 {
				if usingPasswordCommand {
					return nil, errors.New("using --password-command derived password, unable to decrypt configuration")
				}
				if !ci.AskPassword {
					return nil, errors.New("unable to decrypt configuration and not allowed to ask for password - set RCLONE_CONFIG_PASS to your configuration password")
				}
				getConfigPassword("Enter configuration password:")
			}
		}

		// Nonce is first 24 bytes of the ciphertext
		var nonce [24]byte
		copy(nonce[:], box[:24])
		var key [32]byte
		copy(key[:], configKey[:32])

		// Attempt to decrypt
		var ok bool
		out, ok = secretbox.Open(nil, box[24:], &nonce, &key)
		if ok {
			break
		}

		// Retry
		fs.Errorf(nil, "Couldn't decrypt configuration, most likely wrong password.")
		configKey = nil
	}
	return bytes.NewReader(out), nil
}

// Encrypt the config file
func Encrypt(src io.Reader, dst io.Writer) error {
	if len(configKey) == 0 {
		_, err := io.Copy(dst, src)
		return err
	}

	_, _ = fmt.Fprintln(dst, "# Encrypted rclone configuration File")
	_, _ = fmt.Fprintln(dst, "")
	_, _ = fmt.Fprintln(dst, "RCLONE_ENCRYPT_V0:")

	// Generate new nonce and write it to the start of the ciphertext
	var nonce [24]byte
	n, _ := rand.Read(nonce[:])
	if n != 24 {
		return errors.Errorf("nonce short read: %d", n)
	}
	enc := base64.NewEncoder(base64.StdEncoding, dst)
	_, err := enc.Write(nonce[:])
	if err != nil {
		return errors.Errorf("Failed to write config file: %v", err)
	}

	var key [32]byte
	copy(key[:], configKey[:32])

	data, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}
	b := secretbox.Seal(nil, data, &nonce, &key)
	_, err = enc.Write(b)
	if err != nil {
		return errors.Errorf("Failed to write config file: %v", err)
	}
	return enc.Close()
}

// checkPassword normalises and validates the password
func checkPassword(password string) (string, error) {
	if !utf8.ValidString(password) {
		return "", errors.New("password contains invalid utf8 characters")
	}
	// Check for leading/trailing whitespace
	trimmedPassword := strings.TrimSpace(password)
	// Warn user if password has leading+trailing whitespace
	if len(password) != len(trimmedPassword) {
		_, _ = fmt.Fprintln(os.Stderr, "Your password contains leading/trailing whitespace - in previous versions of rclone this was stripped")
	}
	// Normalize to reduce weird variations.
	password = norm.NFKC.String(password)
	if len(password) == 0 || len(trimmedPassword) == 0 {
		return "", errors.New("no characters in password")
	}
	return password, nil
}

// GetPassword asks the user for a password with the prompt given.
func GetPassword(prompt string) string {
	_, _ = fmt.Fprintln(PasswordPromptOutput, prompt)
	for {
		_, _ = fmt.Fprint(PasswordPromptOutput, "password:")
		password := ReadPassword()
		password, err := checkPassword(password)
		if err == nil {
			return password
		}
		_, _ = fmt.Fprintf(os.Stderr, "Bad password: %v\n", err)
	}
}

// ChangePassword will query the user twice for the named password. If
// the same password is entered it is returned.
func ChangePassword(name string) string {
	for {
		a := GetPassword(fmt.Sprintf("Enter %s password:", name))
		b := GetPassword(fmt.Sprintf("Confirm %s password:", name))
		if a == b {
			return a
		}
		fmt.Println("Passwords do not match!")
	}
}

// getConfigPassword will query the user for a password the
// first time it is required.
func getConfigPassword(q string) {
	if len(configKey) != 0 {
		return
	}
	for {
		password := GetPassword(q)
		err := setConfigPassword(password)
		if err == nil {
			return
		}
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
	}
}

// setConfigPassword will set the configKey to the hash of
// the password. If the length of the password is
// zero after trimming+normalization, an error is returned.
func setConfigPassword(password string) error {
	password, err := checkPassword(password)
	if err != nil {
		return err
	}
	// Create SHA256 has of the password
	sha := sha256.New()
	_, err = sha.Write([]byte("[" + password + "][rclone-config]"))
	if err != nil {
		return err
	}
	configKey = sha.Sum(nil)
	if PassConfigKeyForDaemonization {
		tempFile, err := ioutil.TempFile("", "rclone")
		if err != nil {
			log.Fatalf("cannot create temp file to store configKey: %v", err)
		}
		_, err = tempFile.WriteString(obscure.MustObscure(string(configKey)))
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("error writing configKey to temp file and also error deleting it: %v", err)
			}
			log.Fatalf("error writing configKey to temp file: %v", err)
		}
		err = tempFile.Close()
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("error closing temp file with configKey and also error deleting it: %v", err)
			}
			log.Fatalf("error closing temp file with configKey: %v", err)
		}
		fs.Debugf(nil, "saving configKey to temp file")
		err = os.Setenv("_RCLONE_CONFIG_KEY_FILE", tempFile.Name())
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("unable to set environment variable _RCLONE_CONFIG_KEY_FILE and unable to delete the temp file: %v", err)
			}
			log.Fatalf("unable to set environment variable _RCLONE_CONFIG_KEY_FILE: %v", err)
		}
	}
	return nil
}

// changeConfigPassword will query the user twice
// for a password. If the same password is entered
// twice the key is updated.
func changeConfigPassword() {
	err := setConfigPassword(ChangePassword("NEW configuration"))
	if err != nil {
		fmt.Printf("Failed to set config password: %v\n", err)
		return
	}
}

// SetPassword will allow the user to modify the current
// configuration encryption settings.
func SetPassword() {
	for {
		if len(configKey) > 0 {
			fmt.Println("Your configuration is encrypted.")
			what := []string{"cChange Password", "uUnencrypt configuration", "qQuit to main menu"}
			switch i := Command(what); i {
			case 'c':
				changeConfigPassword()
				SaveConfig()
				fmt.Println("Password changed")
				continue
			case 'u':
				configKey = nil
				SaveConfig()
				continue
			case 'q':
				return
			}

		} else {
			fmt.Println("Your configuration is not encrypted.")
			fmt.Println("If you add a password, you will protect your login information to cloud services.")
			what := []string{"aAdd Password", "qQuit to main menu"}
			switch i := Command(what); i {
			case 'a':
				changeConfigPassword()
				SaveConfig()
				fmt.Println("Password set")
				continue
			case 'q':
				return
			}
		}
	}
}
