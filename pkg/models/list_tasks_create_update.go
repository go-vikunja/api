//  Vikunja is a todo-list application to facilitate your life.
//  Copyright 2018 Vikunja and contributors. All rights reserved.
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <https://www.gnu.org/licenses/>.

package models

import (
	"code.vikunja.io/api/pkg/metrics"
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/web"
	"github.com/imdario/mergo"
	"time"
)

// Create is the implementation to create a list task
// @Summary Create a task
// @Description Inserts a task into a list.
// @tags task
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param id path int true "List ID"
// @Param task body models.ListTask true "The task object"
// @Success 200 {object} models.ListTask "The created task object."
// @Failure 400 {object} code.vikunja.io/web.HTTPError "Invalid task object provided."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "The user does not have access to the list"
// @Failure 500 {object} models.Message "Internal error"
// @Router /lists/{id} [put]
func (t *ListTask) Create(a web.Auth) (err error) {
	doer, err := getUserWithError(a)
	if err != nil {
		return err
	}

	t.ID = 0

	// Check if we have at least a text
	if t.Text == "" {
		return ErrListTaskCannotBeEmpty{}
	}

	// Check if the list exists
	l := &List{ID: t.ListID}
	if err = l.GetSimpleByID(); err != nil {
		return
	}

	u, err := GetUserByID(doer.ID)
	if err != nil {
		return err
	}

	// Generate a uuid if we don't already have one
	if t.UID == "" {
		t.UID = utils.MakeRandomString(40)
	}

	t.CreatedByID = u.ID
	t.CreatedBy = u
	if _, err = x.Insert(t); err != nil {
		return err
	}

	// Update the assignees
	if err := t.updateTaskAssignees(t.Assignees); err != nil {
		return err
	}

	metrics.UpdateCount(1, metrics.TaskCountKey)

	err = updateListLastUpdated(&List{ID: t.ListID})
	return
}

// Update updates a list task
// @Summary Update a task
// @Description Updates a task. This includes marking it as done. Assignees you pass will be updated, see their individual endpoints for more details on how this is done. To update labels, see the description of the endpoint.
// @tags task
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param id path int true "Task ID"
// @Param task body models.ListTask true "The task object"
// @Success 200 {object} models.ListTask "The updated task object."
// @Failure 400 {object} code.vikunja.io/web.HTTPError "Invalid task object provided."
// @Failure 403 {object} code.vikunja.io/web.HTTPError "The user does not have access to the task (aka its list)"
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{id} [post]
func (t *ListTask) Update() (err error) {
	// Check if the task exists
	ot, err := GetTaskByID(t.ID)
	if err != nil {
		return
	}

	// Parent task cannot be the same as the current task
	if t.ID == t.ParentTaskID {
		return ErrParentTaskCannotBeTheSame{TaskID: t.ID}
	}

	// When a repeating task is marked as done, we update all deadlines and reminders and set it as undone
	updateDone(&ot, t)

	// Update the assignees
	if err := ot.updateTaskAssignees(t.Assignees); err != nil {
		return err
	}

	// Update the labels
	//
	// Maybe FIXME:
	// I've disabled this for now, because it requires significant changes in the way we do updates (using the
	// Update() function. We need a user object in updateTaskLabels to check if the user has the right to see
	// the label it is currently adding. To do this, we'll need to update the webhandler to let it pass the current
	// user object (like it's already the case with the create method). However when we change it, that'll break
	// a lot of existing code which we'll then need to refactor.
	// This is why.
	//
	//if err := ot.updateTaskLabels(t.Labels); err != nil {
	//	return err
	//}
	// set the labels to ot.Labels because our updateTaskLabels function puts the full label objects in it pretty nicely
	// We also set this here to prevent it being overwritten later on.
	//t.Labels = ot.Labels

	// For whatever reason, xorm dont detect if done is updated, so we need to update this every time by hand
	// Which is why we merge the actual task struct with the one we got from the
	// The user struct overrides values in the actual one.
	if err := mergo.Merge(&ot, t, mergo.WithOverride); err != nil {
		return err
	}

	//////
	// Mergo does ignore nil values. Because of that, we need to check all parameters and set the updated to
	// nil/their nil value in the struct which is inserted.
	////
	// Done
	if !t.Done {
		ot.Done = false
	}
	// Priority
	if t.Priority == 0 {
		ot.Priority = 0
	}
	// Description
	if t.Description == "" {
		ot.Description = ""
	}
	// Due date
	if t.DueDateUnix == 0 {
		ot.DueDateUnix = 0
	}
	// Reminders
	if len(t.RemindersUnix) == 0 {
		ot.RemindersUnix = nil
	}
	// Repeat after
	if t.RepeatAfter == 0 {
		ot.RepeatAfter = 0
	}
	// Parent task
	if t.ParentTaskID == 0 {
		ot.ParentTaskID = 0
	}
	// Start date
	if t.StartDateUnix == 0 {
		ot.StartDateUnix = 0
	}
	// End date
	if t.EndDateUnix == 0 {
		ot.EndDateUnix = 0
	}
	// Color
	if t.HexColor == "" {
		ot.HexColor = ""
	}

	_, err = x.ID(t.ID).
		Cols("text",
			"description",
			"done",
			"due_date_unix",
			"reminders_unix",
			"repeat_after",
			"parent_task_id",
			"priority",
			"start_date_unix",
			"end_date_unix",
			"hex_color",
			"done_at_unix").
		Update(ot)
	*t = ot
	if err != nil {
		return err
	}

	err = updateListLastUpdated(&List{ID: t.ListID})
	return
}

// This helper function updates the reminders and doneAtUnix of the *old* task (since that's the one we're inserting
// with updated values into the db)
func updateDone(oldTask *ListTask, newTask *ListTask) {
	if !oldTask.Done && newTask.Done && oldTask.RepeatAfter > 0 {
		oldTask.DueDateUnix = oldTask.DueDateUnix + oldTask.RepeatAfter // assuming we'll save the old task (merged)

		for in, r := range oldTask.RemindersUnix {
			oldTask.RemindersUnix[in] = r + oldTask.RepeatAfter
		}

		newTask.Done = false
	}

	// Update the "done at" timestamp
	if !oldTask.Done && newTask.Done {
		oldTask.DoneAtUnix = time.Now().Unix()
	}
	// When unmarking a task as done, reset the timestamp
	if oldTask.Done && !newTask.Done {
		oldTask.DoneAtUnix = 0
	}
}
