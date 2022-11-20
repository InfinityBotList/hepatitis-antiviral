// Package “jsonfile“ defines a JSON file storage for hepatitis-antiviral
// Implements both Source, BackupSource and BackupLocation
package jsonfile

import (
	"encoding/json"
	"os"

	"golang.org/x/exp/slices"
)

type JsonFileStore struct {
	Filename string
	// Created on Connect()
	file           *os.File
	diskmap        *map[string][]map[string]any
	IgnoreEntities []string
}

func (m *JsonFileStore) Connect() error {
	// Check if file already exists
	_, err := os.Stat(m.Filename)

	if os.IsNotExist(err) {
		// Create file
		file, err := os.Create(m.Filename)

		if err != nil {
			return err
		}

		// Write empty JSON object
		_, err = file.Write([]byte("{}"))

		if err != nil {
			return err
		}

		m.file = file
		m.diskmap = &map[string][]map[string]any{}
	} else {
		var err error
		m.file, err = os.Open(m.Filename)
		if err != nil {
			return err
		}

		decoder := json.NewDecoder(m.file)

		err = decoder.Decode(&m.diskmap)

		if err != nil {
			return err
		}

		m.file.Close()
	}
	return nil
}

func (m JsonFileStore) GetRecords(entity string) ([]map[string]any, error) {
	if slices.Contains(m.IgnoreEntities, entity) {
		return []map[string]any{}, nil
	}

	mapped := *m.diskmap

	return mapped[entity], nil
}

func (m JsonFileStore) GetCount(entity string) (int64, error) {
	if slices.Contains(m.IgnoreEntities, entity) {
		return 0, nil
	}

	records, err := m.GetRecords(entity)
	if err != nil {
		return 0, err
	}
	return int64(len(records)), nil
}

func (m JsonFileStore) RecordList() ([]string, error) {
	var record []string
	for name := range *m.diskmap {
		if slices.Contains(m.IgnoreEntities, name) {
			continue
		}
		record = append(record, name)
	}

	return record, nil
}

func (m JsonFileStore) BackupRecord(entity string, record map[string]any) error {
	if slices.Contains(m.IgnoreEntities, entity) {
		return nil
	}

	mapped := *m.diskmap

	if _, ok := mapped[entity]; !ok {
		mapped[entity] = []map[string]any{}
	}

	mapped[entity] = append(mapped[entity], record)

	// Copy back
	*m.diskmap = mapped

	return nil
}

func (m JsonFileStore) Clear() error {
	*m.diskmap = map[string][]map[string]any{}
	return nil
}

func (m JsonFileStore) Sync() error {
	// Delete old file
	err := os.Remove(m.Filename)

	if err != nil {
		return err
	}

	// Open file
	file, err := os.OpenFile(m.Filename, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return err
	}

	// Encode JSON
	encoder := json.NewEncoder(file)
	err = encoder.Encode(m.diskmap)

	if err != nil {
		return err
	}

	return nil
}

// No special types for JSON files
func (m JsonFileStore) ExtParse(res any) (any, error) {
	return res, nil
}
