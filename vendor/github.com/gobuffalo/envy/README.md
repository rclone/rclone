# envy
[![Build Status](https://travis-ci.org/gobuffalo/envy.svg?branch=master)](https://travis-ci.org/gobuffalo/envy)

Envy makes working with ENV variables in Go trivial.

* Get ENV variables with default values.
* Set ENV variables safely without affecting the underlying system.
* Temporarily change ENV vars; useful for testing.
* Map all of the key/values in the ENV.
* Loads .env files (by using [godotenv](https://github.com/joho/godotenv/))
* More!

## Installation

```text
$ go get -u github.com/gobuffalo/envy
```

## Usage

```go
func Test_Get(t *testing.T) {
	r := require.New(t)
	r.NotZero(os.Getenv("GOPATH"))
	r.Equal(os.Getenv("GOPATH"), envy.Get("GOPATH", "foo"))
	r.Equal("bar", envy.Get("IDONTEXIST", "bar"))
}

func Test_MustGet(t *testing.T) {
	r := require.New(t)
	r.NotZero(os.Getenv("GOPATH"))
	v, err := envy.MustGet("GOPATH")
	r.NoError(err)
	r.Equal(os.Getenv("GOPATH"), v)

	_, err = envy.MustGet("IDONTEXIST")
	r.Error(err)
}

func Test_Set(t *testing.T) {
	r := require.New(t)
	_, err := envy.MustGet("FOO")
	r.Error(err)

	envy.Set("FOO", "foo")
	r.Equal("foo", envy.Get("FOO", "bar"))
}

func Test_Temp(t *testing.T) {
	r := require.New(t)

	_, err := envy.MustGet("BAR")
	r.Error(err)

	envy.Temp(func() {
		envy.Set("BAR", "foo")
		r.Equal("foo", envy.Get("BAR", "bar"))
		_, err = envy.MustGet("BAR")
		r.NoError(err)
	})

	_, err = envy.MustGet("BAR")
	r.Error(err)
}
```
## .env files support

Envy now supports loading `.env` files by using the [godotenv library](https://github.com/joho/godotenv/).
That means one can use and define multiple `.env` files which will be loaded on-demand. By default, no env files will be loaded. To load one or more, you need to call the `envy.Load` function in one of the following ways:

```go
envy.Load() // 1

envy.Load("MY_ENV_FILE") // 2

envy.Load(".env", ".env.prod") // 3

envy.Load(".env", "NON_EXISTING_FILE") // 4

// 5
envy.Load(".env")
envy.Load("NON_EXISTING_FILE")

// 6
envy.Load(".env", "NON_EXISTING_FILE", ".env.prod")
```

1. Will load the default `.env` file
2. Will load the file `MY_ENV_FILE`, **but not** `.env`
3. Will load the file `.env`, and after that will load the `.env.prod` file. If any variable is redefined in `. env.prod` it will be overwritten (will contain the `env.prod` value)
4. Will load the `.env` file and return an error as the second file does not exist. The values in `.env` will be loaded and available.
5. Same as 4
6. Will load the `.env` file and return an error as the second file does not exist. The values in `.env` will be loaded and available, **but the ones in** `.env.prod` **won't**.
