package plugin

import (
	"testing"

	apis "github.com/openebs/zfs-localpv/v2/pkg/apis/openebs.io/zfs/v1"
)

const (
	snap1 = "schd-20261122010000"
	snap2 = "schd-20261122020000"
	snap3 = "schd-20261122030000"
)

func zfsBackup(snapName string, status apis.ZFSBackupStatus) apis.ZFSBackup {
	bkp := apis.ZFSBackup{}
	bkp.Spec.SnapName = snapName
	bkp.Status = status

	return bkp
}

func TestPrevSnapFromBackups(t *testing.T) {
	tests := []struct {
		name  string
		items []apis.ZFSBackup
		count uint64
		want  string
	}{
		{
			name:  "nothing backed up yet, so a full backup is due",
			count: 2,
			want:  "",
		},
		{
			name:  "one completed backup, the next one continues from it",
			items: []apis.ZFSBackup{zfsBackup(snap1, apis.BKPZFSStatusDone)},
			count: 2,
			want:  snap1,
		},
		{
			name: "the group is complete, so a full backup is due",
			items: []apis.ZFSBackup{
				zfsBackup(snap1, apis.BKPZFSStatusDone),
				zfsBackup(snap2, apis.BKPZFSStatusDone),
			},
			count: 2,
			want:  "",
		},
		{
			name: "an unfinished backup neither moves the group boundary nor becomes the base",
			items: []apis.ZFSBackup{
				zfsBackup(snap1, apis.BKPZFSStatusDone),
				zfsBackup(snap2, apis.BKPZFSStatusInit),
			},
			count: 2,
			want:  snap1,
		},
		{
			name:  "the only backup on record never finished, so a full backup is due",
			items: []apis.ZFSBackup{zfsBackup(snap1, apis.BKPZFSStatusFailed)},
			count: 2,
			want:  "",
		},
		{
			name: "the latest completed backup is the base whatever order the list arrives in",
			items: []apis.ZFSBackup{
				zfsBackup(snap3, apis.BKPZFSStatusDone),
				zfsBackup(snap1, apis.BKPZFSStatusDone),
				zfsBackup(snap2, apis.BKPZFSStatusInProgress),
			},
			count: 4,
			want:  snap3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prevSnapFromBackups(tt.items, tt.count)
			if got != tt.want {
				t.Errorf("prevSnapFromBackups() = %q, want %q", got, tt.want)
			}
		})
	}
}
