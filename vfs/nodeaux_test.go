package vfs

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAux(t *testing.T) {
	var a aux
	owner1, owner2 := new(int), new(int)

	// Nothing attached yet
	assert.Nil(t, a.Aux(owner1))
	assert.Nil(t, a.Sys())

	// Values attached by different owners are independent even if
	// they have different types
	a.SetAux(owner1, "potato")
	a.SetAux(owner2, 2)
	assert.Equal(t, "potato", a.Aux(owner1))
	assert.Equal(t, 2, a.Aux(owner2))

	// Replace a value
	a.SetAux(owner1, "sausage")
	assert.Equal(t, "sausage", a.Aux(owner1))

	// Remove a value
	a.SetAux(owner1, nil)
	assert.Nil(t, a.Aux(owner1))
	assert.Equal(t, 2, a.Aux(owner2))

	// Sys is independent of the other owners
	assert.Nil(t, a.Sys())
	a.SetSys(42)
	assert.Equal(t, 42, a.Sys())
	assert.Equal(t, 2, a.Aux(owner2))

	// Changing the type of the value stored must not panic
	a.SetSys("42")
	assert.Equal(t, "42", a.Sys())

	// Remove the remaining values
	a.SetSys(nil)
	a.SetAux(owner2, nil)
	assert.Nil(t, a.entries.Load())
}

func TestAuxConcurrent(t *testing.T) {
	const (
		owners     = 4
		iterations = 100
	)
	var (
		a  aux
		wg sync.WaitGroup
	)
	for i := range owners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			owner := &i
			for j := range iterations {
				value := fmt.Sprintf("%d-%d", i, j)
				a.SetAux(owner, value)
				assert.Equal(t, value, a.Aux(owner))
			}
		}()
	}
	wg.Wait()
}
