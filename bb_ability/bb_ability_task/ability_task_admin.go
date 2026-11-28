package bb_ability_task

import (
	"context"
	"database/sql"
	"errors"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	adminTaskListDefault = 100
	adminTaskListMax     = 200
)

func adminTaskResult(task *bb_ability_task_model.AsyncTask, includeResult bool) map[string]interface{} {
	item := taskResult(task, includeResult)
	if task.UserID != nil && *task.UserID != "" {
		item["user_id"] = *task.UserID
	} else {
		item["user_id"] = nil
	}
	return item
}

func anyTaskWithSteps(db *gorm.DB, taskID string) (map[string]interface{}, error) {
	task := &bb_ability_task_model.AsyncTask{}
	var steps []map[string]interface{}
	txErr := db.Transaction(func(tx *gorm.DB) error {
		if findErr := tx.Where("task_id = ?", taskID).Take(task).Error; findErr != nil {
			return findErr
		}
		var stepErr error
		steps, stepErr = loadTaskSteps(tx, taskID)
		return stepErr
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if errors.Is(txErr, gorm.ErrRecordNotFound) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if txErr != nil {
		return nil, txErr
	}
	item := adminTaskResult(task, true)
	item["steps"] = steps
	item["execution_count"] = task.ExecutionCount
	item["current_business_step"] = nil
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i]["step_kind"] == StepKindBusiness && steps[i]["attempt"] == task.ExecutionCount {
			item["current_business_step"] = steps[i]
			break
		}
	}
	if len(steps) > 0 {
		item["current_step"] = steps[len(steps)-1]
	} else {
		item["current_step"] = nil
	}
	return item, nil
}

func loadAnyTaskFromParams(ctx context.Context, params map[string]interface{}) (*bb_ability_task_model.AsyncTask, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	taskID, taskIDOK := bb_api_auth_common.RequiredString(params, "task_id")
	if !taskIDOK || uuid.Validate(taskID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	task := &bb_ability_task_model.AsyncTask{}
	findErr := db.Where("task_id = ?", taskID).Take(task).Error
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if findErr != nil {
		return nil, findErr
	}
	return task, nil
}

func AdminTaskList(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasNoUnknownKeys(params, "limit", "offset", "user_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	limit, limitOK := bb_api_auth_common.OptionalInt(params, "limit", adminTaskListDefault, 1, adminTaskListMax)
	offset, offsetOK := bb_api_auth_common.OptionalInt(params, "offset", 0, 0, 1_000_000_000)
	if !limitOK || !offsetOK {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	var ownerID string
	if _, exists := params["user_id"]; exists {
		value, ok := bb_api_auth_common.RequiredString(params, "user_id")
		if !ok || uuid.Validate(value) != nil {
			return nil, bb_api_error_code.ErrInvalidArguments
		}
		ownerID = value
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	query := db.Model(&bb_ability_task_model.AsyncTask{})
	if ownerID != "" {
		query = query.Where("user_id = ?", ownerID)
	}
	var total int64
	if err = query.Count(&total).Error; err != nil {
		return nil, err
	}
	tasks := make([]*bb_ability_task_model.AsyncTask, 0)
	if err = query.Order("id DESC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, err
	}
	results := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		results = append(results, adminTaskResult(task, false))
	}
	return map[string]interface{}{
		"tasks":  results,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}, nil
}

func AdminTaskGet(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	taskID, taskIDOK := bb_api_auth_common.RequiredString(params, "task_id")
	if !taskIDOK || uuid.Validate(taskID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	item, err := anyTaskWithSteps(db, taskID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"task": item}, nil
}

func AdminTaskCancel(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	taskID, ok := bb_api_auth_common.RequiredString(params, "task_id")
	if !ok || uuid.Validate(taskID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	return cancelTaskByID(ctx, taskID, "")
}

func AdminTaskArtifacts(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	task, err := loadAnyTaskFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	artifacts := make([]artifactInfo, 0)
	jobDir, err := confinedJobDir(task)
	if err == nil && jobDir != "" {
		listed, listErr := listTaskArtifacts(jobDir)
		if listErr != nil {
			return nil, listErr
		}
		artifacts = listed
	}
	item := map[string]interface{}{"task_id": task.TaskID, "artifacts": artifacts}
	if task.UserID != nil {
		item["user_id"] = *task.UserID
	}
	return item, nil
}

func AdminTaskArtifact(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id", "path") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	task, err := loadAnyTaskFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	rel, ok := bb_api_auth_common.RequiredString(params, "path")
	if !ok {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	jobDir, err := confinedJobDir(task)
	if err != nil || jobDir == "" {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	file, err := readTaskArtifact(jobDir, rel)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id":      task.TaskID,
		"path":         file.Path,
		"name":         file.Name,
		"kind":         file.Kind,
		"size":         file.Size,
		"content_type": file.ContentType,
		"sha256":       file.SHA256,
		"encoding":     "base64",
		"content":      file.Content,
	}, nil
}
