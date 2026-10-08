package journal

type Repository interface {
	Create(entry Entry) (Entry, error)
	ListByUser(userID string) ([]Entry, error)
	GetByID(userID string, entryID string) (Entry, error)
	Update(entry Entry) (Entry, error)
	Delete(userID string, entryID string) error
}
