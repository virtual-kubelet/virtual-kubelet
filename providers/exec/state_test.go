package exec

import (
	"github.com/boltdb/bolt"
	"os"
	"path/filepath"
	"testing"
	"reflect"
	"time"
)

var entry = Entry{
	PodName:"podName",
	Namespace: "namespace",
	Processes: map[string]Process{ "main": Process{"image", 123, "logPath", time.Now().Unix()}},
}

func TestGetNonExistingRecord(t *testing.T) {
	s := createEmptyState(t)
	defer cleanup(s)
	e, err := s.Get("namespace", "podName")
	if err != nil {
		t.Errorf("Get should return an empty Entry and no error for non existing record, error: %s", err.Error())
	}
	if e != nil {
		t.Errorf("Get should return an empty Entry for non existing record")
	}
}


func TestPutAndGet(t *testing.T) {
	var err error
	s := createEmptyState(t)
	defer cleanup(s)

	// Put
	err = s.Put(entry)
	if err != nil {
		t.Error(err.Error())
	}

	// Get
	getAndCompare(t, s, &entry)
}

func TestRemove(t *testing.T) {
	var err error
	s := createEmptyState(t)
	defer cleanup(s)

	// Put
	err = s.Put(entry)
	if err != nil {
		t.Error(err.Error())
	}

	// Delete
	err = s.Remove(entry.Namespace, entry.PodName)
	if err != nil {
		t.Error(err.Error())
	}

	// Get
	if e, _ := s.Get(entry.Namespace, entry.PodName); e != nil {
		t.Error("Deleted entry should not be returned")
	}
}

func TestGetAll(t *testing.T) {
	var err error
	s := createEmptyState(t)
	defer cleanup(s)

	// Put
	entry.PodName = "pod1"
	err = s.Put(entry)
	if err != nil {
		t.Error(err.Error())
	}

	// Put
	entry.PodName = "pod2"
	err = s.Put(entry)
	if err != nil {
		t.Error(err.Error())
	}

	// GetAll
	entries, err := s.GetAll()
	if err != nil {
		t.Error(err.Error())
	}
	if len(entries) != 2 {
		t.Error("Multiple entries should be returned")
	}
}

func getAndCompare(t *testing.T, s *state, e1 *Entry) {
	e2, err := s.Get(e1.Namespace, e1.PodName)
	if err != nil {
		t.Error(err.Error())
	}
	if ! reflect.DeepEqual(e1, e2) {
		t.Errorf("Put followed by Get should return an empty Entry for non existing record, %v %v", e1, e2)
	}
}

func createEmptyState(t *testing.T) *state {
	s, err := NewState(filepath.Join(os.TempDir(), t.Name()))

	if err != nil {
		t.Errorf("Could not create State %s", err.Error())
	}
	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			err = b.Delete(k)
			if err != nil {
				t.Errorf("Could not delete %v, error: %s", k, err.Error())
				return err
			}
		}
		return err
	})
	return s
}

func cleanup(s *state) {
	path := s.db.Path()
	s.Close()
	os.Remove(path)
}