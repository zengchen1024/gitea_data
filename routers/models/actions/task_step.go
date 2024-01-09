package actions

import "code.gitea.io/gitea/modules/timeutil"

// ActionTaskStep represents a step of ActionTask
type ActionTaskStep struct {
	ID        int64
	Name      string `xorm:"VARCHAR(255)"`
	TaskID    int64  `xorm:"index unique(task_index)"`
	Index     int64  `xorm:"index unique(task_index)"`
	RepoID    int64  `xorm:"index"`
	Status    Status `xorm:"index"`
	LogIndex  int64
	LogLength int64
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	Created   timeutil.TimeStamp `xorm:"created"`
	Updated   timeutil.TimeStamp `xorm:"updated"`
}
