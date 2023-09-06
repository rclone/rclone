# infectious

[![GoDoc](https://godoc.org/github.com/vivint/infectious?status.png)](https://godoc.org/github.com/vivint/infectious)

Infectious implements
[Reed-Solomon forward error correction](https://en.wikipedia.org/wiki/Reed%E2%80%93Solomon_error_correction).
It uses the
[Berlekamp-Welch error correction algorithm](https://en.wikipedia.org/wiki/Berlekamp%E2%80%93Welch_algorithm)
to achieve the ability to actually correct errors.

[We wrote a blog post about how this library works!](https://innovation.vivint.com/introduction-to-reed-solomon-bc264d0794f8)

### Example

```golang
const (
	required = 8
	total    = 14
)

// Create a *FEC, which will require required pieces for reconstruction at
// minimum, and generate total total pieces.
f, err := infectious.NewFEC(required, total)
if err != nil {
	panic(err)
}

// Prepare to receive the shares of encoded data.
shares := make([]infectious.Share, total)
output := func(s infectious.Share) {
	// the memory in s gets reused, so we need to make a deep copy
	shares[s.Number] = s.DeepCopy()
}

// the data to encode must be padded to a multiple of required, hence the
// underscores.
err = f.Encode([]byte("hello, world! __"), output)
if err != nil {
	panic(err)
}

// we now have total shares.
for _, share := range shares {
	fmt.Printf("%d: %#v\n", share.Number, string(share.Data))
}

// Let's reconstitute with two pieces missing and one piece corrupted.
shares = shares[2:]     // drop the first two pieces
shares[2].Data[1] = '!' // mutate some data

result, err := f.Decode(nil, shares)
if err != nil {
	panic(err)
}

// we have the original data!
fmt.Printf("got: %#v\n", string(result))
```

**Caution:** this package API leans toward providing the user more power and
performance at the expense of having some really sharp edges! Read the
documentation about memory lifecycles carefully!

Please see the docs at http://godoc.org/github.com/vivint/infectious

### Thanks

We're forever indebted to the giants on whose shoulders we stand. The LICENSE 
has our full copyright history, but an extra special thanks to Klaus Post for 
much of the initial Go code. See his post for more: 
http://blog.klauspost.com/blazingly-fast-reed-solomon-coding/

### LICENSE

 * Copyright (C) 2016-2017 Vivint, Inc.
 * Copyright (c) 2015 Klaus Post
 * Copyright (c) 2015 Backblaze
 * Copyright (C) 2011 Billy Brumley (billy.brumley@aalto.fi)
 * Copyright (C) 2009-2010 Jack Lloyd (lloyd@randombit.net)
 * Copyright (C) 1996-1998 Luigi Rizzo (luigi@iet.unipi.it)

Portions derived from code by Phil Karn (karn@ka9q.ampr.org),
Robert Morelos-Zaragoza (robert@spectra.eng.hawaii.edu) and Hari
Thirumoorthy (harit@spectra.eng.hawaii.edu), Aug 1995

**Portions of this project (labeled in each file) are licensed under this
license:**

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the
   distribution.

THIS SOFTWARE IS PROVIDED BY THE AUTHORS ``AS IS'' AND ANY EXPRESS OR
IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY DIRECT,
INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.

**All other portions of this project are licensed under this license:**

The MIT License (MIT)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
