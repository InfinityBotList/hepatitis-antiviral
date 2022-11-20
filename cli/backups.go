// Extra feature of hepatitis-antiviral: a simple backup system from one DB to another
// so the result can then be used with normal data moving.
//
// This assumes that the DB schema is already setup
package cli

import (
	"fmt"
)

type BackupSource interface {
	RecordList() ([]string, error)
	// Same as Source interface
	GetRecords(entity string) ([]map[string]any, error)
	// Extra parsers
	ExtParse(res any) (any, error)
}

type BackupLocation interface {
	BackupRecord(entity string, record map[string]any) error
	// Clear the backup location
	Clear() error
	// Sync to disk
	Sync() error
}

func Backup(src BackupSource, recv BackupLocation) {
	/*if Bar == nil {
		mb = mpb.New(mpb.WithWidth(64))
	}*/

	recordList, err := src.RecordList()

	NotifyMsg("info", "Record list:"+fmt.Sprint(recordList))

	if err != nil {
		NotifyMsg("error", "Error getting record list: "+err.Error())
		return
	}

	err = recv.Clear()

	if err != nil {
		NotifyMsg("error", "Error clearing: "+err.Error())
		return
	}

	for _, entity := range recordList {
		records, err := src.GetRecords(entity)

		if err != nil {
			NotifyMsg("error", "Error getting records: "+err.Error())
			return
		}

		//bar := StartBar(entity, int64(len(records)), true)
		for _, record := range records {
			//bar.Increment()
			err := recv.BackupRecord(entity, record)

			if err != nil {
				NotifyMsg("error", "Error backing up record: "+err.Error())
				return
			}
		}
	}

	err = recv.Sync()

	if err != nil {
		NotifyMsg("error", "Error syncing: "+err.Error())
		return
	}
}
