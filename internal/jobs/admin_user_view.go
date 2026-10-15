package jobs

func (s *Store) RecordAdminUserView(actor, target string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = auditTx(tx, actor, "user_read_only_view", target, nil, map[string]string{"user_id": target, "mode": "read_only"}, "管理员查看用户工作台"); err != nil {
		return err
	}
	return tx.Commit()
}
