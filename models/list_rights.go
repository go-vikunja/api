package models

// IsAdmin returns whether the user has admin rights on the list or not
func (l *List) IsAdmin(user *User) bool {
	// Owners are always admins
	if l.Owner.ID == user.ID {
		return true
	}

	return l.checkListTeamRight(user, TeamRightAdmin)
}

// CanWrite return whether the user can write on that list or not
func (l *List) CanWrite(user *User) bool {
	// Owners always have write access
	if l.Owner.ID == user.ID {
		return true
	}

	// Admins always have write access
	if l.IsAdmin(user) {
		return true
	}

	return l.checkListTeamRight(user, TeamRightWrite)
}

// CanRead checks if a user has read access to a list
func (l *List) CanRead(user *User) bool {
	// Owners always have read access
	if l.Owner.ID == user.ID {
		return true
	}

	// Admins always have read access
	if l.IsAdmin(user) {
		return true
	}

	return l.checkListTeamRight(user, TeamRightRead)
}

// CanDelete checks if the user can delete a list
func (l *List) CanDelete(doer *User) bool {
	list, _ := GetListByID(l.ID)
	return list.IsAdmin(doer)
}

// CanUpdate checks if the user can update a list
func (l *List) CanUpdate(doer *User) bool {
	list, _ := GetListByID(l.ID)
	return list.CanWrite(doer)
}

// CanCreate checks if the user can update a list
func (l *List) CanCreate(doer *User) bool {
	// A user can create a list if he has write access to the namespace
	n, _ := GetNamespaceByID(l.NamespaceID)
	return n.CanWrite(doer)
}

func (l *List) checkListTeamRight(user *User, r TeamRight) bool {
	exists, err := x.Select("l.*").
		Table("list").
		Alias("l").
		Join("LEFT", []string{"team_namespaces", "tn"}, "tn.namespace_id = tn.id").
		Join("LEFT", []string{"team_members", "tm"}, "tm.team_id = tn.team_id").
		Join("LEFT", []string{"team_list", "tl"}, "l.id = tl.list_id").
		Join("LEFT", []string{"team_members", "tm2"}, "tm2.team_id = tl.team_id").
		Where("((tm.user_id = ? AND tn.right = ?) OR (tm2.user_id = ? AND tl.rights = ?)) AND l.id = ?",
			user.ID, r, user.ID, r, l.ID).
		Get(&List{})
	if err != nil {
		return false
	}

	return exists
}
