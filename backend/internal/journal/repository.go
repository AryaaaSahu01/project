package journal

type Repository interface {
	Create(entry Entry) (Entry, error)
	ListByUser(userID string) ([]Entry, error)
	GetByID(userID string, entryID string) (Entry, error)
}
