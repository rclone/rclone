# go-vcard

[![godocs.io](http://godocs.io/github.com/emersion/go-vcard?status.svg)](http://godocs.io/github.com/emersion/go-vcard)
[![builds.sr.ht status](https://builds.sr.ht/~emersion/go-vcard/commits.svg)](https://builds.sr.ht/~emersion/go-vcard/commits?)

A Go library to parse and format [vCard](https://tools.ietf.org/html/rfc6350).

## Usage

```go
f, err := os.Open("cards.vcf")
if err != nil {
	log.Fatal(err)
}
defer f.Close()

dec := vcard.NewDecoder(f)
for {
	card, err := dec.Decode()
	if err == io.EOF {
		break
	} else if err != nil {
		log.Fatal(err)
	}

	log.Println(card.PreferredValue(vcard.FieldFormattedName))
}
```

## License

MIT
