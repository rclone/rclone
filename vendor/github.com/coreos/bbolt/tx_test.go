package bolt_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/coreos/bbolt"
)

// TestTx_Check_ReadOnly tests consistency checking on a ReadOnly database.
func TestTx_Check_ReadOnly(t *testing.T) {
	db := MustOpenDB()
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Close(); err != nil {
		t.Fatal(err)
	}

	readOnlyDB, err := bolt.Open(db.f, 0666, &bolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer readOnlyDB.Close()

	tx, err := readOnlyDB.Begin(false)
	if err != nil {
		t.Fatal(err)
	}
	// ReadOnly DB will load freelist on Check call.
	numChecks := 2
	errc := make(chan error, numChecks)
	check := func() {
		err, _ := <-tx.Check()
		errc <- err
	}
	// Ensure the freelist is not reloaded and does not race.
	for i := 0; i < numChecks; i++ {
		go check()
	}
	for i := 0; i < numChecks; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
}

// Ensure that committing a closed transaction returns an error.
func TestTx_Commit_ErrTxClosed(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	tx, err := db.Begin(true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.CreateBucket([]byte("foo")); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(); err != bolt.ErrTxClosed {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Ensure that rolling back a closed transaction returns an error.
func TestTx_Rollback_ErrTxClosed(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	tx, err := db.Begin(true)
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != bolt.ErrTxClosed {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Ensure that committing a read-only transaction returns an error.
func TestTx_Commit_ErrTxNotWritable(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	tx, err := db.Begin(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != bolt.ErrTxNotWritable {
		t.Fatal(err)
	}
}

// Ensure that a transaction can retrieve a cursor on the root bucket.
func TestTx_Cursor(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}

		if _, err := tx.CreateBucket([]byte("woojits")); err != nil {
			t.Fatal(err)
		}

		c := tx.Cursor()
		if k, v := c.First(); !bytes.Equal(k, []byte("widgets")) {
			t.Fatalf("unexpected key: %v", k)
		} else if v != nil {
			t.Fatalf("unexpected value: %v", v)
		}

		if k, v := c.Next(); !bytes.Equal(k, []byte("woojits")) {
			t.Fatalf("unexpected key: %v", k)
		} else if v != nil {
			t.Fatalf("unexpected value: %v", v)
		}

		if k, v := c.Next(); k != nil {
			t.Fatalf("unexpected key: %v", k)
		} else if v != nil {
			t.Fatalf("unexpected value: %v", k)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that creating a bucket with a read-only transaction returns an error.
func TestTx_CreateBucket_ErrTxNotWritable(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.View(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("foo"))
		if err != bolt.ErrTxNotWritable {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that creating a bucket on a closed transaction returns an error.
func TestTx_CreateBucket_ErrTxClosed(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	tx, err := db.Begin(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.CreateBucket([]byte("foo")); err != bolt.ErrTxClosed {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Ensure that a Tx can retrieve a bucket.
func TestTx_Bucket(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}
		if tx.Bucket([]byte("widgets")) == nil {
			t.Fatal("expected bucket")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a Tx retrieving a non-existent key returns nil.
func TestTx_Get_NotFound(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}

		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		if b.Get([]byte("no_such_key")) != nil {
			t.Fatal("expected nil value")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a bucket can be created and retrieved.
func TestTx_CreateBucket(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	// Create a bucket.
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		} else if b == nil {
			t.Fatal("expected bucket")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Read the bucket through a separate transaction.
	if err := db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("widgets")) == nil {
			t.Fatal("expected bucket")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a bucket can be created if it doesn't already exist.
func TestTx_CreateBucketIfNotExists(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		// Create bucket.
		if b, err := tx.CreateBucketIfNotExists([]byte("widgets")); err != nil {
			t.Fatal(err)
		} else if b == nil {
			t.Fatal("expected bucket")
		}

		// Create bucket again.
		if b, err := tx.CreateBucketIfNotExists([]byte("widgets")); err != nil {
			t.Fatal(err)
		} else if b == nil {
			t.Fatal("expected bucket")
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Read the bucket through a separate transaction.
	if err := db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("widgets")) == nil {
			t.Fatal("expected bucket")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure transaction returns an error if creating an unnamed bucket.
func TestTx_CreateBucketIfNotExists_ErrBucketNameRequired(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte{}); err != bolt.ErrBucketNameRequired {
			t.Fatalf("unexpected error: %s", err)
		}

		if _, err := tx.CreateBucketIfNotExists(nil); err != bolt.ErrBucketNameRequired {
			t.Fatalf("unexpected error: %s", err)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a bucket cannot be created twice.
func TestTx_CreateBucket_ErrBucketExists(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	// Create a bucket.
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Create the same bucket again.
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucket([]byte("widgets")); err != bolt.ErrBucketExists {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a bucket is created with a non-blank name.
func TestTx_CreateBucket_ErrBucketNameRequired(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucket(nil); err != bolt.ErrBucketNameRequired {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that a bucket can be deleted.
func TestTx_DeleteBucket(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	// Create a bucket and add a value.
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Delete the bucket and make sure we can't get the value.
	if err := db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}
		if tx.Bucket([]byte("widgets")) != nil {
			t.Fatal("unexpected bucket")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		// Create the bucket again and make sure there's not a phantom value.
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if v := b.Get([]byte("foo")); v != nil {
			t.Fatalf("unexpected phantom value: %v", v)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that deleting a bucket on a closed transaction returns an error.
func TestTx_DeleteBucket_ErrTxClosed(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	tx, err := db.Begin(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := tx.DeleteBucket([]byte("foo")); err != bolt.ErrTxClosed {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Ensure that deleting a bucket with a read-only transaction returns an error.
func TestTx_DeleteBucket_ReadOnly(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.View(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte("foo")); err != bolt.ErrTxNotWritable {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that nothing happens when deleting a bucket that doesn't exist.
func TestTx_DeleteBucket_NotFound(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte("widgets")); err != bolt.ErrBucketNotFound {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that no error is returned when a tx.ForEach function does not return
// an error.
func TestTx_ForEach_NoError(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}

		if err := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that an error is returned when a tx.ForEach function returns an error.
func TestTx_ForEach_WithError(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}

		marker := errors.New("marker")
		if err := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			return marker
		}); err != marker {
			t.Fatalf("unexpected error: %s", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Ensure that Tx commit handlers are called after a transaction successfully commits.
func TestTx_OnCommit(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	var x int
	if err := db.Update(func(tx *bolt.Tx) error {
		tx.OnCommit(func() { x += 1 })
		tx.OnCommit(func() { x += 2 })
		if _, err := tx.CreateBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	} else if x != 3 {
		t.Fatalf("unexpected x: %d", x)
	}
}

// Ensure that Tx commit handlers are NOT called after a transaction rolls back.
func TestTx_OnCommit_Rollback(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	var x int
	if err := db.Update(func(tx *bolt.Tx) error {
		tx.OnCommit(func() { x += 1 })
		tx.OnCommit(func() { x += 2 })
		if _, err := tx.CreateBucket([]byte("widgets")); err != nil {
			t.Fatal(err)
		}
		return errors.New("rollback this commit")
	}); err == nil || err.Error() != "rollback this commit" {
		t.Fatalf("unexpected error: %s", err)
	} else if x != 0 {
		t.Fatalf("unexpected x: %d", x)
	}
}

// Ensure that the database can be copied to a file path.
func TestTx_CopyFile(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()

	path := tempfile()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("baz"), []byte("bat")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(path, 0600)
	}); err != nil {
		t.Fatal(err)
	}

	db2, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := db2.View(func(tx *bolt.Tx) error {
		if v := tx.Bucket([]byte("widgets")).Get([]byte("foo")); !bytes.Equal(v, []byte("bar")) {
			t.Fatalf("unexpected value: %v", v)
		}
		if v := tx.Bucket([]byte("widgets")).Get([]byte("baz")); !bytes.Equal(v, []byte("bat")) {
			t.Fatalf("unexpected value: %v", v)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}
}

type failWriterError struct{}

func (failWriterError) Error() string {
	return "error injected for tests"
}

type failWriter struct {
	// fail after this many bytes
	After int
}

func (f *failWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > f.After {
		n = f.After
		err = failWriterError{}
	}
	f.After -= n
	return n, err
}

// Ensure that Copy handles write errors right.
func TestTx_CopyFile_Error_Meta(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("baz"), []byte("bat")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *bolt.Tx) error {
		return tx.Copy(&failWriter{})
	}); err == nil || err.Error() != "meta 0 copy: error injected for tests" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Ensure that Copy handles write errors right.
func TestTx_CopyFile_Error_Normal(t *testing.T) {
	db := MustOpenDB()
	defer db.MustClose()
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			t.Fatal(err)
		}
		if err := b.Put([]byte("baz"), []byte("bat")); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *bolt.Tx) error {
		return tx.Copy(&failWriter{3 * db.Info().PageSize})
	}); err == nil || err.Error() != "error injected for tests" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestTx_releaseRange ensures db.freePages handles page releases
// correctly when there are transaction that are no longer reachable
// via any read/write transactions and are "between" ongoing read
// transactions, which requires they must be freed by
// freelist.releaseRange.
func TestTx_releaseRange(t *testing.T) {
	// Set initial mmap size well beyond the limit we will hit in this
	// test, since we are testing with long running read transactions
	// and will deadlock if db.grow is triggered.
	db := MustOpenWithOption(&bolt.Options{InitialMmapSize: os.Getpagesize() * 100})
	defer db.MustClose()

	bucket := "bucket"

	put := func(key, value string) {
		if err := db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				t.Fatal(err)
			}
			return b.Put([]byte(key), []byte(value))
		}); err != nil {
			t.Fatal(err)
		}
	}

	del := func(key string) {
		if err := db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				t.Fatal(err)
			}
			return b.Delete([]byte(key))
		}); err != nil {
			t.Fatal(err)
		}
	}

	getWithTxn := func(txn *bolt.Tx, key string) []byte {
		return txn.Bucket([]byte(bucket)).Get([]byte(key))
	}

	openReadTxn := func() *bolt.Tx {
		readTx, err := db.Begin(false)
		if err != nil {
			t.Fatal(err)
		}
		return readTx
	}

	checkWithReadTxn := func(txn *bolt.Tx, key string, wantValue []byte) {
		value := getWithTxn(txn, key)
		if !bytes.Equal(value, wantValue) {
			t.Errorf("Wanted value to be %s for key %s, but got %s", wantValue, key, string(value))
		}
	}

	rollback := func(txn *bolt.Tx) {
		if err := txn.Rollback(); err != nil {
			t.Fatal(err)
		}
	}

	put("k1", "v1")
	rtx1 := openReadTxn()
	put("k2", "v2")
	hold1 := openReadTxn()
	put("k3", "v3")
	hold2 := openReadTxn()
	del("k3")
	rtx2 := openReadTxn()
	del("k1")
	hold3 := openReadTxn()
	del("k2")
	hold4 := openReadTxn()
	put("k4", "v4")
	hold5 := openReadTxn()

	// Close the read transactions we established to hold a portion of the pages in pending state.
	rollback(hold1)
	rollback(hold2)
	rollback(hold3)
	rollback(hold4)
	rollback(hold5)

	// Execute a write transaction to trigger a releaseRange operation in the db
	// that will free multiple ranges between the remaining open read transactions, now that the
	// holds have been rolled back.
	put("k4", "v4")

	// Check that all long running reads still read correct values.
	checkWithReadTxn(rtx1, "k1", []byte("v1"))
	checkWithReadTxn(rtx2, "k2", []byte("v2"))
	rollback(rtx1)
	rollback(rtx2)

	// Check that the final state is correct.
	rtx7 := openReadTxn()
	checkWithReadTxn(rtx7, "k1", nil)
	checkWithReadTxn(rtx7, "k2", nil)
	checkWithReadTxn(rtx7, "k3", nil)
	checkWithReadTxn(rtx7, "k4", []byte("v4"))
	rollback(rtx7)
}

func ExampleTx_Rollback() {
	// Open the database.
	db, err := bolt.Open(tempfile(), 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(db.Path())

	// Create a bucket.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("widgets"))
		return err
	}); err != nil {
		log.Fatal(err)
	}

	// Set a value for a key.
	if err := db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("widgets")).Put([]byte("foo"), []byte("bar"))
	}); err != nil {
		log.Fatal(err)
	}

	// Update the key but rollback the transaction so it never saves.
	tx, err := db.Begin(true)
	if err != nil {
		log.Fatal(err)
	}
	b := tx.Bucket([]byte("widgets"))
	if err := b.Put([]byte("foo"), []byte("baz")); err != nil {
		log.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		log.Fatal(err)
	}

	// Ensure that our original value is still set.
	if err := db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket([]byte("widgets")).Get([]byte("foo"))
		fmt.Printf("The value for 'foo' is still: %s\n", value)
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	// Close database to release file lock.
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}

	// Output:
	// The value for 'foo' is still: bar
}

func ExampleTx_CopyFile() {
	// Open the database.
	db, err := bolt.Open(tempfile(), 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(db.Path())

	// Create a bucket and a key.
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			return err
		}
		if err := b.Put([]byte("foo"), []byte("bar")); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	// Copy the database to another file.
	toFile := tempfile()
	if err := db.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(toFile, 0666)
	}); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(toFile)

	// Open the cloned database.
	db2, err := bolt.Open(toFile, 0666, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Ensure that the key exists in the copy.
	if err := db2.View(func(tx *bolt.Tx) error {
		value := tx.Bucket([]byte("widgets")).Get([]byte("foo"))
		fmt.Printf("The value for 'foo' in the clone is: %s\n", value)
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	// Close database to release file lock.
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}

	if err := db2.Close(); err != nil {
		log.Fatal(err)
	}

	// Output:
	// The value for 'foo' in the clone is: bar
}
