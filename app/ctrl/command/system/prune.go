package system

import (
	"os"
	"runtime"
	"strings"

	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Prune struct {
}

func (self Prune) GetName() string {
	return "system:prune"
}

func (self Prune) GetDescription() string {
	return "Prune message, event, and temp file."
}

func (self Prune) Configure(cmd *cobra.Command) {

}

func (self Prune) Handle(cmd *cobra.Command, args []string) {
	total := 0
	if v := (storage.Local{}).GetSaveRootPath(); v != "" {
		if list, err := os.ReadDir(v); err == nil {
			for _, entry := range list {
				if strings.HasPrefix(entry.Name(), "dpanel-temp-") {
					if err := os.Remove(storage.Local{}.GetSaveRealPath(entry.Name())); err == nil {
						total++
					}
				}

			}
		}
	}
	var eventTotal int64
	if oldRow, _ := dao.Event.Last(); oldRow != nil {
		query := dao.Event.Where(dao.Event.ID.Lte(oldRow.ID))
		eventTotal, _ = query.Count()
		_, _ = query.Delete()
	}

	var noticeTotal int64
	if oldRow, _ := dao.Notice.Last(); oldRow != nil {
		query := dao.Notice.Where(dao.Notice.ID.Lte(oldRow.ID))
		noticeTotal, _ = query.Count()
		_, _ = query.Delete()
	}

	runtime.GC()
	if db, err := facade.GetDbFactory().Channel("default"); err == nil {
		db.Exec("vacuum")
	}

	utils.Result{}.Success(gin.H{
		"gc":     true,
		"db":     "vacuum",
		"temp":   total,
		"events": eventTotal,
		"notice": noticeTotal,
	})
}
