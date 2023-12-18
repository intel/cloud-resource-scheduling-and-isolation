/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"fmt"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
	"k8s.io/klog/v2"
)

var (
	StorePath      = "/opt/ioiservice/ioservice.db"
	StoreDirectory = "/opt/ioiservice"
)

type IPersist interface {
	CreateTable(table string, subTables []string) error
	Save(table string, key string, value []byte) error
	Load(table string, key string) (map[string][]byte, error)
	Delete(table string, key string) error
	Close() error
}

type DBPersist struct {
	bdb *bolt.DB
}

func NewDBPersist() IPersist {
	err := os.MkdirAll(StoreDirectory, 0o666)
	if err != nil {
		klog.Errorf("create directory %s error: %v", StoreDirectory, err)
		return nil
	}

	db, err := bolt.Open(StorePath, 0600, nil)
	if err != nil {
		klog.Errorf("open database error: %v", err)
		return nil
	}
	return &DBPersist{bdb: db}
}

func (store *DBPersist) CreateTable(table string, subTables []string) error {
	err := store.bdb.Update(func(tx *bolt.Tx) error {
		// create rdt bucket
		bRoot, err := tx.CreateBucketIfNotExists([]byte(table))
		if err != nil {
			return fmt.Errorf("could not create %s root bucket: %v", table, err)
		}
		for _, bucket := range subTables {
			_, err = bRoot.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("could not create %s bucket: %v", bucket, err)
			}
		}

		return nil
	})
	return err
}

func (store *DBPersist) Save(table string, key string, value []byte) error {
	if key == "" {
		return fmt.Errorf("key is empty")
	}
	s := strings.Split(table, ".")
	sLen := len(s)
	err := store.bdb.Update(func(tx *bolt.Tx) error {
		var err error
		if sLen == 1 {
			err = tx.Bucket([]byte(s[0])).Put([]byte(key), value)
		} else if sLen == 2 {
			err = tx.Bucket([]byte(s[0])).Bucket([]byte(s[1])).Put([]byte(key), value)
		} else {
			return fmt.Errorf("table name error: %s", table)
		}
		if err != nil {
			return fmt.Errorf("could not save %s table %s key: %v", table, key, err)
		}
		return nil
	})

	return err
}

func (store *DBPersist) Load(table string, key string) (map[string][]byte, error) {
	all := make(map[string][]byte)
	s := strings.Split(table, ".")
	var siBytes []byte
	err := store.bdb.View(func(tx *bolt.Tx) error {
		sLen := len(s)
		if sLen == 1 {
			if key != "" {
				siBytes = tx.Bucket([]byte(table)).Get([]byte(key))
				if len(siBytes) == 0 {
					return nil
				}
				all[string(key)] = siBytes
				return nil
			} else {
				b := tx.Bucket([]byte(s[0]))
				err := b.ForEach(func(k, v []byte) error {
					all[string(k)] = v
					return nil
				})
				return err
			}
		} else if sLen == 2 {
			if key != "" {
				siBytes = tx.Bucket([]byte(s[0])).Bucket([]byte(s[1])).Get([]byte(key))
				if len(siBytes) == 0 {
					return nil
				}
				all[string(key)] = siBytes
				return nil
			} else {
				b := tx.Bucket([]byte(s[0])).Bucket([]byte(s[1]))
				err := b.ForEach(func(k, v []byte) error {
					all[string(k)] = v
					return nil
				})
				return err
			}
		} else {
			return fmt.Errorf("table name error: %s", table)
		}
	})

	return all, err
}

func (store *DBPersist) Delete(table string, key string) error {
	s := strings.Split(table, ".")
	sLen := len(s)
	err := store.bdb.Update(func(tx *bolt.Tx) error {
		var err error
		if sLen == 1 {
			err = tx.Bucket([]byte(s[0])).Delete([]byte(key))
		} else if sLen == 2 {
			err = tx.Bucket([]byte(s[0])).Bucket([]byte(s[1])).Delete([]byte(key))
		} else {
			return fmt.Errorf("table name error: %s", table)
		}
		if err != nil {
			return fmt.Errorf("could not delete app: %v", err)
		}
		return nil
	})
	return err
}

func (store *DBPersist) Close() error {
	return store.bdb.Close()
}
