# deheap

Package deheap provides the implementation of a doubly ended heap.
Doubly ended heaps are heaps with two sides, a min side and a max side.
Like normal single-sided heaps, elements can be pushed onto and pulled
off of a deheap.  deheaps have an additional `Pop` function, `PopMax`, that
returns elements from the opposite side of the ordering.

This implementation has emphasized compatibility with existing libraries
in the sort and heap packages.

Performace of the deheap functions should be very close to the
performance of the functions of the heap library

