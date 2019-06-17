package googlephotos

import (
	"testing"

	"github.com/ncw/rclone/backend/googlephotos/api"
	"github.com/stretchr/testify/assert"
)

func TestNewAlbums(t *testing.T) {
	albums := newAlbums()
	assert.NotNil(t, albums.dupes)
	assert.NotNil(t, albums.byID)
	assert.NotNil(t, albums.byTitle)
	assert.NotNil(t, albums.path)
}

func TestAlbumsAdd(t *testing.T) {
	albums := newAlbums()

	assert.Equal(t, map[string][]*api.Album{}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{}, albums.byID)
	assert.Equal(t, map[string]*api.Album{}, albums.byTitle)
	assert.Equal(t, map[string][]string{}, albums.path)

	a1 := &api.Album{
		Title: "one",
		ID:    "1",
	}
	albums.add(a1)

	assert.Equal(t, map[string][]*api.Album{
		"one": []*api.Album{a1},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1": a1,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one": a1,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"": []string{"one"},
	}, albums.path)

	a2 := &api.Album{
		Title: "two",
		ID:    "2",
	}
	albums.add(a2)

	assert.Equal(t, map[string][]*api.Album{
		"one": []*api.Album{a1},
		"two": []*api.Album{a2},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1": a1,
		"2": a2,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one": a1,
		"two": a2,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"": []string{"one", "two"},
	}, albums.path)

	// Add a duplicate
	a2a := &api.Album{
		Title: "two",
		ID:    "2a",
	}
	albums.add(a2a)

	assert.Equal(t, map[string][]*api.Album{
		"one": []*api.Album{a1},
		"two": []*api.Album{a2, a2a},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1":  a1,
		"2":  a2,
		"2a": a2a,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one":      a1,
		"two {2}":  a2,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"": []string{"one", "two {2}", "two {2a}"},
	}, albums.path)

	// Add a sub directory
	a1sub := &api.Album{
		Title: "one/sub",
		ID:    "1sub",
	}
	albums.add(a1sub)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1":    a1,
		"2":    a2,
		"2a":   a2a,
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one":      a1,
		"one/sub":  a1sub,
		"two {2}":  a2,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one", "two {2}", "two {2a}"},
		"one": []string{"sub"},
	}, albums.path)

	// Add a weird path
	a0 := &api.Album{
		Title: "/../././..////.",
		ID:    "0",
	}
	albums.add(a0)

	assert.Equal(t, map[string][]*api.Album{
		"{0}":     []*api.Album{a0},
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"0":    a0,
		"1":    a1,
		"2":    a2,
		"2a":   a2a,
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"{0}":      a0,
		"one":      a1,
		"one/sub":  a1sub,
		"two {2}":  a2,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one", "two {2}", "two {2a}", "{0}"},
		"one": []string{"sub"},
	}, albums.path)
}

func TestAlbumsDel(t *testing.T) {
	albums := newAlbums()

	a1 := &api.Album{
		Title: "one",
		ID:    "1",
	}
	albums.add(a1)

	a2 := &api.Album{
		Title: "two",
		ID:    "2",
	}
	albums.add(a2)

	// Add a duplicate
	a2a := &api.Album{
		Title: "two",
		ID:    "2a",
	}
	albums.add(a2a)

	// Add a sub directory
	a1sub := &api.Album{
		Title: "one/sub",
		ID:    "1sub",
	}
	albums.add(a1sub)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1":    a1,
		"2":    a2,
		"2a":   a2a,
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one":      a1,
		"one/sub":  a1sub,
		"two {2}":  a2,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one", "two {2}", "two {2a}"},
		"one": []string{"sub"},
	}, albums.path)

	albums.del(a1)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"2":    a2,
		"2a":   a2a,
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one/sub":  a1sub,
		"two {2}":  a2,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one", "two {2}", "two {2a}"},
		"one": []string{"sub"},
	}, albums.path)

	albums.del(a2)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"2a":   a2a,
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one/sub":  a1sub,
		"two {2a}": a2a,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one", "two {2a}"},
		"one": []string{"sub"},
	}, albums.path)

	albums.del(a2a)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{
		"1sub": a1sub,
	}, albums.byID)
	assert.Equal(t, map[string]*api.Album{
		"one/sub": a1sub,
	}, albums.byTitle)
	assert.Equal(t, map[string][]string{
		"":    []string{"one"},
		"one": []string{"sub"},
	}, albums.path)

	albums.del(a1sub)

	assert.Equal(t, map[string][]*api.Album{
		"one":     []*api.Album{a1},
		"two":     []*api.Album{a2, a2a},
		"one/sub": []*api.Album{a1sub},
	}, albums.dupes)
	assert.Equal(t, map[string]*api.Album{}, albums.byID)
	assert.Equal(t, map[string]*api.Album{}, albums.byTitle)
	assert.Equal(t, map[string][]string{}, albums.path)
}

func TestAlbumsGet(t *testing.T) {
	albums := newAlbums()

	a1 := &api.Album{
		Title: "one",
		ID:    "1",
	}
	albums.add(a1)

	album, ok := albums.get("one")
	assert.Equal(t, true, ok)
	assert.Equal(t, a1, album)

	album, ok = albums.get("notfound")
	assert.Equal(t, false, ok)
	assert.Nil(t, album)
}

func TestAlbumsGetDirs(t *testing.T) {
	albums := newAlbums()

	a1 := &api.Album{
		Title: "one",
		ID:    "1",
	}
	albums.add(a1)

	dirs, ok := albums.getDirs("")
	assert.Equal(t, true, ok)
	assert.Equal(t, []string{"one"}, dirs)

	dirs, ok = albums.getDirs("notfound")
	assert.Equal(t, false, ok)
	assert.Nil(t, dirs)
}
