# saferith

The purpose of this package is to provide a version of arbitrary sized
arithmetic, in a safer (i.e. constant-time) way, for cryptography.

*This is experimental software, use at your own peril*.

# Assembly

This code reuses some assembly routines from Go's standard library,
inside of the `arith*.go`. These have been adjusted to remove some
non-constant-time codepaths, most of which aren't used anyways.

# Integrating with Go

Initially, this code was structured to be relatively straightforwardly
patched into Go's standard library. The idea would be to use the `arith*.go`
files already in Go's `math/big` package, and just add a `num.go` file.

Unfortunately, this approach doesn't seem to be possible, because of
`addVWlarge` and `subVWlarge`, which are two non-constant time routines.
These are jumped to inside of the assembly code in Go's `math/big` routines,
so using them would require intrusive modification, which rules out
this code living alongside `math/big`, and sharing its routines.

## Merging things upstream

The easiest path towards merging this work upstream, in all likelihood,
is having this package live in `crypto`, and duplicating some of
the assembly code as necessary.

The rationale here is that `math/big`'s needs will inevitably lead to situations
like this, where a routine is tempted to bail towards a non-constant time
variant for large or special inputs. Ultimately, having this code live
in `crypto` is much more likely to allow us to ensure its integrity.
It would also allow us to add assembly specifically tailored for
our operations, such as conditional addition, and things like that.

# Benchmarks

Run with assembly routines:

```
go test -bench=.
```

Run with pure Go code:

```
go test -bench=. -tags math_big_pure_go
```

# Licensing

The files `arith*.go` come from Go's standard library, and are licensed under
a BSD license in `LICENSE_go`. The rest of the code is under an MIT license.
