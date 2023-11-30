package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Log struct {
	controller.Abstract
}

func (self Log) Task(http *gin.Context) {
	type ParamsValidate struct {
		SiteId int32 `form:"siteId" binding:"required,number"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	taskRow, _ := dao.Task.Where(dao.Task.TaskLinkID.Eq(params.SiteId)).First()
	if taskRow == nil {
		self.JsonResponseWithError(http, errors.New("当前没有进行的中任务"), 500)
		return
	}
	if taskRow.Status != logic.STATUS_PROCESSING {
		self.JsonResponseWithoutError(http, gin.H{
			"status":  taskRow.Status,
			"message": taskRow.Message,
		})
		return
	}
	task := logic.NewContainerTask()
	stepLog := task.GetTaskStepLog(taskRow.TaskLinkID)
	if stepLog == nil {
		self.JsonResponseWithError(http, errors.New("当前没有进行的中任务或是已经完成"), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		logic.STEP_IMAGE_PULL: stepLog.GetProcess(),
	})

	//	json := `{
	//    "code": 200,
	//    "data": {
	//        "imagePull": {
	//            "1f7ce2fa46ab": {
	//                "downloading": 25,
	//                "extracting": 0
	//            },
	//            "249ff3a7bbe6": {
	//                "downloading": 67,
	//                "extracting": 0
	//            },
	//            "33777aea940a": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "48824c101c6a": {
	//                "downloading": 100,
	//                "extracting": 0
	//            },
	//            "5dcbbe73fcf0": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "5fdd7aa4a423": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "706c00f0b7d2": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "749c44fa1213": {
	//                "downloading": 60,
	//                "extracting": 30
	//            },
	//            "8e9959c9dd31": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "92eeb6cb0068": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "9c62851e2826": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "aa5d47f22b64": {
	//                "downloading": 100,
	//                "extracting": 0
	//            },
	//            "b3a08d032c4e": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "c2e56069baaf": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "dfc8a35621ec": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "e83ad87cf6a6": {
	//                "downloading": 100,
	//                "extracting": 0
	//            },
	//            "e84c71b81827": {
	//                "downloading": 0,
	//                "extracting": 0
	//            },
	//            "f82137d66483": {
	//                "downloading": 0,
	//                "extracting": 0
	//            }
	//        }
	//    }
	//}
	//`
	//	http.String(200, json)
	return
}

func (self Log) Run(http *gin.Context) {

}
