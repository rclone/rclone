# Writing documentation

## Sequence diagrams

The sequence diagrams are generated with `seqdiag`:
http://blockdiag.com/en/seqdiag/index.html

An easy way to work on them is to automatically update the generated
files with https://github.com/cespare/reflex :

    reflex -g 'doc/[^.]*.seq' -- seqdiag -T svg -o '{}.svg' '{}' &

    reflex -g 'doc/[^.]*.seq' -- seqdiag -T png -o '{}.png' '{}' &

The markdown files refer to PNG images because of Github limitations,
but the SVG is generally more pleasant to view.
