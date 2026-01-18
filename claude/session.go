package claude

// Session はセッション情報を保持する
type Session struct {
	ID        string // セッションID
	IsForked  bool   // 分岐したセッションかどうか
	ParentID  string // 分岐元のセッションID（分岐の場合のみ）
	Continued bool   // 継続されたセッションかどうか
}

// NewSession は新しいセッションを作成する
func NewSession(id string) *Session {
	return &Session{
		ID: id,
	}
}

// Fork はセッションを分岐する設定を返す
func (s *Session) Fork() *Session {
	return &Session{
		ParentID: s.ID,
		IsForked: true,
	}
}

// FileCheckpoint はファイルチェックポイント情報を保持する
type FileCheckpoint struct {
	UserMessageID string // ユーザーメッセージのUUID
	SessionID     string // セッションID
}

// NewFileCheckpoint は新しいファイルチェックポイントを作成する
func NewFileCheckpoint(userMessageID, sessionID string) *FileCheckpoint {
	return &FileCheckpoint{
		UserMessageID: userMessageID,
		SessionID:     sessionID,
	}
}
