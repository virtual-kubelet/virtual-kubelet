package exec

import (
	"fmt"
	"encoding/json"
	"github.com/boltdb/bolt"
)

var bucket = []byte("ExecState")

type state struct {
	db				*bolt.DB
}

type Entry struct {
	PodName 	string 				`json:"podName"`
	Namespace	string 				`json:"namespace"`
	Processes	map[string]Process	`json:"processes"`
}

type Process struct {
	Image				string 	`json:"image"`
	Pid 				int 	`json:"pid"`
	LogPath 			string	`json:"logPath"`
	StartTimestamp		int64	`json:"startTimestamp"`

}

func NewState(stateFile string) (*state, error) {
	db, err := bolt.Open(stateFile, 0600, nil)
	if err != nil {
		fmt.Printf("Error when opening DB at %s: %s\n", stateFile, err.Error())
		return &state{}, err
	}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return fmt.Errorf("creating ExecTracker bucket: %s", err)
		}
		return nil
	})

	return &state{ db }, err
}

func (s *state) Put(entry Entry) error {
	key := s.key(entry.Namespace, entry.PodName)
	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		err = tx.Bucket(bucket).Put([]byte(key), json)
		return err
	})

	return err
}

func (s *state) Get(namespace string, podName string) (*Entry, error) {
	var val []byte
	var entry Entry
	key := s.key(namespace, podName)

	err := s.db.View(func(tx *bolt.Tx) error {
		val = tx.Bucket(bucket).Get([]byte(key))
		return nil
	})

	if err != nil || len(val) == 0 {
		return nil, err
	}

	return &entry, json.Unmarshal(val, &entry)
}

func (s *state) GetAll() ([]Entry, error) {
	var entries []Entry

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry Entry
			err := json.Unmarshal(v, &entry)
			if err != nil {
				return err
			}
			entries = append(entries, entry)
		}

		return nil
	}); err != nil {
		return entries, err
	}

	return entries, nil
}

func (s *state) Remove(namespace string, podName string) error {
	key := s.key(namespace, podName)
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Delete([]byte(key))
	})
}

func (s *state) key(namespace string, podName string) string {
	return fmt.Sprintf("%s/%s", namespace, podName)
}


func (s *state) Close() {
	s.db.Close()
}